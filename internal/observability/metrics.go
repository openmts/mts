package observability

import "sync"

type Registry struct {
	mu      sync.RWMutex
	metrics map[string]*Metric
}

type Metric struct {
	Name  string
	Help  string
	Type  string
	Value float64
}

func NewRegistry() *Registry {
	return &Registry{metrics: make(map[string]*Metric)}
}

func (r *Registry) AddCounter(name string, help string, delta float64) {
	r.add(name, help, "counter", delta)
}

func (r *Registry) SetGauge(name string, help string, value float64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	metric := r.ensureMetricLocked(name, help, "gauge")
	metric.Value = value
}

func (r *Registry) ObserveHistogram(name string, help string, value float64) {
	r.add(name+"_sum", help, "counter", value)
	r.add(name+"_count", help, "counter", 1)
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

func (r *Registry) add(name string, help string, metricType string, delta float64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	metric := r.ensureMetricLocked(name, help, metricType)
	metric.Value += delta
}

func (r *Registry) ensureMetricLocked(name string, help string, metricType string) *Metric {
	metric, ok := r.metrics[name]
	if ok {
		return metric
	}
	metric = &Metric{Name: name, Help: help, Type: metricType}
	r.metrics[name] = metric
	return metric
}
