package controller

import (
	"strconv"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/prometheus/client_golang/prometheus"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/metrics"

	konfigurev1alpha1 "github.com/giantswarm/konfigure-operator/api/v1alpha1"
)

var renderingFixedLabels = []string{
	"resource_kind",
	"resource_name",
	"resource_namespace",
	"iteration_name",
	"destination_namespace",
}

// renderingCollector is an unchecked Prometheus collector for the rendering
// metric. Each iteration can declare its own custom labels via
// Iteration.MetricLabels, so the Desc is built per-sample at scrape time.
type renderingCollector struct {
	mu      sync.Mutex
	entries map[string]renderingEntry
}

type renderingEntry struct {
	fixedLabelValues []string
	customLabels     []konfigurev1alpha1.NameValuePair
	value            float64
}

func newRenderingCollector() *renderingCollector {
	return &renderingCollector{entries: make(map[string]renderingEntry)}
}

func (c *renderingCollector) Describe(_ chan<- *prometheus.Desc) {
	// Unchecked: descs are built per-sample in Collect so that each iteration
	// can carry its own custom label names.
}

func (c *renderingCollector) Collect(ch chan<- prometheus.Metric) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, entry := range c.entries {
		size := len(renderingFixedLabels) + len(entry.customLabels)
		labelNames := make([]string, 0, size)
		labelValues := make([]string, 0, size)

		labelNames = append(labelNames, renderingFixedLabels...)
		labelValues = append(labelValues, entry.fixedLabelValues...)
		for _, l := range entry.customLabels {
			labelNames = append(labelNames, l.Name)
			labelValues = append(labelValues, l.Value)
		}

		desc := prometheus.NewDesc(
			"konfigure_operator_rendering",
			"Configuration rendering status of a given iteration",
			labelNames,
			nil,
		)
		ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, entry.value, labelValues...)
	}
}

func (c *renderingCollector) set(key string, fixedLabelValues []string, customLabels []konfigurev1alpha1.NameValuePair, value float64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[key] = renderingEntry{
		fixedLabelValues: fixedLabelValues,
		customLabels:     customLabels,
		value:            value,
	}
}

var (
	conditionGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "konfigure_operator_reconcile_condition",
			Help: "The current condition status of a Konfigure Operator resource reconciliation.",
		},
		[]string{"resource_kind", "resource_name", "resource_namespace", "condition_type", "condition_status"},
	)

	generationGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "konfigure_operator_generation",
			Help: "Configuration generation status of a given app",
		},
		[]string{"resource_kind", "resource_name", "resource_namespace", "app_name", "config_cluster_name", "destination_namespace"},
	)

	renderingMetric = newRenderingCollector()

	reconcileDurationHistogram = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "konfigure_operator_reconcile_duration_seconds",
			Help: "The duration in seconds of a Konfigure Operator resource reconciliation.",
			// Use a histogram with 10 count buckets between 1ms - 1hour
			Buckets: prometheus.ExponentialBucketsRange(10e-3, 1800, 10),
		},
		[]string{"resource_kind", "resource_name", "resource_namespace"},
	)

	schemaFetchCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "konfigure_operator_schema_fetch_total",
			Help: "Total number of remote KonfigurationSchema fetch attempts, labelled by URL and HTTP status code (0 on transport error).",
		},
		[]string{"schema_url", "status_code"},
	)
)

func RecordConditions(gvk schema.GroupVersionKind, meta v1.ObjectMeta, conditions []v1.Condition) {
	for _, condition := range conditions {
		RecordCondition(gvk.Kind, meta, condition)
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

func RecordRendering(obj *konfigurev1alpha1.Konfiguration, iterationName string, metricLabels []konfigurev1alpha1.NameValuePair, success bool) {
	var value float64
	if success {
		value = 1
	}

	fixedLabelValues := []string{
		obj.Kind,
		obj.Name,
		obj.Namespace,
		iterationName,
		obj.Spec.Destination.Namespace,
	}

	key := obj.Kind + "/" + obj.Namespace + "/" + obj.Name + "/" + iterationName
	renderingMetric.set(key, fixedLabelValues, metricLabels, value)
}

func RecordReconcileDuration(gvk schema.GroupVersionKind, meta v1.ObjectMeta, start time.Time) {
	reconcileDurationHistogram.WithLabelValues(gvk.Kind, meta.Name, meta.Namespace).Observe(time.Since(start).Seconds())
}

func RecordSchemaFetch(schemaUrl string, statusCode int) {
	schemaFetchCounter.WithLabelValues(schemaUrl, strconv.Itoa(statusCode)).Inc()
}

func init() {
	metrics.Registry.MustRegister(conditionGauge, generationGauge, renderingMetric, reconcileDurationHistogram, schemaFetchCounter)
}
