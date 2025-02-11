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
	"os"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	konfigureFluxUpdater "github.com/giantswarm/konfigure/pkg/fluxupdater"

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

	logger.Info("Reconciling ManagementClusterConfiguration")

	updater, err := konfigureFluxUpdater.New(konfigureFluxUpdater.Config{
		CacheDir:                "/tmp/konfigure-cache",
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

	//konfigure, err := konfigureService.New(konfigureService.Config{
	//	Log: logger,
	//})
	//
	//if err != nil {
	//	return ctrl.Result{}, err
	//}
	//
	//cm, secret, err := konfigure.Generate(ctx, konfigureService.GenerateInput{})
	//
	//if err != nil {
	//	return ctrl.Result{}, err
	//}
	//
	//logger.Info(cm.String())
	//logger.Info(secret.String())

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ManagementClusterConfigurationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&konfigurev1alpha1.ManagementClusterConfiguration{}).
		Named("managementclusterconfiguration").
		Complete(r)
}
