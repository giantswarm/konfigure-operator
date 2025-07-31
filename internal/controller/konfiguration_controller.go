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
	"strings"
	"time"

	konfigureModel "github.com/giantswarm/konfigure/pkg/model"
	konfigure "github.com/giantswarm/konfigure/pkg/service"

	apiMachineryErrors "k8s.io/apimachinery/pkg/api/errors"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/giantswarm/konfigure-operator/internal/controller/logic"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	konfigurev1alpha1 "github.com/giantswarm/konfigure-operator/api/v1alpha1"
)

// KonfigurationReconciler reconciles a Konfiguration object
type KonfigurationReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=konfigure.giantswarm.io,resources=konfigurations,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=konfigure.giantswarm.io,resources=konfigurations/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=konfigure.giantswarm.io,resources=konfigurations/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Konfiguration object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.20.2/pkg/reconcile
func (r *KonfigurationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	reconcileStart := time.Now()

	// Get resource under reconciliation
	cr := &konfigurev1alpha1.Konfiguration{}
	if err := r.Get(ctx, req.NamespacedName, cr); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	logger.Info(fmt.Sprintf("Reconciling Konfiguration: %s/%s", cr.GetNamespace(), cr.GetName()))

	defer func() {
		RecordReconcileDuration(cr.GroupVersionKind(), cr.ObjectMeta, reconcileStart)

		for _, condition := range cr.Status.Conditions {
			logger.Info(fmt.Sprintf("Finished Reconciling ManagementClusterConfiguration: %s/%s with status: %s/%s :: %s", cr.GetNamespace(), cr.GetName(), condition.Type, condition.Status, condition.Reason))
		}

		RecordConditions(cr.GroupVersionKind(), cr.ObjectMeta, cr.Status.Conditions)
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

	// Initialize SOPS Environment
	sops, err := InitializeSopsEnv(ctx, "/sopsenv/kfg")
	if err != nil {
		if updateStatusErr := r.updateStatusOnSetupFailure(ctx, cr, err); updateStatusErr != nil {
			logger.Error(updateStatusErr, "Failed to update status on setup failure")
		}

		return ctrl.Result{RequeueAfter: cr.Spec.Reconciliation.RetryInterval.Duration}, err
	}
	logger.Info(fmt.Sprintf("SOPS environment successfully set up at: %s", sops.GetKeysDir()))

	// Initialize Flux Updater
	fluxUpdater, err := InitializeFluxUpdater("/tmp/konfigure-cache/kfg", cr.Spec.Sources.Flux)
	if err != nil {
		if updateStatusErr := r.updateStatusOnSetupFailure(ctx, cr, err); updateStatusErr != nil {
			logger.Error(updateStatusErr, "Failed to update status on setup failure")
		}

		return ctrl.Result{RequeueAfter: cr.Spec.Reconciliation.RetryInterval.Duration}, err
	}
	logger.Info("Konfigure cache successfully updated!")

	// Initialize Dynamic Service
	service := konfigure.NewDynamicService(konfigure.DynamicServiceConfig{
		Log: logger,
	})

	// Fetch konfiguration schema
	schemaFilePath, err := r.fetchKonfigurationSchema(ctx, cr.Spec.Targets.Schema)

	defer func(name string) {
		if schemaFilePath == "" {
			return
		}

		err := os.Remove(name)
		if err != nil {
			logger.Error(err, fmt.Sprintf("Failed to remove temporary schema file: %s", schemaFilePath))
		}
	}(schemaFilePath)

	if err != nil {
		if updateStatusErr := r.updateStatusOnSetupFailure(ctx, cr, err); updateStatusErr != nil {
			logger.Error(updateStatusErr, "Failed to update status on setup failure")
		}

		return ctrl.Result{RequeueAfter: cr.Spec.Reconciliation.RetryInterval.Duration}, err
	}
	logger.Info(fmt.Sprintf("Konfiguration schema file path: %s", schemaFilePath))

	// Render targets
	failures := make(map[string]string)
	for _, iteration := range cr.Spec.Targets.Iterations {
		variables := make(map[string]string)

		for _, defaultVariable := range cr.Spec.Targets.Defaults.Variables {
			variables[defaultVariable.Name] = defaultVariable.Value
		}

		for _, valueOverride := range iteration.Variables {
			variables[valueOverride.Name] = valueOverride.Value
		}

		var rawVariables []string
		for k, v := range variables {
			rawVariables = append(rawVariables, fmt.Sprintf("%s=%s", k, v))
		}

		cm, secret, err := service.Render(konfigure.RenderInput{
			Dir:              path.Join(fluxUpdater.CacheDir, "latest"),
			Schema:           schemaFilePath,
			Variables:        rawVariables,
			Name:             cr.Spec.Destination.Naming.Render(iteration.Name),
			Namespace:        cr.Spec.Destination.Namespace,
			ConfigMapDataKey: konfigureModel.DefaultConfigMapDataKey,
			SecretDataKey:    konfigureModel.DefaultSecretDataKey,
		})
		if err != nil {
			logger.Error(err, fmt.Sprintf("Failed to render iteration: %s with variables: %s", iteration.Name, strings.Join(rawVariables, ",")))

			failures[iteration.Name] = err.Error()

			// TODO Record metric for generation
			continue
		}

		logger.Info(fmt.Sprintf("Succesfully rendered config map for %s iteration: %s", iteration.Name, cm))
		logger.Info(fmt.Sprintf("Succesfully rendered secret for %s iteration: %s", iteration.Name, secret))
	}

	return ctrl.Result{RequeueAfter: cr.Spec.Reconciliation.Interval.Duration}, nil
}

func (r *KonfigurationReconciler) updateStatusOnSetupFailure(ctx context.Context, cr *konfigurev1alpha1.Konfiguration, err error) error {
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

const (
	KonfigurationSchemaDir = "/tmp/konfiguration-schemas"
)

func (r *KonfigurationReconciler) fetchKonfigurationSchema(ctx context.Context, spec konfigurev1alpha1.Schema) (string, error) {
	schema := &konfigurev1alpha1.KonfigurationSchema{}
	err := r.Get(ctx, client.ObjectKey{Name: spec.Reference.Name, Namespace: spec.Reference.Namespace}, schema)
	if apiMachineryErrors.IsNotFound(err) {
		return "", fmt.Errorf("KonfigurationSchema %s/%s not found", spec.Reference.Namespace, spec.Reference.Name)
	}

	file, err := os.CreateTemp(KonfigurationSchemaDir, fmt.Sprintf("%s-%s", spec.Reference.Namespace, spec.Reference.Name))
	if err != nil {
		return "", err
	}

	_, err = file.WriteString(schema.Spec.Raw)
	if err != nil {
		return "", err
	}

	return file.Name(), err
}

// SetupWithManager sets up the controller with the Manager.
func (r *KonfigurationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	err := os.MkdirAll(KonfigurationSchemaDir, 0700)
	if err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&konfigurev1alpha1.Konfiguration{}, builder.WithPredicates(
			predicate.GenerationChangedPredicate{},
		)).
		Named("konfiguration").
		Complete(r)
}
