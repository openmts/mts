package observability

import (
	"strings"
	"testing"
)

func TestRegistryExportsPrometheusText(t *testing.T) {
	registry := NewRegistry()
	registry.AddCounter("mts_wal_records_total", "WAL records.", 2)
	registry.SetGauge("mts_memtable_samples", "MemTable samples.", 10)
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
}
