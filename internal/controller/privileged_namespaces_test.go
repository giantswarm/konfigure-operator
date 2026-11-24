package controller

import (
	"context"
	"strings"
	"testing"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	konfigurev1alpha1 "github.com/giantswarm/konfigure-operator/api/v1alpha1"
	"github.com/giantswarm/konfigure-operator/internal/controller/logic"
)

// A Konfiguration outside the privileged namespaces must be refused before the operator loads its
// SOPS keys or fetches the source and schema, even when it only references its own namespace, and
// without getting the operator's finalizer. Suspending it does not hide the refusal.
func TestReconcileRefusesUnprivilegedNamespaces(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := konfigurev1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("adding scheme: %v", err)
	}

	cr := &konfigurev1alpha1.Konfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "test",
			Namespace:  "org-acme",
			Generation: 1,
		},
		Spec: konfigurev1alpha1.KonfigurationSpec{
			Sources: konfigurev1alpha1.Sources{
				Flux: konfigurev1alpha1.FluxSource{
					GitRepository: konfigurev1alpha1.FluxSourceGitRepository{Name: "config", Namespace: "org-acme"},
				},
			},
			Targets: konfigurev1alpha1.Targets{
				Schema: konfigurev1alpha1.Schema{
					Reference: konfigurev1alpha1.SchemaReference{Name: "schema", Namespace: "org-acme"},
				},
			},
			Destination:    konfigurev1alpha1.Destination{Namespace: "org-acme"},
			Reconciliation: konfigurev1alpha1.Reconciliation{Suspend: true},
		},
		Status: konfigurev1alpha1.KonfigurationStatus{
			Failed: []konfigurev1alpha1.FailedIteration{{Name: "app", Message: "earlier failure"}},
		},
	}

	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(cr).
		WithStatusSubresource(&konfigurev1alpha1.Konfiguration{}).
		Build()
	r := &KonfigurationReconciler{
		Client:  c,
		Scheme:  scheme,
		Options: KonfigurationReconcilerOptions{PrivilegedNamespaces: []string{"giantswarm", "flux-giantswarm"}},
	}

	key := types.NamespacedName{Name: cr.Name, Namespace: cr.Namespace}
	result, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: key})
	if err != nil {
		t.Fatalf("Reconcile returned error: %v", err)
	}
	if result != (ctrl.Result{}) {
		t.Fatalf("Reconcile result = %+v, want no requeue", result)
	}

	got := &konfigurev1alpha1.Konfiguration{}
	if err := c.Get(context.Background(), client.ObjectKeyFromObject(cr), got); err != nil {
		t.Fatalf("get: %v", err)
	}
	if len(got.Status.Conditions) != 1 {
		t.Fatalf("conditions = %+v, want exactly one", got.Status.Conditions)
	}

	condition := got.Status.Conditions[0]
	if condition.Type != logic.ReadyCondition || condition.Status != metav1.ConditionFalse {
		t.Errorf("condition = %s/%s, want %s/False", condition.Type, condition.Status, logic.ReadyCondition)
	}
	if condition.Reason != logic.NamespaceNotAllowedReason {
		t.Errorf("reason = %q, want %q", condition.Reason, logic.NamespaceNotAllowedReason)
	}
	if !strings.Contains(condition.Message, "org-acme") {
		t.Errorf("message %q does not name the namespace", condition.Message)
	}
	if len(got.Finalizers) != 0 {
		t.Errorf("finalizers = %v, want none on a refused Konfiguration", got.Finalizers)
	}
	if len(got.Status.Failed) != 0 {
		t.Errorf("failed iterations = %+v, want them cleared", got.Status.Failed)
	}
}

func newTestReconciler(t *testing.T, objects ...client.Object) (*KonfigurationReconciler, client.Client) {
	t.Helper()

	scheme := runtime.NewScheme()
	if err := konfigurev1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("adding scheme: %v", err)
	}

	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(objects...).
		WithStatusSubresource(&konfigurev1alpha1.Konfiguration{}).
		Build()

	return &KonfigurationReconciler{
		Client:  c,
		Scheme:  scheme,
		Options: KonfigurationReconcilerOptions{PrivilegedNamespaces: []string{"giantswarm", "flux-giantswarm"}},
	}, c
}

func referencingOwnNamespace(name, namespace string) *konfigurev1alpha1.Konfiguration {
	return &konfigurev1alpha1.Konfiguration{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace, Generation: 1},
		Spec: konfigurev1alpha1.KonfigurationSpec{
			Sources: konfigurev1alpha1.Sources{
				Flux: konfigurev1alpha1.FluxSource{
					GitRepository: konfigurev1alpha1.FluxSourceGitRepository{Name: "config", Namespace: namespace},
				},
			},
			Targets: konfigurev1alpha1.Targets{
				Schema: konfigurev1alpha1.Schema{
					Reference: konfigurev1alpha1.SchemaReference{Name: "schema", Namespace: namespace},
				},
			},
			Destination: konfigurev1alpha1.Destination{Namespace: namespace},
		},
	}
}

// A Konfiguration that got the finalizer before the operator started refusing it must still be
// deletable.
func TestReconcileRemovesFinalizerFromDeletedUnprivilegedKonfiguration(t *testing.T) {
	cr := referencingOwnNamespace("test", "org-acme")
	cr.Finalizers = []string{konfigurev1alpha1.KonfigureOperatorFinalizer}
	now := metav1.Now()
	cr.DeletionTimestamp = &now

	r, c := newTestReconciler(t, cr)

	if _, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKeyFromObject(cr)}); err != nil {
		t.Fatalf("Reconcile returned error: %v", err)
	}

	got := &konfigurev1alpha1.Konfiguration{}
	err := c.Get(context.Background(), client.ObjectKeyFromObject(cr), got)
	if !apierrors.IsNotFound(err) {
		t.Fatalf("get = %v, finalizers %v; want the object gone", err, got.Finalizers)
	}
}

// A suspended Konfiguration that was refused and is now in a privileged namespace must not keep
// showing the refusal.
func TestReconcileClearsStaleRefusalOnSuspendedKonfiguration(t *testing.T) {
	cr := referencingOwnNamespace("test", "giantswarm")
	cr.Finalizers = []string{konfigurev1alpha1.KonfigureOperatorFinalizer}
	cr.Spec.Reconciliation.Suspend = true
	cr.Status.Conditions = []metav1.Condition{{
		Type:               logic.ReadyCondition,
		Status:             metav1.ConditionFalse,
		Reason:             logic.NamespaceNotAllowedReason,
		Message:            "refused earlier",
		LastTransitionTime: metav1.Now(),
	}}

	r, c := newTestReconciler(t, cr)

	if _, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKeyFromObject(cr)}); err != nil {
		t.Fatalf("Reconcile returned error: %v", err)
	}

	got := &konfigurev1alpha1.Konfiguration{}
	if err := c.Get(context.Background(), client.ObjectKeyFromObject(cr), got); err != nil {
		t.Fatalf("get: %v", err)
	}
	if len(got.Status.Conditions) != 0 {
		t.Fatalf("conditions = %+v, want the stale refusal cleared", got.Status.Conditions)
	}
}
