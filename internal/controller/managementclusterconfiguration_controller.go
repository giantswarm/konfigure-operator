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
	"os"
	"path"

	"github.com/giantswarm/konfigure/pkg/sopsenv"
	sopsenvKey "github.com/giantswarm/konfigure/pkg/sopsenv/key"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	controllerutil "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	konfigureFluxUpdater "github.com/giantswarm/konfigure/pkg/fluxupdater"
	konfigureService "github.com/giantswarm/konfigure/pkg/service"
	konfigureVaultClient "github.com/giantswarm/konfigure/pkg/vaultclient"

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

	logger.Info("Fetching ManagementClusterConfiguration")

	configurationCr := &konfigurev1alpha1.ManagementClusterConfiguration{}
	err := r.Get(ctx, req.NamespacedName, configurationCr)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// If the custom resource is not found then it usually means that it was deleted or not created
			// In this way, we will stop the reconciliation
			logger.Info("ManagementClusterConfiguration resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		logger.Error(err, "Failed to get ManagementClusterConfiguration")
		return ctrl.Result{}, err
	}

	logger.Info(fmt.Sprintf("Reconciling ManagementClusterConfiguration: %s/%s", configurationCr.GetNamespace(), configurationCr.GetName()))

	// SOPS environment
	cfg := sopsenv.SOPSEnvConfig{
		KeysDir:    "/sopsenv",
		KeysSource: sopsenvKey.KeysSourceKubernetes,
		Logger:     logger,
	}

	sopsEnv, err := sopsenv.NewSOPSEnv(cfg)
	if err != nil {
		return ctrl.Result{}, err
	}

	err = sopsEnv.Setup(ctx)
	if err != nil {
		return ctrl.Result{}, err
	}

	logger.Info(fmt.Sprintf("SOPS environment successfully set up at: %s", sopsEnv.GetKeysDir()))

	// Konfigure cache
	cacheDir := "/tmp/konfigure-cache"
	updater, err := konfigureFluxUpdater.New(konfigureFluxUpdater.Config{
		CacheDir:                cacheDir,
		ApiServerHost:           os.Getenv("KUBERNETES_SERVICE_HOST"),
		ApiServerPort:           os.Getenv("KUBERNETES_SERVICE_PORT"),
		SourceControllerService: "source-controller.flux-giantswarm.svc",
		GitRepository:           "flux-giantswarm/giantswarm-config",
	})

	if err != nil {
		return ctrl.Result{}, err
	}

	err = updater.UpdateConfig()
	if err != nil {
		return ctrl.Result{}, err
	}

	logger.Info("Konfigure cache successfully updated")

	// Setting up konfigure service
	// TODO It would be nice to be able to create the service vaultless
	vaultClient, err := konfigureVaultClient.NewClientUsingEnv(ctx)

	konfigure, err := konfigureService.New(konfigureService.Config{
		VaultClient: vaultClient,

		Log:            logger,
		Dir:            path.Join(cacheDir, "latest"),
		Installation:   configurationCr.Spec.Configuration.Cluster.Name,
		SOPSKeysDir:    "/sopsenv",
		SOPSKeysSource: "local",
		Verbose:        true,
	})

	if err != nil {
		return ctrl.Result{}, err
	}

	cm, secret, err := konfigure.Generate(ctx, konfigureService.GenerateInput{
		App:       "app-operator",
		Name:      "laszlo-test",
		Namespace: "default",
		// Must set, keep it main or maybe fetch from the string in /tmp/konfigure-cache/lastarchive
		// If we don't set this to a non-empty string, konfigure will need git binary in container, but it would
		// fault anyway cos the pulled source from source-controller does not have the .git metadata.
		VersionOverride: "main",
	})

	if err != nil {
		logger.Error(err, fmt.Sprintf("Failed to generate CM and Secret: %s", err))
		return ctrl.Result{}, err
	}

	logger.Info("Successfully generated CM and Secret!")

	logger.Info(cm.String())

	desiredCm := v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "laszlo-test",
			Namespace: "default",
		},
	}
	_, err = controllerutil.CreateOrUpdate(ctx, r.Client, &desiredCm, func() error {
		desiredCm.Data = cm.Data
		return nil
	})
	if err != nil {
		logger.Error(err, fmt.Sprintf("Failed to create or update CM: %s", err))
		return ctrl.Result{}, err
	} else {
		logger.Info("Successfully created or updated CM!")
	}

	logger.Info(secret.String())

	desiredSecret := v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "laszlo-test",
			Namespace: "default",
		},
	}

	_, err = controllerutil.CreateOrUpdate(ctx, r.Client, &desiredSecret, func() error {
		desiredSecret.Data = secret.Data
		desiredSecret.StringData = secret.StringData
		return nil
	})

	if err != nil {
		logger.Error(err, fmt.Sprintf("Failed to create or update Secret: %s", err))
		return ctrl.Result{}, err
	} else {
		logger.Info("Successfully created or updated Secret!")
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ManagementClusterConfigurationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&konfigurev1alpha1.ManagementClusterConfiguration{}).
		Named("managementclusterconfiguration").
		Complete(r)
}
