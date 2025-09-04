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
	"io"
	"maps"
	"net/http"
	"os"
	"path"
	"slices"
	"strings"
	"time"

	"github.com/go-logr/logr"

	v1 "k8s.io/api/core/v1"

	"github.com/giantswarm/konfigure-operator/v2/internal/konfigure"

	konfigureModel "github.com/giantswarm/konfigure/v2/pkg/model"
	konfigureService "github.com/giantswarm/konfigure/v2/pkg/service"

	apiMachineryErrors "k8s.io/apimachinery/pkg/api/errors"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/giantswarm/konfigure-operator/v2/internal/controller/logic"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	konfigurev1alpha1 "github.com/giantswarm/konfigure-operator/v2/api/v1alpha1"
)

type KonfigurationReconcilerOptions struct {
	Verbose bool
}

// KonfigurationReconciler reconciles a Konfiguration object
type KonfigurationReconciler struct {
	client.Client
	Scheme  *runtime.Scheme
	Options KonfigurationReconcilerOptions
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
			logger.Info(fmt.Sprintf("Finished reconciling Konfiguration: %s/%s with status: %s/%s :: %s", cr.GetNamespace(), cr.GetName(), condition.Type, condition.Status, condition.Reason))
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
	var dynamicServiceLogger logr.Logger
	if r.Options.Verbose {
		dynamicServiceLogger = logger
	} else {
		dynamicServiceLogger = logr.Discard()
	}

	service := konfigureService.NewDynamicService(konfigureService.DynamicServiceConfig{
		Log: dynamicServiceLogger,
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

	revision, err := konfigure.GetLastArchiveSHA(fluxUpdater.CacheDir)
	if err != nil {
		logger.Error(err, fmt.Sprintf("Failed to get last archive SHA from: %s", fluxUpdater.CacheDir))
		revision = "unknown"
	}

	ownershipLabels := logic.GenerateOwnershipLabels(cr.GroupVersionKind(), cr.ObjectMeta, revision)

	// Render targets
	iterationNames := slices.Collect(maps.Keys(cr.Spec.Targets.Iterations))
	slices.Sort(iterationNames)

	failures := make(map[string]string)
	var disabledIterations []konfigurev1alpha1.DisabledIteration
	for _, iterationName := range iterationNames {
		iteration := cr.Spec.Targets.Iterations[iterationName]

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

		configmap, secret, err := service.Render(konfigureService.RenderInput{
			Dir:              path.Join(fluxUpdater.CacheDir, "latest"),
			Schema:           schemaFilePath,
			Variables:        rawVariables,
			Name:             cr.Spec.Destination.Naming.Render(iterationName),
			Namespace:        cr.Spec.Destination.Namespace,
			ConfigMapDataKey: konfigureModel.DefaultConfigMapDataKey,
			SecretDataKey:    konfigureModel.DefaultSecretDataKey,
			ExtraLabels:      ownershipLabels,
		})
		if err != nil {
			logger.Error(err, fmt.Sprintf("Failed to render iteration: %s with variables: %s", iterationName, strings.Join(rawVariables, ",")))

			failures[iterationName] = err.Error()

			RecordRendering(cr, iterationName, false)
			continue
		} else {
			RecordRendering(cr, iterationName, true)
		}

		logger.Info(fmt.Sprintf("Successfully rendered iteration: %s", iterationName))

		// Pre-flight check config map apply
		if err = r.canApplyConfigMap(ctx, configmap); err != nil {
			failures[iterationName] = err.Error()
		}

		// Pre-flight check secret apply. Present both errors to avoid the need to fix in multiple turns.
		if err = r.canApplySecret(ctx, secret); err != nil {
			if failures[iterationName] != "" {
				failures[iterationName] = failures[iterationName] + " " + err.Error()
			} else {
				failures[iterationName] = err.Error()
			}
		}

		if failures[iterationName] != "" {
			continue
		}

		shouldReconcile, err := r.applyConfigMap(ctx, configmap)
		if !shouldReconcile {
			logger.Info(fmt.Sprintf("Skipping apply for configmap %s/%s as it is disabled for reconciliation", configmap.Namespace, configmap.Name))

			disabledIterations = append(disabledIterations, konfigurev1alpha1.DisabledIteration{
				Name: iterationName,
				Kind: "ConfigMap",
				Target: konfigurev1alpha1.DisabledIterationTarget{
					Name:      secret.Name,
					Namespace: secret.Namespace,
				},
			})
		}

		if err != nil {
			logger.Error(err, fmt.Sprintf("Failed to apply configmap %s/%s for app: %s", configmap.Namespace, configmap.Name, iterationName))

			failures[iterationName] = err.Error()
			continue
		}

		shouldReconcile, err = r.applySecret(ctx, secret)
		if !shouldReconcile {
			logger.Info(fmt.Sprintf("Skipping apply for secret %s/%s as it is disabled for reconciliation", configmap.Namespace, configmap.Name))

			disabledIterations = append(disabledIterations, konfigurev1alpha1.DisabledIteration{
				Name: iterationName,
				Kind: "Secret",
				Target: konfigurev1alpha1.DisabledIterationTarget{
					Name:      secret.Name,
					Namespace: secret.Namespace,
				},
			})
		}

		if err != nil {
			logger.Error(err, fmt.Sprintf("Failed to apply secret %s/%s for app: %s", secret.Namespace, secret.Name, iterationName))

			failures[iterationName] = err.Error()
			continue
		}

		logger.Info(fmt.Sprintf("Successfully reconciled rendered configmap and secret for: %s", iterationName))
	}

	logger.Info(fmt.Sprintf("Failures: %s", failures))

	cr.Status.Failed = []konfigurev1alpha1.FailedIteration{}
	for failedIteration, failureMessage := range failures {
		cr.Status.Failed = append(cr.Status.Failed, konfigurev1alpha1.FailedIteration{
			Name:    failedIteration,
			Message: failureMessage,
		})
	}

	// Status update for disabled reconciliations
	cr.Status.Disabled = disabledIterations

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

	prefix := fmt.Sprintf("%s-%s", spec.Reference.Namespace, spec.Reference.Name)

	if schema.Spec.Raw.Remote.Url != "" {
		return r.fetchKonfigurationSchemaFromUrl(prefix, schema.Spec.Raw.Remote.Url)
	}

	return r.saveKonfigurationSchemaRawContentToTempFile(prefix, schema.Spec.Raw.Content)
}

func (r *KonfigurationReconciler) fetchKonfigurationSchemaFromUrl(prefix string, url string) (string, error) {
	request, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return "", err
	}

	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(response.Body)

	file, err := os.CreateTemp(KonfigurationSchemaDir, prefix)
	if err != nil {
		return "", err
	}

	_, err = io.Copy(file, response.Body)
	if err != nil {
		_ = os.Remove(file.Name())
		return "", err
	}

	return file.Name(), nil
}

func (r *KonfigurationReconciler) saveKonfigurationSchemaRawContentToTempFile(prefix string, content string) (string, error) {
	file, err := os.CreateTemp(KonfigurationSchemaDir, prefix)
	if err != nil {
		return "", err
	}

	_, err = file.WriteString(content)
	if err != nil {
		return "", err
	}

	return file.Name(), nil
}

func (r *KonfigurationReconciler) canApplyConfigMap(ctx context.Context, configmap *v1.ConfigMap) error {
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

func (r *KonfigurationReconciler) applyConfigMap(ctx context.Context, generatedConfigMap *v1.ConfigMap) (bool, error) {
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

func (r *KonfigurationReconciler) canApplySecret(ctx context.Context, generatedSecret *v1.Secret) error {
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

func (r *KonfigurationReconciler) applySecret(ctx context.Context, generatedSecret *v1.Secret) (bool, error) {
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
