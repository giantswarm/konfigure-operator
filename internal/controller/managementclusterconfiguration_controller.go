/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"fmt"
	"github.com/giantswarm/konfigure/pkg/fluxupdater"
	"github.com/giantswarm/konfigure/pkg/sopsenv"
	"k8s.io/api/core/v1"
	apiMachineryErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/giantswarm/konfigure-operator/internal/controller/logic"
	"github.com/giantswarm/konfigure-operator/internal/konfigure"

	konfigureService "github.com/giantswarm/konfigure/pkg/service"

	konfigurev1alpha1 "github.com/giantswarm/konfigure-operator/api/v1alpha1"
)

// ManagementClusterConfigurationReconciler reconciles a ManagementClusterConfiguration object
type ManagementClusterConfigurationReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=konfigure.giantswarm.io,resources=managementclusterconfigurations,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=konfigure.giantswarm.io,resources=managementclusterconfigurations/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=konfigure.giantswarm.io,resources=managementclusterconfigurations/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the ManagementClusterConfiguration object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.19.1/pkg/reconcile
func (r *ManagementClusterConfigurationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (result ctrl.Result, returnError error) {
	logger := log.FromContext(ctx)
	reconcileStart := time.Now()

	// Get resource under reconciliation
	cr := &konfigurev1alpha1.ManagementClusterConfiguration{}
	if err := r.Get(ctx, req.NamespacedName, cr); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	logger.Info(fmt.Sprintf("Reconciling ManagementClusterConfiguration: %s/%s", cr.GetNamespace(), cr.GetName()))

	asd := false

	// Add finalizer
	if !controllerutil.ContainsFinalizer(cr, konfigurev1alpha1.KonfigureOperatorFinalizer) {
		//logger.Info(fmt.Sprintf("Need to add finalizer to the CR: %s", cr))
		//cr.Finalizers = append(cr.Finalizers, konfigurev1alpha1.KonfigureOperatorFinalizer)
		//logger.Info(fmt.Sprintf("Added finalizer to the CR: %s", cr))

		if controllerutil.AddFinalizer(cr, konfigurev1alpha1.KonfigureOperatorFinalizer) {
			logger.Info(fmt.Sprintf("Added finalizer to the CR: %s", cr))
			_ = r.Update(ctx, cr)
		} else {
			logger.Info(fmt.Sprintf("No need to add finalizer to the CR: %s", cr))
		}

		asd = true
		return ctrl.Result{Requeue: true}, nil
	}

	// Update status
	defer func() {
		if asd {
			return
		}

		//base := client.MergeFrom(cr.DeepCopyObject().(client.Object))
		//err := r.Status().Update(ctx, cr)
		//if err := r.Patch(ctx, cr, base); err != nil {
		logger.Info(fmt.Sprintf("Defer, before update for: %s", cr))
		if err := r.Status().Update(ctx, cr); err != nil {
			logger.Error(err, fmt.Sprintf("Failed to update status for: %s/%s", cr.GetNamespace(), cr.GetName()))
			returnError = err
		}

		logger.Info(fmt.Sprintf("Defer, after update for: %s", cr))
		logger.Info(fmt.Sprintf("Updated status and finalizers for: %s/%s", cr.GetNamespace(), cr.GetName()))

		// TODO Handle reschedule in case if failures.
		conditions := cr.Status.Conditions
		if conditions != nil {
			for _, condition := range conditions {
				if condition.Type == logic.ReadyCondition {
					if condition.Status == metav1.ConditionTrue {
						logger.Info(fmt.Sprintf("Reconciliation finished in %s, next run in %s", time.Since(reconcileStart).String(), cr.Spec.Reconciliation.Interval.Duration.String()))
					}
				}
			}
		}
	}()

	// Run finalizers if the object is being deleted
	if !cr.ObjectMeta.DeletionTimestamp.IsZero() {
		asd = true
		return r.finalize(ctx, cr)
	}

	// Initialize Konfigure
	sops, err := r.initializeSopsEnv(ctx)
	if err != nil {
		r.updateStatusOnSetupFailure(ctx, cr, err)

		return ctrl.Result{}, err
	}
	logger.Info(fmt.Sprintf("SOPS environment successfully set up at: %s", sops.GetKeysDir()))

	fluxUpdater, err := r.initializeFluxUpdater(cr.Spec.Sources.Flux)
	if err != nil {
		r.updateStatusOnSetupFailure(ctx, cr, err)

		return ctrl.Result{}, err
	}
	logger.Info("Konfigure cache successfully updated!")

	service, err := r.initializeKonfigure(ctx, sops.GetKeysDir(), fluxUpdater.CacheDir, cr.Spec.Configuration.Cluster.Name)
	if err != nil {
		r.updateStatusOnSetupFailure(ctx, cr, err)

		return ctrl.Result{}, err
	}

	appsToRender, missedExactMatchers, err := logic.GetAppsToReconcile(service.GetDir(), &cr.Spec.Configuration)

	logger.Info(fmt.Sprintf("Apps to reconcile: %s", strings.Join(appsToRender, ",")))
	logger.Info(fmt.Sprintf("Missed exact matchers: %s", strings.Join(missedExactMatchers, ",")))

	// TODO Handles misses for status updates

	revision, err := konfigure.GetLastArchiveSHA(fluxUpdater.CacheDir)
	if err != nil {
		logger.Error(err, fmt.Sprintf("Failed to get last archive SHA from: %s", service.GetDir()))
		revision = "unknown"
	}

	ownershipLabels := logic.GenerateOwnershipLabels(cr, revision)

	failures := make(map[string]string)
	for _, appToRender := range appsToRender {
		configmap, secret, err := r.renderAppConfiguration(ctx, service, appToRender, revision, cr.Spec.Destination, ownershipLabels)

		if err != nil {
			logger.Error(err, fmt.Sprintf("Failed to render app configuration for: %s", appToRender))

			failures[appToRender] = err.Error()
			continue
		}

		logger.Info(fmt.Sprintf("Succesfully rendered app configuration for: %s", appToRender))

		// Pre-flight check config map apply
		if err = r.canApplyConfigMap(ctx, configmap); err != nil {
			failures[appToRender] = err.Error()
		}

		// Pre-flight check secret apply. Present both errors to avoid the need to fix in multiple turns.
		if err = r.canApplySecret(ctx, secret); err != nil {
			if failures[appToRender] != "" {
				failures[appToRender] = failures[appToRender] + " " + err.Error()
			} else {
				failures[appToRender] = err.Error()
			}
		}

		if failures[appToRender] != "" {
			continue
		}

		err = r.applyConfigMap(ctx, configmap)
		if err != nil {
			logger.Error(err, fmt.Sprintf("Failed to apply configmap %s/%s for app: %s", configmap.Namespace, configmap.Name, appToRender))

			failures[appToRender] = err.Error()
			continue
		}

		err = r.applySecret(ctx, secret)
		if err != nil {
			logger.Error(err, fmt.Sprintf("Failed to apply secret %s/%s for app: %s", secret.Namespace, secret.Name, appToRender))

			failures[appToRender] = err.Error()
			continue
		}

		logger.Info(fmt.Sprintf("Succesfully applied rendered configmap and secret for: %s", appToRender))
	}

	// Status update for failures
	logger.Info(fmt.Sprintf("Failures: %s", failures))

	cr.Status.Failures = []konfigurev1alpha1.FailureStatus{}
	for failedAppName, failureMessage := range failures {
		cr.Status.Failures = append(cr.Status.Failures, konfigurev1alpha1.FailureStatus{
			AppName: failedAppName,
			Message: failureMessage,
		})
	}

	cr.Status.ObservedGeneration = cr.ObjectMeta.Generation
	cr.Status.LastReconciledAt = time.Now().Format(time.RFC3339Nano)

	cr.Status.LastAttemptedRevision = revision

	cr.Status.Conditions = []metav1.Condition{}
	if len(failures) > 0 {
		cr.Status.Conditions = append(cr.Status.Conditions, metav1.Condition{
			Type:               logic.ReadyCondition,
			Status:             metav1.ConditionFalse,
			ObservedGeneration: cr.ObjectMeta.Generation,
			LastTransitionTime: metav1.NewTime(time.Now().UTC().Truncate(time.Second)),
			Reason:             logic.ReconciliationFailedReason,
			Message:            fmt.Sprintf("Attempted revision: %s", revision),
		})

		return ctrl.Result{RequeueAfter: cr.Spec.Reconciliation.RetryInterval.Duration}, nil
	}

	cr.Status.LastAppliedRevision = revision

	cr.Status.Conditions = append(cr.Status.Conditions, metav1.Condition{
		Type:               logic.ReadyCondition,
		Status:             metav1.ConditionTrue,
		ObservedGeneration: cr.ObjectMeta.Generation,
		LastTransitionTime: metav1.NewTime(time.Now().UTC().Truncate(time.Second)),
		Reason:             logic.ReconciliationSucceededReason,
		Message:            fmt.Sprintf("Applied revision: %s", revision),
	})

	return ctrl.Result{RequeueAfter: cr.Spec.Reconciliation.Interval.Duration}, nil
}

func (r *ManagementClusterConfigurationReconciler) initializeSopsEnv(ctx context.Context) (*sopsenv.SOPSEnv, error) {
	sopsKeysDir := "/sopsenv"
	sopsEnv, err := konfigure.InitializeSopsEnvFromKubernetes(ctx, sopsKeysDir)

	if err != nil {
		return nil, err
	}

	err = sopsEnv.Setup(ctx)
	if err != nil {
		return sopsEnv, err
	}

	return sopsEnv, nil
}

func (r *ManagementClusterConfigurationReconciler) initializeFluxUpdater(fluxSource konfigurev1alpha1.FluxSource) (*fluxupdater.FluxUpdater, error) {
	// Konfigure cache
	cacheDir := "/tmp/konfigure-cache"

	// Default Flux installation source-controller URL
	sourceControllerUrl := "source-controller.flux-system.svc"
	if fluxSource.Service.Url != "" {
		sourceControllerUrl = fluxSource.Service.Url
	}

	fluxUpdater, err := konfigure.InitializeFluxUpdater(cacheDir, sourceControllerUrl, fluxSource.GitRepository.Namespace, fluxSource.GitRepository.Name)

	if err != nil {
		return nil, err
	}

	err = fluxUpdater.UpdateConfig()

	if err != nil {
		return fluxUpdater, err
	}

	return fluxUpdater, nil
}

func (r *ManagementClusterConfigurationReconciler) initializeKonfigure(ctx context.Context, sopsKeysDir, cacheDir, installation string) (*konfigureService.Service, error) {
	logger := log.FromContext(ctx)

	// Konfigure service
	service, err := konfigure.InitializeService(ctx, cacheDir, sopsKeysDir, installation)

	if err != nil {
		return nil, err
	}

	logger.Info("Konfigure service successfully initialized!")

	return service, err
}

func (r *ManagementClusterConfigurationReconciler) updateStatusOnSetupFailure(ctx context.Context, cr *konfigurev1alpha1.ManagementClusterConfiguration, err error) {
	cr.Status.ObservedGeneration = cr.ObjectMeta.Generation
	cr.Status.LastReconciledAt = time.Now().Format(time.RFC3339Nano)

	cr.Status.Conditions = []metav1.Condition{}

	cr.Status.Conditions = append(cr.Status.Conditions, metav1.Condition{
		Type:               logic.ReadyCondition,
		Status:             metav1.ConditionFalse,
		ObservedGeneration: cr.ObjectMeta.Generation,
		LastTransitionTime: metav1.NewTime(time.Now().UTC().Truncate(time.Second)),
		Reason:             logic.SetupFailedReason,
		Message:            fmt.Sprintf("Setup failed: %s", err.Error()),
	})
}

func (r *ManagementClusterConfigurationReconciler) renderAppConfiguration(ctx context.Context, service *konfigureService.Service, app, revision string, destination konfigurev1alpha1.Destination, ownershipLabels map[string]string) (*v1.ConfigMap, *v1.Secret, error) {
	name := app

	separator := ""
	if destination.Naming.UseSeparator {
		separator = "-"
	}

	if destination.Naming.Prefix != "" {
		name = destination.Naming.Prefix + separator + name
	}

	if destination.Naming.Suffix != "" {
		name = name + separator + destination.Naming.Suffix
	}

	return service.Generate(ctx, konfigureService.GenerateInput{
		App:         app,
		Name:        name,
		Namespace:   destination.Namespace,
		ExtraLabels: ownershipLabels,
		// If we don't set this to a non-empty string, konfigure will need git binary in container, but it would
		// fail anyway cos the pulled source from source-controller does not have the .git metadata.
		VersionOverride: revision,
	})
}

func (r *ManagementClusterConfigurationReconciler) canApplyConfigMap(ctx context.Context, configmap *v1.ConfigMap) error {
	existingObject := &v1.ConfigMap{}
	err := r.Client.Get(ctx, client.ObjectKeyFromObject(configmap), existingObject)

	if err != nil {
		if apiMachineryErrors.IsNotFound(err) {
			return nil
		}

		return err
	}

	if err = logic.MatchOwnership(existingObject.ObjectMeta, configmap.ObjectMeta); err != nil {
		return fmt.Errorf("desired configmap exists already and is owned by another object: %s", err.Error())
	}

	return nil
}

func (r *ManagementClusterConfigurationReconciler) applyConfigMap(ctx context.Context, configmap *v1.ConfigMap) error {
	desiredCm := v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      configmap.Name,
			Namespace: configmap.Namespace,
		},
	}

	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, &desiredCm, func() error {
		desiredCm.Labels = configmap.Labels

		desiredCm.Data = configmap.Data

		return nil
	})

	return err
}

