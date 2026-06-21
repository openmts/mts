package observability

import (
	"strings"
	"testing"
)

func TestRegistryExportsPrometheusText(t *testing.T) {
	registry := NewRegistry()
	registry.AddCounter("mts_wal_records_total", "WAL records.", 2)
	registry.SetGauge("mts_memtable_samples", "MemTable samples.", 10)
	registry.AddCounter("mts_downsample_runs_total", "Downsample runs.", 3)
	registry.SetGaugeLabels(
		"mts_downsample_policy_lag_seconds",
		map[string]string{"policy": "cpu_1m"},
		"Policy lag.",
		12,
	)
	registry.ObserveHistogram("mts_query_duration_seconds", "Query duration.", 1.5)
	text := PrometheusText(registry.Snapshot())
	if !strings.Contains(text, "# TYPE mts_wal_records_total counter") {
		t.Fatalf("metrics text missing counter type:\n%s", text)
	}
	if !strings.Contains(text, "mts_memtable_samples 10") {
		t.Fatalf("metrics text missing gauge value:\n%s", text)
	}
	if !strings.Contains(text, "mts_query_duration_seconds_sum 1.5") {
		t.Fatalf("metrics text missing histogram sum:\n%s", text)
	}
	if !strings.Contains(text, "mts_query_duration_seconds_count 1") {
		t.Fatalf("metrics text missing histogram count:\n%s", text)
	}
	if !strings.Contains(text, "mts_downsample_runs_total 3") {
		t.Fatalf("metrics text missing downsample counter:\n%s", text)
	}
	if !strings.Contains(text, "mts_downsample_policy_lag_seconds{policy=\"cpu_1m\"} 12") {
		t.Fatalf("metrics text missing labeled policy gauge:\n%s", text)
	}
}
