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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"strings"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
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

	// Get resource under reconciliation
	cr, err := r.getCustomResource(ctx, req)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// If the custom resource is not found then it usually means that it was deleted or not created
			// In this way, we will stop the reconciliation
			logger.Info("ManagementClusterConfiguration resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, err
	}

	if cr.Generation == cr.Status.ObservedGeneration {
		logger.Info(fmt.Sprintf("Generation matches observed generation, skipping reconciliation for: %s/%s", cr.Namespace, cr.Name))
		return ctrl.Result{}, nil
	}

	// Initialize Konfigure
	sops, err := r.initializeSopsEnv(ctx)
	if err != nil {
		return ctrl.Result{}, err
	}
	logger.Info(fmt.Sprintf("SOPS environment successfully set up at: %s", sops.GetKeysDir()))

	fluxUpdater, err := r.initializeFluxUpdater(ctx, cr.Spec.Sources.Flux)
	if err != nil {
		return ctrl.Result{}, err
	}
	logger.Info("Konfigure cache successfully updated!")

	service, err := r.initializeKonfigure(ctx, sops.GetKeysDir(), fluxUpdater.CacheDir, cr.Spec.Configuration.Cluster.Name)
	if err != nil {
		return ctrl.Result{}, err
	}

	appsToRender, missedExactMatchers, err := logic.GetAppsToReconcile(service.GetDir(), &cr.Spec.Configuration)

	logger.Info(fmt.Sprintf("Apps to reconcile: %s", strings.Join(appsToRender, ",")))
	logger.Info(fmt.Sprintf("Missed exact matchers: %s", strings.Join(missedExactMatchers, ",")))

	// TODO Handles misses for status updates

	failures := make(map[string]string)
	for _, appToRender := range appsToRender {
		configmap, secret, err := r.renderAppConfiguration(ctx, service, appToRender, cr.Spec.Destination.Namespace)

		if err != nil {
			// TODO Collect for status updates
			logger.Error(err, fmt.Sprintf("Failed to render app configuration for: %s", appToRender))

			failures[appToRender] = err.Error()
			continue
		}

		logger.Info(fmt.Sprintf("Succesfully rendered app configuration for: %s", appToRender))

		//logger.Info(fmt.Sprintf("ConfigMap for %s: %s", appToRender, configmap))
		//logger.Info(fmt.Sprintf("Secret for %s: %s", appToRender, secret))

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

	revision, err := konfigure.GetLastArchiveSHA(fluxUpdater.CacheDir)
	if err != nil {
		logger.Error(err, fmt.Sprintf("Failed to get last archive SHA from: %s", service.GetDir()))
		revision = "unknown"
	}

	cr.Status.LastAttemptedRevision = revision

	cr.Status.Conditions = []metav1.Condition{}
	if len(failures) == 0 {
		cr.Status.LastAppliedRevision = revision

		cr.Status.Conditions = append(cr.Status.Conditions, metav1.Condition{
			Type:               "Ready",
			Status:             metav1.ConditionTrue,
			ObservedGeneration: cr.ObjectMeta.Generation,
			LastTransitionTime: metav1.NewTime(time.Now().UTC().Truncate(time.Second)),
			Reason:             "ReconciliationSucceeded",
			Message:            fmt.Sprintf("Applied revision: %s", revision),
		})
	} else {
		cr.Status.Conditions = append(cr.Status.Conditions, metav1.Condition{
			Type:               "Ready",
			Status:             metav1.ConditionFalse,
			ObservedGeneration: cr.ObjectMeta.Generation,
			LastTransitionTime: metav1.NewTime(time.Now().UTC().Truncate(time.Second)),
			Reason:             "ReconciliationFailed",
			Message:            fmt.Sprintf("Attempted revision: %s", revision),
		})
	}

	err = r.Status().Update(ctx, cr)
	if err != nil {
		logger.Error(err, fmt.Sprintf("Failed to update status for: %s/%s", cr.GetNamespace(), cr.GetName()))
		return ctrl.Result{}, err
	}

	// TODO Handle reschedule based on interval or differently in case if failures.

	return ctrl.Result{}, nil
}

func (r *ManagementClusterConfigurationReconciler) getCustomResource(ctx context.Context, req ctrl.Request) (*konfigurev1alpha1.ManagementClusterConfiguration, error) {
	logger := log.FromContext(ctx)

	cr := &konfigurev1alpha1.ManagementClusterConfiguration{}
	err := r.Get(ctx, req.NamespacedName, cr)

	if err != nil {
		return nil, err
	}

	logger.Info(fmt.Sprintf("Reconciling ManagementClusterConfiguration: %s/%s", cr.GetNamespace(), cr.GetName()))

	return cr, nil
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

func (r *ManagementClusterConfigurationReconciler) initializeFluxUpdater(ctx context.Context, fluxSource konfigurev1alpha1.FluxSource) (*fluxupdater.FluxUpdater, error) {
	// Konfigure cache
	cacheDir := "/tmp/konfigure-cache"

	fluxUpdater, err := konfigure.InitializeFluxUpdater(cacheDir, fluxSource.Service.Url, fluxSource.GitRepository.Namespace, fluxSource.GitRepository.Name)

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

func (r *ManagementClusterConfigurationReconciler) renderAppConfiguration(ctx context.Context, service *konfigureService.Service, app, targetNamespace string) (*v1.ConfigMap, *v1.Secret, error) {
	return service.Generate(ctx, konfigureService.GenerateInput{
		App: app,
		// TODO Generate unique name
		Name:      fmt.Sprintf("%s-%s", app, "laszlo-test"),
		Namespace: targetNamespace,
		// Must set, keep it main or maybe fetch from the string in /tmp/konfigure-cache/lastarchive
		// If we don't set this to a non-empty string, konfigure will need git binary in container, but it would
		// fault anyway cos the pulled source from source-controller does not have the .git metadata.
		VersionOverride: "main",
	})
}

func (r *ManagementClusterConfigurationReconciler) applyConfigMap(ctx context.Context, configmap *v1.ConfigMap) error {
	desiredCm := v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      configmap.Name,
			Namespace: configmap.Namespace,
		},
	}
	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, &desiredCm, func() error {
		desiredCm.Data = configmap.Data
		return nil
	})

	return err
}

func (r *ManagementClusterConfigurationReconciler) applySecret(ctx context.Context, secret *v1.Secret) error {
	desiredSecret := v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secret.Name,
			Namespace: secret.Namespace,
		},
	}

	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, &desiredSecret, func() error {
		desiredSecret.Data = secret.Data
		desiredSecret.StringData = secret.StringData
		return nil
	})

	return err
}

// SetupWithManager sets up the controller with the Manager.
func (r *ManagementClusterConfigurationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&konfigurev1alpha1.ManagementClusterConfiguration{}).
		Named("managementclusterconfiguration").
		Complete(r)
}
