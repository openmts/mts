#!/usr/bin/env bash
set -euo pipefail
umask 077

readonly default_baseline='docs/benchmarks/storage-engine-baseline.txt'
readonly default_output="${TMPDIR:-/tmp}/mts-storage-benchmark.txt"
readonly benchmark_pattern='BenchmarkEngine(Write(Batch|WideBatch)|Query(Row|Column)Iterator)/points=10000$'
readonly sample_count=10
readonly max_time_regression_percent=10

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

baseline="${1:-$default_baseline}"
output="${2:-$default_output}"
go_cmd="${MTS_BENCH_GO:-go}"
benchstat_cmd="${MTS_BENCHSTAT:-benchstat}"
benchmark_timeout="${MTS_BENCH_TIMEOUT:-5m}"
baseline_tmp=''
benchstat_csv=''

cleanup() {
  if [[ -n "$baseline_tmp" && -f "$baseline_tmp" ]]; then
    unlink "$baseline_tmp"
  fi
  if [[ -n "$benchstat_csv" && -f "$benchstat_csv" ]]; then
    unlink "$benchstat_csv"
  fi
}
trap cleanup EXIT

require_tool() {
  local tool="$1"
  if ! command -v "$tool" >/dev/null 2>&1; then
    echo "required tool not found: $tool" >&2
    return 1
  fi
}

current_governor() {
  if [[ -r /sys/devices/system/cpu/cpu0/cpufreq/scaling_governor ]]; then
    cat /sys/devices/system/cpu/cpu0/cpufreq/scaling_governor
    return
  fi
  printf 'unknown\n'
}

write_metadata() {
  local target="$1"
  local governor
  governor="$(current_governor)"
  {
    printf '# mts benchmark baseline\n'
    printf '# generated_at_utc: %s\n' "$(date -u +'%Y-%m-%dT%H:%M:%SZ')"
    printf '# go_version: %s\n' "$($go_cmd version)"
    printf '# cpu_governor: %s\n' "$governor"
    printf '# parameters: package=./internal/bench count=%d benchmem=true pattern=%s\n' \
      "$sample_count" "$benchmark_pattern"
    cat "$output"
  } >"$target"
}

validate_benchmark_output() {
  local input="$1"
  local label="$2"
  awk -v label="$label" -v samples="$sample_count" '
    BEGIN {
      expected["BenchmarkEngineWriteBatch/points=10000"] = 1
      expected["BenchmarkEngineWriteWideBatch/points=10000"] = 1
      expected["BenchmarkEngineQueryRowIterator/points=10000"] = 1
      expected["BenchmarkEngineQueryColumnIterator/points=10000"] = 1
    }
    /^Benchmark/ {
      name = $1
      sub(/-[0-9]+$/, "", name)
      if (name in expected) count[name]++
    }
    END {
      failed = 0
      for (name in expected) {
        if (count[name] != samples) {
          printf "invalid benchmark samples: file=%s benchmark=%s got=%d want=%d\n", label, name, count[name], samples > "/dev/stderr"
          failed = 1
        }
      }
      exit failed
    }
  ' "$input"
}

benchmark_config() {
  local input="$1"
  awk -F': ' '
    $1 == "goos" || $1 == "goarch" || $1 == "cpu" { config[$1] = $2 }
    END { printf "goos=%s;goarch=%s;cpu=%s", config["goos"], config["goarch"], config["cpu"] }
  ' "$input"
}

validate_environment() {
  local baseline_config current_config baseline_go current_go baseline_governor current_governor_value
  baseline_config="$(benchmark_config "$baseline")"
  current_config="$(benchmark_config "$output")"
  if [[ "$baseline_config" != "$current_config" ]]; then
    echo "benchmark environment mismatch: baseline=$baseline_config current=$current_config" >&2
    return 1
  fi
  baseline_go="$(awk '/^# go_version:/ { sub(/^# go_version: /, ""); print; exit }' "$baseline")"
  current_go="$($go_cmd version)"
  if [[ -z "$baseline_go" || "$baseline_go" != "$current_go" ]]; then
    echo "Go version mismatch: baseline=${baseline_go:-missing} current=$current_go" >&2
    return 1
  fi
  baseline_governor="$(awk -F': ' '/^# cpu_governor:/ { print $2; exit }' "$baseline")"
  current_governor_value="$(current_governor)"
  if [[ -z "$baseline_governor" || "$baseline_governor" != "$current_governor_value" ]]; then
    echo "CPU governor mismatch: baseline=${baseline_governor:-missing} current=$current_governor_value" >&2
    return 1
  fi
}