func (r *ManagementClusterConfigurationReconciler) canApplySecret(ctx context.Context, secret *v1.Secret) error {
	existingObject := &v1.Secret{}
	err := r.Client.Get(ctx, client.ObjectKeyFromObject(secret), existingObject)

	if err != nil {
		if apiMachineryErrors.IsNotFound(err) {
			return nil
		}

		return err
	}

	if err = logic.MatchOwnership(existingObject.ObjectMeta, secret.ObjectMeta); err != nil {
		return fmt.Errorf("desired secret exists already and is owned by another object: %s", err.Error())
	}

	return nil
}

func (r *ManagementClusterConfigurationReconciler) applySecret(ctx context.Context, secret *v1.Secret) error {
	desiredSecret := v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secret.Name,
			Namespace: secret.Namespace,
		},
	}
	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, &desiredSecret, func() error {
		desiredSecret.Labels = secret.Labels

		desiredSecret.Data = secret.Data
		desiredSecret.StringData = secret.StringData

		return nil
	})

	return err
}

func (r *ManagementClusterConfigurationReconciler) finalize(ctx context.Context, cr *konfigurev1alpha1.ManagementClusterConfiguration) (ctrl.Result, error) {
	controllerutil.RemoveFinalizer(cr, konfigurev1alpha1.KonfigureOperatorFinalizer)

	err := r.Update(ctx, cr)

	if err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ManagementClusterConfigurationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&konfigurev1alpha1.ManagementClusterConfiguration{}, builder.WithPredicates(
			predicate.GenerationChangedPredicate{},
		)).
		Named("managementclusterconfiguration").
		Complete(r)
}
