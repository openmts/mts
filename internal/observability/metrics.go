package observability

import (
	"sync"

	"github.com/openmts/mts/internal/collections"
)

type Registry struct {
	mu      sync.RWMutex
	metrics map[string]*Metric
}

type Metric struct {
	Name   string
	Labels map[string]string
	Help   string
	Type   string
	Value  float64
}

func NewRegistry() *Registry {
	return &Registry{metrics: make(map[string]*Metric)}
}

func (r *Registry) AddCounter(name string, help string, delta float64) {
	r.add(name, nil, help, "counter", delta)
}

func (r *Registry) SetGauge(name string, help string, value float64) {
	r.SetGaugeLabels(name, nil, help, value)
}

func (r *Registry) AddCounterLabels(
	name string,
	labels map[string]string,
	help string,
	delta float64,
) {
	r.add(name, labels, help, "counter", delta)
}

func (r *Registry) SetGaugeLabels(
	name string,
	labels map[string]string,
	help string,
	value float64,
) {
	r.mu.Lock()
	defer r.mu.Unlock()
	metric := r.ensureMetricLocked(name, labels, help, "gauge")
	metric.Value = value
}

func (r *Registry) ObserveHistogram(name string, help string, value float64) {
	r.add(name+"_sum", nil, help, "counter", value)
	r.add(name+"_count", nil, help, "counter", 1)
}

func (r *Registry) Snapshot() []Metric {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Metric, 0, len(r.metrics))
	for _, metric := range r.metrics {
		out = append(out, *metric)
	}
	sortMetrics(out)
	return out
}

func (r *Registry) add(
	name string,
	labels map[string]string,
	help string,
	metricType string,
	delta float64,
) {
	r.mu.Lock()
	defer r.mu.Unlock()
	metric := r.ensureMetricLocked(name, labels, help, metricType)
	metric.Value += delta
}

func (r *Registry) ensureMetricLocked(
	name string,
	labels map[string]string,
	help string,
	metricType string,
) *Metric {
	key := metricKey(name, labels)
	metric, ok := r.metrics[key]
	if ok {
		return metric
	}
	metric = &Metric{Name: name, Labels: cloneLabels(labels), Help: help, Type: metricType}
	r.metrics[key] = metric
	return metric
}

func cloneLabels(labels map[string]string) map[string]string {
	return collections.CloneMapNilIfEmpty(labels)
}