evaluate_benchstat_csv() {
  local csv="$1"
  awk -F, -v max_time="$max_time_regression_percent" '
    BEGIN { metric = ""; rows = 0; failed = 0 }
    $2 == "sec/op" || $2 == "B/op" || $2 == "allocs/op" { metric = $2; next }
    metric != "" && $1 != "" && $1 != "geomean" {
      rows++
      change = $6
      baseline_median = $2 + 0
      current_median = $4 + 0
      if (baseline_median <= 0) {
        printf "invalid baseline median: benchmark=%s metric=%s value=%s\n", $1, metric, $2 > "/dev/stderr"
        failed = 1
        next
      }
      median_delta = ((current_median / baseline_median) - 1) * 100
      if (metric == "sec/op" && median_delta > max_time) {
        printf "sec/op median regression: benchmark=%s change=+%.2f%% limit=+%.2f%%\n", $1, median_delta, max_time > "/dev/stderr"
        failed = 1
      }
      if (change == "" || change == "~") next
      gsub(/^\+/, "", change)
      gsub(/%$/, "", change)
      delta = change + 0
      if (metric == "sec/op" && delta > max_time && median_delta <= max_time) {
        printf "sec/op median regression: benchmark=%s change=+%.2f%% limit=+%.2f%%\n", $1, delta, max_time > "/dev/stderr"
        failed = 1
      }
      if ((metric == "B/op" || metric == "allocs/op") && ($4 + 0) > ($2 + 0) && $7 ~ /^p=/) {
        p = $7
        sub(/^p=/, "", p)
        sub(/ .*/, "", p)
        if ((p + 0) < 0.05) {
          printf "significant allocation regression: benchmark=%s metric=%s change=%s p=%s\n", $1, metric, $6, p > "/dev/stderr"
          failed = 1
        }
      }
    }
    END {
      if (rows == 0) {
        print "benchstat comparison produced no comparable rows" > "/dev/stderr"
        exit 1
      }
      exit failed
    }
  ' "$csv"
}

require_tool "$go_cmd"
require_tool timeout

if [[ "$update_baseline" != true && ! -f "$baseline" ]]; then
  echo "baseline not found: $baseline" >&2
  echo 'create it explicitly with --update-baseline on the benchmark runner' >&2
  exit 1
fi
if [[ "$update_baseline" != true ]]; then
  require_tool "$benchstat_cmd"
fi
if [[ "$baseline" == "$output" ]]; then
  echo 'baseline and output paths must differ' >&2
  exit 1
fi

output_dir="$(dirname "$output")"
if [[ ! -d "$output_dir" ]]; then
  install -d -m 0700 "$output_dir"
fi
install -m 0600 /dev/null "$output"

timeout "$benchmark_timeout" "$go_cmd" test ./internal/bench \
  -run '^$' \
  -bench "$benchmark_pattern" \
  -benchmem \
  -count="$sample_count" | tee "$output"
validate_benchmark_output "$output" current

if [[ "$update_baseline" == true ]]; then
  baseline_dir="$(dirname "$baseline")"
  if [[ ! -d "$baseline_dir" ]]; then
    install -d -m 0700 "$baseline_dir"
  fi
  baseline_tmp="$(mktemp "${baseline}.tmp.XXXXXX")"
  write_metadata "$baseline_tmp"
  chmod 0600 "$baseline_tmp"
  mv "$baseline_tmp" "$baseline"
  baseline_tmp=''
  echo "updated baseline: $baseline"
  exit 0
fi

validate_benchmark_output "$baseline" baseline
validate_environment

benchstat_csv="$(mktemp "${output}.benchstat.csv.XXXXXX")"

"$benchstat_cmd" baseline="$baseline" current="$output"
"$benchstat_cmd" -format csv baseline="$baseline" current="$output" >"$benchstat_csv"
evaluate_benchstat_csv "$benchstat_csv"
echo "benchmark gate passed: sec/op median regression <= ${max_time_regression_percent}%; no significant allocation regression"
