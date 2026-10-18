package controller

import (
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	konfigurev1alpha1 "github.com/giantswarm/konfigure-operator/api/v1alpha1"
)

// ReconcileRequestedPredicate passes update events where the reconcile request
// annotation value changed, including the annotation being added or removed. Other event types pass, as with GenerationChangedPredicate.
func ReconcileRequestedPredicate() predicate.Funcs {
	return predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			if e.ObjectOld == nil || e.ObjectNew == nil {
				return false
			}

			oldValue, oldOk := e.ObjectOld.GetAnnotations()[konfigurev1alpha1.ReconcileRequestAnnotation]
			newValue, newOk := e.ObjectNew.GetAnnotations()[konfigurev1alpha1.ReconcileRequestAnnotation]

			return oldOk != newOk || oldValue != newValue
		},
	}
}

// recordHandledReconcileRequest copies the requested annotation value into status.
func recordHandledReconcileRequest(cr *konfigurev1alpha1.Konfiguration) {
	if value, ok := cr.GetAnnotations()[konfigurev1alpha1.ReconcileRequestAnnotation]; ok {
		cr.Status.LastHandledReconcileAt = value
	}
}
