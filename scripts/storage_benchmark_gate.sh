#!/usr/bin/env bash
set -euo pipefail

baseline="${1:-docs/benchmarks/storage-engine-baseline.txt}"
output="${2:-/tmp/mts-storage-benchmark.txt}"

go test ./internal/bench \
  -run '^$' \
  -bench 'BenchmarkEngineWrite(Batch|WideBatch)/points=10000$' \
  -benchmem \
  -count=10 | tee "$output"

if [[ -f "$baseline" ]]; then
  benchstat "$baseline" "$output"
else
  echo "baseline not found: $baseline"
  echo "saved current benchmark to: $output"
fi
