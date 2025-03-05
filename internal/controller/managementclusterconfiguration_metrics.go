package controller

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/metrics"

	konfigurev1alpha1 "github.com/giantswarm/konfigure-operator/api/v1alpha1"
)

var (
	conditionGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "konfigure_operator_reconcile_condition",
			Help: "The current condition status of a Konfigure Operator resource reconciliation.",
		},
		[]string{"config_kind", "config_name", "config_namespace", "condition_type", "condition_status"},
	)

	generationGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "konfigure_operator_generation",
			Help: "Configuration generation status of a given app",
		},
		[]string{"resource_kind", "resource_name", "resource_namespace", "app_name", "config_cluster_name", "destination_namespace"},
	)

	reconcileDurationHistogram = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "konfigure_operator_reconcile_duration_seconds",
			Help: "The duration in seconds of a Konfigure Operator resource reconciliation.",
			// Use a histogram with 10 count buckets between 1ms - 1hour
			Buckets: prometheus.ExponentialBucketsRange(10e-3, 1800, 10),
		},
		[]string{"config_kind", "config_name", "config_namespace"},
	)
)

func RecordConditions(obj *konfigurev1alpha1.ManagementClusterConfiguration) {
	for _, condition := range obj.Status.Conditions {
		RecordCondition(obj.Kind, obj.ObjectMeta, condition)
	}
}

func RecordCondition(kind string, meta v1.ObjectMeta, condition v1.Condition) {
	for _, status := range []v1.ConditionStatus{v1.ConditionTrue, v1.ConditionFalse, v1.ConditionUnknown} {
		var value float64
		if status == condition.Status {
			value = 1
		}

		conditionGauge.WithLabelValues(kind, meta.Name, meta.Namespace, condition.Type, string(status)).Set(value)
	}
}

func RecordGeneration(obj *konfigurev1alpha1.ManagementClusterConfiguration, app string, success bool) {
	var value float64
	if success {
		value = 1
	}

	generationGauge.WithLabelValues(obj.Kind, obj.Name, obj.Namespace, app, obj.Spec.Configuration.Cluster.Name, obj.Spec.Destination.Namespace).Set(value)
}

func RecordReconcileDuration(obj *konfigurev1alpha1.ManagementClusterConfiguration, start time.Time) {
	reconcileDurationHistogram.WithLabelValues(obj.Kind, obj.Name, obj.Namespace).Observe(time.Since(start).Seconds())
}

func init() {
	metrics.Registry.MustRegister(conditionGauge, generationGauge, reconcileDurationHistogram)
}
