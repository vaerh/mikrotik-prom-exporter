package exporter

import (
	prom "github.com/prometheus/client_golang/prometheus"
)

type MetricType uint

const (
	CounterVec = "CounterVec"
	GaugeVec   = "GaugeVec"

	Int   = "int"
	Time  = "time"
	Const = "const"
	Bool  = "bool"

	OperAdd      = "Add"
	OperSub      = "Sub"
	OperInc      = "Inc"
	OperDec      = "Dec"
	OperSet      = "Set"
	OperCurrTime = "SetToCurrentTime"
)

type ResourceSchema struct {
	// https://pkg.go.dev/github.com/prometheus/client_golang/prometheus#BuildFQName
	PromNamespace string `yaml:"namespace"`
	PromSubsystem string `yaml:"subsystem"`
	// ResourcePath Resource path in routeros
	MikrotikResourcePath string `yaml:"resource_path"`
	// PromGlobalLabels Global map of labels and label values that will contain all resource metrics
	PromGlobalLabels prom.Labels `yaml:"global_labels,omitempty"`
	// ResourceFilter Filter executed on find to select interfaces (optional)
	ResourceFilter map[string]string `yaml:"resource_filter,omitempty"`

	Metrics []ResourceMetric `yaml:"metrics"`
}

type ResourceMetric struct {
	// PromMetricName Name of the metric to be created
	PromMetricName string `yaml:"name"`
	// PromMetricType
	PromMetricType string `yaml:"type"`
	// Metric operation
	PromMetricOperation string `yaml:"operation,omitempty"`
	// Reset metric to zero on new statistic collection
	PromResetGaugeEveryTime bool   `yaml:"reset_gauge,omitempty"`
	MtFieldName             string `yaml:"field,omitempty"`
	// MtFieldType The Mikrotik field from which the value will be retrieved
	// Haven't had time to look, if the types of incoming metrics to add are clearly defined,
	// then these fields are not needed.
	MtFieldType string `yaml:"field_type"`
	// PromLabels Private map of labels and label values that are constant for the metric
	PromLabels prom.Labels `yaml:"labels,omitempty"`
	// PromMetricHelp help description of the metric
	PromMetricHelp string `yaml:"help,omitempty"`

	labels      prom.Labels
	constLabels prom.Labels
}

func (m *ResourceMetric) GetLabels() []string {
	var res = make([]string, 0, len(m.labels))
	for key := range m.labels {
		res = append(res, key)
	}
	return res
}
