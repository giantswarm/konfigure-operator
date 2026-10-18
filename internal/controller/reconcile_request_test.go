package controller

import (
	"context"
	"errors"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/event"

	konfigurev1alpha1 "github.com/giantswarm/konfigure-operator/api/v1alpha1"
)

func konfWithAnnotations(generation int64, annotations map[string]string) *konfigurev1alpha1.Konfiguration {
	return &konfigurev1alpha1.Konfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "test",
			Namespace:   "default",
			Generation:  generation,
			Annotations: annotations,
		},
	}
}

func TestReconcileRequestedPredicateUpdate(t *testing.T) {
	key := konfigurev1alpha1.ReconcileRequestAnnotation

	testCases := []struct {
		name     string
		old, new map[string]string
		want     bool
	}{
		{name: "annotation added", old: nil, new: map[string]string{key: "2026-01-01T00:00:00Z"}, want: true},
		{name: "annotation changed", old: map[string]string{key: "a"}, new: map[string]string{key: "b"}, want: true},
		{name: "annotation unchanged", old: map[string]string{key: "a"}, new: map[string]string{key: "a"}, want: false},
		{name: "annotation removed", old: map[string]string{key: "a"}, new: nil, want: true},
		{name: "other annotation changed", old: map[string]string{"other": "a"}, new: map[string]string{"other": "b"}, want: false},
		{name: "no annotations", old: nil, new: nil, want: false},
	}

	p := ReconcileRequestedPredicate()
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := p.Update(event.UpdateEvent{
				ObjectOld: konfWithAnnotations(1, tc.old),
				ObjectNew: konfWithAnnotations(1, tc.new),
			})
			if got != tc.want {
				t.Fatalf("Update() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestReconcileRequestedPredicateCreateDelete(t *testing.T) {
	p := ReconcileRequestedPredicate()
	obj := konfWithAnnotations(1, map[string]string{konfigurev1alpha1.ReconcileRequestAnnotation: "a"})

	if !p.Create(event.CreateEvent{Object: obj}) {
		t.Error("Create() = false, want true")
	}
	if !p.Delete(event.DeleteEvent{Object: obj}) {
		t.Error("Delete() = false, want true")
	}
	if !p.Generic(event.GenericEvent{Object: obj}) {
		t.Error("Generic() = false, want true")
	}
}

func TestSetupFailureRecordsHandledReconcileRequest(t *testing.T) {
	const requested = "2026-10-18T10:11:12.123456789Z"

	scheme := runtime.NewScheme()
	if err := konfigurev1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("adding scheme: %v", err)
	}

	testCases := []struct {
		name        string
		annotations map[string]string
		want        string
	}{
		{name: "annotation present", annotations: map[string]string{konfigurev1alpha1.ReconcileRequestAnnotation: requested}, want: requested},
		{name: "annotation absent", annotations: nil, want: ""},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cr := konfWithAnnotations(1, tc.annotations)
			c := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(cr).
				WithStatusSubresource(&konfigurev1alpha1.Konfiguration{}).
				Build()
			r := &KonfigurationReconciler{Client: c}

			if err := r.updateStatusOnSetupFailure(context.Background(), cr, errors.New("boom")); err != nil {
				t.Fatalf("updateStatusOnSetupFailure: %v", err)
			}

			got := &konfigurev1alpha1.Konfiguration{}
			if err := c.Get(context.Background(), client.ObjectKeyFromObject(cr), got); err != nil {
				t.Fatalf("get: %v", err)
			}
			if got.Status.LastHandledReconcileAt != tc.want {
				t.Fatalf("LastHandledReconcileAt = %q, want %q", got.Status.LastHandledReconcileAt, tc.want)
			}
		})
	}
}

func TestRecordHandledReconcileRequest(t *testing.T) {
	cr := konfWithAnnotations(1, map[string]string{konfigurev1alpha1.ReconcileRequestAnnotation: "x"})
	recordHandledReconcileRequest(cr)
	if cr.Status.LastHandledReconcileAt != "x" {
		t.Fatalf("got %q, want x", cr.Status.LastHandledReconcileAt)
	}
}
