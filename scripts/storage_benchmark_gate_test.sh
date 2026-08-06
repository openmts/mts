#!/usr/bin/env bash
set -euo pipefail
umask 077

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
gate="$repo_root/scripts/storage_benchmark_gate.sh"
tmp_dir="$(mktemp -d)"
trap 'rm -rf "$tmp_dir"' EXIT
chmod 0700 "$tmp_dir"

fake_go="$tmp_dir/fake-go"
cat >"$fake_go" <<'FAKE_GO'
#!/usr/bin/env bash
set -euo pipefail
value="${MTS_FAKE_NS_OP:-100}"
spread="${MTS_FAKE_SPREAD:-0}"
bytes="${MTS_FAKE_B_OP:-64}"
allocs="${MTS_FAKE_ALLOCS_OP:-2}"
if [[ "${1:-}" == 'version' ]]; then
  printf 'go version go1.26.5 linux/amd64\n'
  exit 0
fi
printf 'goos: linux\ngoarch: amd64\npkg: github.com/openmts/mts/internal/bench\ncpu: Fake CPU\n'
benchmarks=(
  BenchmarkEngineWriteBatch
  BenchmarkEngineWriteWideBatch
  BenchmarkEngineQueryRowIterator
  BenchmarkEngineQueryColumnIterator
)
for benchmark in "${benchmarks[@]}"; do
  for sample in $(seq 1 10); do
    printf '%s/points=10000-8 1 %s ns/op %s B/op %s allocs/op\n' \
      "$benchmark" "$((value + spread * sample))" "$bytes" "$allocs"
  done
done
FAKE_GO
chmod 0700 "$fake_go"

fail() {
  printf 'FAIL: %s\n' "$*" >&2
  exit 1
}

expect_failure() {
  local name="$1"
  shift
  if "$@" >"$tmp_dir/$name.log" 2>&1; then
    fail "$name unexpectedly succeeded"
  fi
}

baseline="$tmp_dir/baseline.txt"
output="$tmp_dir/current.txt"

expect_failure missing-baseline env MTS_BENCH_GO="$fake_go" bash "$gate" "$baseline" "$output"
grep -q 'baseline not found' "$tmp_dir/missing-baseline.log" || fail 'missing baseline error not reported'

MTS_BENCH_GO="$fake_go" bash "$gate" --update-baseline "$baseline" "$output" >/dev/null
[[ -f "$baseline" ]] || fail 'update did not create baseline'
[[ "$(stat -c '%a' "$baseline")" == '600' ]] || fail 'baseline permission is not 0600'

expect_failure missing-benchstat env \
  MTS_BENCH_GO="$fake_go" \
  MTS_BENCHSTAT="$tmp_dir/not-installed" \
  bash "$gate" "$baseline" "$output"
grep -q 'required tool not found' "$tmp_dir/missing-benchstat.log" || fail 'missing tool error not reported'

expect_failure time-regression env \
  MTS_BENCH_GO="$fake_go" \
  MTS_FAKE_NS_OP=120 \
  bash "$gate" "$baseline" "$output"
grep -q 'sec/op median regression' "$tmp_dir/time-regression.log" || fail 'time regression not reported'

expect_failure noisy-time-regression env \
  MTS_BENCH_GO="$fake_go" \
  MTS_FAKE_NS_OP=110 \
  MTS_FAKE_SPREAD=2 \
  bash "$gate" "$baseline" "$output"
grep -q 'sec/op median regression' "$tmp_dir/noisy-time-regression.log" || fail 'noisy median regression not reported'

expect_failure allocation-regression env \
  MTS_BENCH_GO="$fake_go" \
  MTS_FAKE_B_OP=80 \
  bash "$gate" "$baseline" "$output"
grep -q 'significant allocation regression' "$tmp_dir/allocation-regression.log" || fail 'allocation regression not reported'

MTS_BENCH_GO="$fake_go" bash "$gate" "$baseline" "$output" >/dev/null

printf 'storage benchmark gate tests passed\n'
