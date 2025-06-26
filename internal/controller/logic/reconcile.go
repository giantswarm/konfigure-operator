package logic

import v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

const (
	ReconcileLabel = KonfigureOperatorPrefix + "/reconcile"
)

func ShouldReconcile(meta v1.ObjectMeta) bool {
	for label, value := range meta.Labels {
		if label == ReconcileLabel && value == DisabledValue {
			return false
		}
	}

	return true
}
