#!/usr/bin/env bash
set -euo pipefail

update_baseline=false
if [[ "${1:-}" == "--help" || "${1:-}" == "-h" ]]; then
  cat <<'USAGE'
usage: scripts/storage_benchmark_gate.sh [--update-baseline] [baseline] [output]

    Runs the 10K storage write and query iterator benchmarks.
Without --update-baseline, an existing baseline is compared with benchstat.
With --update-baseline, the current output is atomically written as baseline.
USAGE
  exit 0
fi

if [[ "${1:-}" == "--update-baseline" ]]; then
  update_baseline=true
  shift
fi

baseline="${1:-docs/benchmarks/storage-engine-baseline.txt}"
output="${2:-/tmp/mts-storage-benchmark.txt}"

go test ./internal/bench \
  -run '^$' \
  -bench 'BenchmarkEngine(Write(Batch|WideBatch)|Query(Row|Column)Iterator)/points=10000$' \
  -benchmem \
  -count=10 | tee "$output"

if [[ "$update_baseline" == true ]]; then
  baseline_dir="$(dirname "$baseline")"
  if [[ ! -d "$baseline_dir" ]]; then
    install -d -m 0700 "$baseline_dir"
  fi
  tmp="$(mktemp "${baseline}.tmp.XXXXXX")"
  cp "$output" "$tmp"
  chmod 0600 "$tmp"
  mv "$tmp" "$baseline"
  echo "updated baseline: $baseline"
  exit 0
fi

if [[ -f "$baseline" ]]; then
  benchstat "$baseline" "$output"
else
  echo "baseline not found: $baseline"
  echo "saved current benchmark to: $output"
fi
