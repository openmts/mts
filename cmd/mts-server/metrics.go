package main

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

type serverMetrics struct {
	mu       sync.Mutex
	requests map[string]*metricCounter
}

type metricCounter struct {
	Total        int64
	Errors       int64
	DurationNano int64
}

func newServerMetrics() *serverMetrics {
	return &serverMetrics{requests: make(map[string]*metricCounter)}
}

func (m *serverMetrics) observe(protocol string, route string, code string, duration time.Duration) {
	if m == nil {
		return
	}
	key := protocol + "\xff" + sanitizeMetricLabel(route) + "\xff" + sanitizeMetricLabel(code)
	m.mu.Lock()
	defer m.mu.Unlock()
	counter := m.requests[key]
	if counter == nil {
		counter = &metricCounter{}
		m.requests[key] = counter
	}
	counter.Total++
	if isErrorCode(code) {
		counter.Errors++
	}
	counter.DurationNano += duration.Nanoseconds()
}

func (m *serverMetrics) prometheusText() string {
	if m == nil {
		return ""
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	keys := make([]string, 0, len(m.requests))
	for key := range m.requests {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var builder strings.Builder
	builder.WriteString("# HELP mts_server_requests_total Server requests by protocol, route and code.\n")
	builder.WriteString("# TYPE mts_server_requests_total counter\n")
	builder.WriteString("# HELP mts_server_request_errors_total Server request errors by protocol, route and code.\n")
	builder.WriteString("# TYPE mts_server_request_errors_total counter\n")
	builder.WriteString("# HELP mts_server_request_duration_seconds_total Cumulative server request duration.\n")
	builder.WriteString("# TYPE mts_server_request_duration_seconds_total counter\n")
	for _, key := range keys {
		parts := strings.Split(key, "\xff")
		counter := m.requests[key]
		labels := fmt.Sprintf(`protocol="%s",route="%s",code="%s"`, parts[0], parts[1], parts[2])
		_, _ = fmt.Fprintf(&builder, "mts_server_requests_total{%s} %d\n", labels, counter.Total)
		_, _ = fmt.Fprintf(&builder, "mts_server_request_errors_total{%s} %d\n", labels, counter.Errors)
		_, _ = fmt.Fprintf(&builder, "mts_server_request_duration_seconds_total{%s} %.9f\n", labels, float64(counter.DurationNano)/float64(time.Second))
	}
	return builder.String()
}

func sanitizeMetricLabel(value string) string {
	value = strings.ReplaceAll(value, "\\", "\\\\")
	value = strings.ReplaceAll(value, "\"", "\\\"")
	value = strings.ReplaceAll(value, "\n", " ")
	if strings.TrimSpace(value) == "" {
		return "unknown"
	}
	return value
}

func isErrorCode(code string) bool {
	return strings.HasPrefix(code, "4") || strings.HasPrefix(code, "5") ||
		strings.EqualFold(code, "unauthenticated") ||
		strings.EqualFold(code, "permission_denied") ||
		strings.EqualFold(code, "resource_exhausted") ||
		strings.EqualFold(code, "internal")
}
