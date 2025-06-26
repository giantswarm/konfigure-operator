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
	"strings"
	"time"

	"github.com/giantswarm/konfigure/pkg/fluxupdater"
	"github.com/giantswarm/konfigure/pkg/sopsenv"
	v1 "k8s.io/api/core/v1"
	apiMachineryErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

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
func (r *ManagementClusterConfigurationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	reconcileStart := time.Now()

	// Get resource under reconciliation
	cr := &konfigurev1alpha1.ManagementClusterConfiguration{}
	if err := r.Get(ctx, req.NamespacedName, cr); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	logger.Info(fmt.Sprintf("Reconciling ManagementClusterConfiguration: %s/%s", cr.GetNamespace(), cr.GetName()))

	defer func() {
		RecordReconcileDuration(cr, reconcileStart)

		for _, condition := range cr.Status.Conditions {
			logger.Info(fmt.Sprintf("Finished Reconciling ManagementClusterConfiguration: %s/%s with status: %s/%s :: %s", cr.GetNamespace(), cr.GetName(), condition.Type, condition.Status, condition.Reason))
		}

		RecordConditions(cr)
	}()

	// Handle finalizer
	if cr.DeletionTimestamp.IsZero() {
		// The object is not being deleted, so if it does not have our finalizer,
		// then lets add the finalizer and update the object. This is equivalent
		// to registering our finalizer.
		if !controllerutil.ContainsFinalizer(cr, konfigurev1alpha1.KonfigureOperatorFinalizer) {
			logger.Info(fmt.Sprintf("Adding finalizer: %s to %s/%s", konfigurev1alpha1.KonfigureOperatorFinalizer, cr.GetNamespace(), cr.GetName()))

			finalizersUpdated := controllerutil.AddFinalizer(cr, konfigurev1alpha1.KonfigureOperatorFinalizer)
			if !finalizersUpdated {
				logger.Error(nil, fmt.Sprintf("Failed to add finalizer: %s to %s/%s", konfigurev1alpha1.KonfigureOperatorFinalizer, cr.GetNamespace(), cr.GetName()))
			}

			if err := r.Update(ctx, cr); err != nil {
				return ctrl.Result{}, err
			}

			return ctrl.Result{Requeue: true}, nil
		}
	} else {
		// The object is being deleted
		if controllerutil.ContainsFinalizer(cr, konfigurev1alpha1.KonfigureOperatorFinalizer) {
			// remove our finalizer from the list and update it.
			controllerutil.RemoveFinalizer(cr, konfigurev1alpha1.KonfigureOperatorFinalizer)
			if err := r.Update(ctx, cr); err != nil {
				return ctrl.Result{}, err
			}
		}

		// Stop reconciliation as the item is being deleted
		return ctrl.Result{}, nil
	}

	if cr.Spec.Reconciliation.Suspend {
		logger.Info("Reconciliation is suspended for this object, skipping until next update.")
		return ctrl.Result{}, nil
	}

	// Initialize Konfigure
	sops, err := r.initializeSopsEnv(ctx)
	if err != nil {
		if updateStatusErr := r.updateStatusOnSetupFailure(ctx, cr, err); updateStatusErr != nil {
			logger.Error(updateStatusErr, "Failed to update status on setup failure")
		}

		return ctrl.Result{RequeueAfter: cr.Spec.Reconciliation.RetryInterval.Duration}, err
	}
	logger.Info(fmt.Sprintf("SOPS environment successfully set up at: %s", sops.GetKeysDir()))

	fluxUpdater, err := r.initializeFluxUpdater(cr.Spec.Sources.Flux)
	if err != nil {
		if updateStatusErr := r.updateStatusOnSetupFailure(ctx, cr, err); updateStatusErr != nil {
			logger.Error(updateStatusErr, "Failed to update status on setup failure")
		}

		return ctrl.Result{RequeueAfter: cr.Spec.Reconciliation.RetryInterval.Duration}, err
	}
	logger.Info("Konfigure cache successfully updated!")

	service, err := r.initializeKonfigure(ctx, sops.GetKeysDir(), fluxUpdater.CacheDir, cr.Spec.Configuration.Cluster.Name)
	if err != nil {
		if updateStatusErr := r.updateStatusOnSetupFailure(ctx, cr, err); updateStatusErr != nil {
			logger.Error(updateStatusErr, "Failed to update status on setup failure")
		}

		return ctrl.Result{RequeueAfter: cr.Spec.Reconciliation.RetryInterval.Duration}, err
	}

	appsToRender, missedExactMatchers, err := logic.GetAppsToReconcile(service.GetDir(), &cr.Spec.Configuration)
	if err != nil {
		if updateStatusErr := r.updateStatusOnSetupFailure(ctx, cr, err); updateStatusErr != nil {
			logger.Error(updateStatusErr, "Failed to update status on setup failure")
		}

		return ctrl.Result{RequeueAfter: cr.Spec.Reconciliation.RetryInterval.Duration}, err
	}

	logger.Info(fmt.Sprintf("Apps to reconcile: %s", strings.Join(appsToRender, ",")))
	logger.Info(fmt.Sprintf("Missed exact matchers: %s", strings.Join(missedExactMatchers, ",")))

	revision, err := konfigure.GetLastArchiveSHA(fluxUpdater.CacheDir)
	if err != nil {
		logger.Error(err, fmt.Sprintf("Failed to get last archive SHA from: %s", service.GetDir()))
		revision = "unknown"
	}

	ownershipLabels := logic.GenerateOwnershipLabels(cr, revision)

	failures := make(map[string]string)
	var disabledReconciles []konfigurev1alpha1.DisabledReconcile
	for _, appToRender := range appsToRender {
		configmap, secret, err := r.renderAppConfiguration(ctx, service, appToRender, revision, cr.Spec.Destination, ownershipLabels)

		if err != nil {
			logger.Error(err, fmt.Sprintf("Failed to render app configuration for: %s", appToRender))

			failures[appToRender] = err.Error()

			RecordGeneration(cr, appToRender, false)
			continue
		} else {
			RecordGeneration(cr, appToRender, true)
		}

		logger.Info(fmt.Sprintf("Successfully rendered app configuration for: %s", appToRender))

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

		disabledReconcile, err := r.applyConfigMap(ctx, configmap)
		if disabledReconcile {
			logger.Info(fmt.Sprintf("Skipping apply for configmap %s/%s as it disabled for reconciliation", configmap.Namespace, configmap.Name))

			disabledReconciles = append(disabledReconciles, konfigurev1alpha1.DisabledReconcile{
				AppName: appToRender,
				Kind:    "ConfigMap",
				Target: konfigurev1alpha1.DisabledReconcileTarget{
					Name:      secret.Name,
					Namespace: secret.Namespace,
				},
			})
		}

		if err != nil {
			logger.Error(err, fmt.Sprintf("Failed to apply configmap %s/%s for app: %s", configmap.Namespace, configmap.Name, appToRender))

			failures[appToRender] = err.Error()
			continue
		}

		disabledReconcile, err = r.applySecret(ctx, secret)
		if disabledReconcile {
			logger.Info(fmt.Sprintf("Skipping apply for secret %s/%s as it disabled for reconciliation", configmap.Namespace, configmap.Name))

			disabledReconciles = append(disabledReconciles, konfigurev1alpha1.DisabledReconcile{
				AppName: appToRender,
				Kind:    "ConfigMap",
				Target: konfigurev1alpha1.DisabledReconcileTarget{
					Name:      secret.Name,
					Namespace: secret.Namespace,
				},
			})
		}

		if err != nil {
			logger.Error(err, fmt.Sprintf("Failed to apply secret %s/%s for app: %s", secret.Namespace, secret.Name, appToRender))

			failures[appToRender] = err.Error()
			continue
		}

		logger.Info(fmt.Sprintf("Successfully applied rendered configmap and secret for: %s", appToRender))
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

	// Status update for disabled reconciles
	cr.Status.DisabledReconciles = disabledReconciles

	// Status update for missed matchers
	cr.Status.Misses = missedExactMatchers

	cr.Status.ObservedGeneration = cr.Generation
	cr.Status.LastReconciledAt = time.Now().Format(time.RFC3339Nano)

	cr.Status.LastAttemptedRevision = revision

	cr.Status.Conditions = []metav1.Condition{}
	if len(failures) == 0 {
		cr.Status.LastAppliedRevision = revision

		cr.Status.Conditions = append(cr.Status.Conditions, metav1.Condition{
			Type:               logic.ReadyCondition,
			Status:             metav1.ConditionTrue,
			ObservedGeneration: cr.Generation,
			LastTransitionTime: metav1.NewTime(time.Now().UTC().Truncate(time.Second)),
			Reason:             logic.ReconciliationSucceededReason,
			Message:            fmt.Sprintf("Applied revision: %s", revision),
		})
	} else {
		cr.Status.Conditions = append(cr.Status.Conditions, metav1.Condition{
			Type:               logic.ReadyCondition,
			Status:             metav1.ConditionFalse,
			ObservedGeneration: cr.Generation,
			LastTransitionTime: metav1.NewTime(time.Now().UTC().Truncate(time.Second)),
			Reason:             logic.ReconciliationFailedReason,
			Message:            fmt.Sprintf("Attempted revision: %s", revision),
		})
	}

	err = r.Status().Update(ctx, cr)
	if err != nil {
		logger.Error(err, fmt.Sprintf("Failed to update status for: %s/%s", cr.GetNamespace(), cr.GetName()))
		return ctrl.Result{}, err
	}

	if len(failures) > 0 {
		logger.Info(fmt.Sprintf("Reconciliation finished in %s with %d failures, next run in %s", time.Since(reconcileStart).String(), len(failures), cr.Spec.Reconciliation.RetryInterval.Duration.String()))

		return ctrl.Result{RequeueAfter: cr.Spec.Reconciliation.RetryInterval.Duration}, nil
	}

	logger.Info(fmt.Sprintf("Reconciliation finished in %s, next run in %s", time.Since(reconcileStart).String(), cr.Spec.Reconciliation.Interval.Duration.String()))

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

func (r *ManagementClusterConfigurationReconciler) updateStatusOnSetupFailure(ctx context.Context, cr *konfigurev1alpha1.ManagementClusterConfiguration, err error) error {
	cr.Status.ObservedGeneration = cr.Generation
	cr.Status.LastReconciledAt = time.Now().Format(time.RFC3339Nano)

	cr.Status.Conditions = []metav1.Condition{}

	cr.Status.Conditions = append(cr.Status.Conditions, metav1.Condition{
		Type:               logic.ReadyCondition,
		Status:             metav1.ConditionFalse,
		ObservedGeneration: cr.Generation,
		LastTransitionTime: metav1.NewTime(time.Now().UTC().Truncate(time.Second)),
		Reason:             logic.SetupFailedReason,
		Message:            fmt.Sprintf("Setup failed: %s", err.Error()),
	})

	return r.Status().Update(ctx, cr)
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

	err := r.Get(ctx, client.ObjectKeyFromObject(configmap), existingObject)
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

func (r *ManagementClusterConfigurationReconciler) applyConfigMap(ctx context.Context, generatedConfigMap *v1.ConfigMap) (bool, error) {
	existingObject := &v1.ConfigMap{}

	err := r.Get(ctx, client.ObjectKeyFromObject(generatedConfigMap), existingObject)
	if err != nil && !apiMachineryErrors.IsNotFound(err) {
		return true, err
	}

	if !logic.ShouldReconcile(existingObject.ObjectMeta) {
		return false, nil
	}

	// Respect external annotations and labels.
	// Do it this way to avoid keeping a removed or renamed konfigure-operator annotation or label being kept forever.
	externalAnnotations := logic.FilterExternalFromMap(existingObject.Annotations)
	externalLabels := logic.FilterExternalFromMap(existingObject.Labels)

	desiredConfigMap := v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:        generatedConfigMap.Name,
			Namespace:   generatedConfigMap.Namespace,
			Annotations: externalAnnotations,
			Labels:      externalLabels,
		},
	}

	_, err = controllerutil.CreateOrUpdate(ctx, r.Client, &desiredConfigMap, func() error {
		// Enforce desired annotations
		for key, value := range generatedConfigMap.Annotations {
			desiredConfigMap.Annotations[key] = value
		}

		// Enforce desired labels
		for key, value := range generatedConfigMap.Labels {
			desiredConfigMap.Labels[key] = value
		}

		// Always enforce data
		desiredConfigMap.Data = generatedConfigMap.Data

		return nil
	})

	return true, err
}

func (r *ManagementClusterConfigurationReconciler) canApplySecret(ctx context.Context, generatedSecret *v1.Secret) error {
	existingObject := &v1.Secret{}

	err := r.Get(ctx, client.ObjectKeyFromObject(generatedSecret), existingObject)
	if err != nil {
		if apiMachineryErrors.IsNotFound(err) {
			return nil
		}

		return err
	}

	if err = logic.MatchOwnership(existingObject.ObjectMeta, generatedSecret.ObjectMeta); err != nil {
		return fmt.Errorf("desired secret exists already and is owned by another object: %s", err.Error())
	}

	return nil
}

func (r *ManagementClusterConfigurationReconciler) applySecret(ctx context.Context, generatedSecret *v1.Secret) (bool, error) {
	existingObject := &v1.Secret{}

	err := r.Get(ctx, client.ObjectKeyFromObject(generatedSecret), existingObject)
	if err != nil && !apiMachineryErrors.IsNotFound(err) {
		return true, err
	}

	if !logic.ShouldReconcile(existingObject.ObjectMeta) {
		return false, nil
	}

	// Respect external annotations and labels.
	// Do it this way to avoid keeping a removed or renamed konfigure-operator annotation or label being kept forever.
	externalAnnotations := logic.FilterExternalFromMap(existingObject.Annotations)
	externalLabels := logic.FilterExternalFromMap(existingObject.Labels)

	desiredSecret := v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:        generatedSecret.Name,
			Namespace:   generatedSecret.Namespace,
			Annotations: externalAnnotations,
			Labels:      externalLabels,
		},
	}

	_, err = controllerutil.CreateOrUpdate(ctx, r.Client, &desiredSecret, func() error {
		// Enforce desired annotations
		for key, value := range generatedSecret.Annotations {
			desiredSecret.Annotations[key] = value
		}

		// Enforce desired labels
		for key, value := range generatedSecret.Labels {
			desiredSecret.Labels[key] = value
		}

		// Always enforce data
		desiredSecret.Data = generatedSecret.Data
		desiredSecret.StringData = generatedSecret.StringData

		return nil
	})

	return true, err
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
