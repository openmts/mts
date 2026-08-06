#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
gate="$repo_root/scripts/coverage_gate.sh"
fixture_dir="$(mktemp -d "${TMPDIR:-/tmp}/mts-coverage-gate-test.XXXXXX")"

cleanup() {
  rm -rf "$fixture_dir"
}
trap cleanup EXIT

if [[ ! -x "$gate" ]]; then
  echo "coverage gate missing or not executable: $gate" >&2
  exit 1
fi

chmod 0700 "$fixture_dir"
install -d -m 0700 \
  "$fixture_dir/empty" \
  "$fixture_dir/covered" \
  "$fixture_dir/uncovered" \
  "$fixture_dir/node_modules/rogue"

printf '%s\n' 'module example.com/coveragefixture' 'go 1.26.0' > "$fixture_dir/go.mod"
printf '%s\n' 'package empty' > "$fixture_dir/empty/doc.go"
printf '%s\n' 'package covered' '' 'func Value() int { return 1 }' > "$fixture_dir/covered/value.go"
printf '%s\n' 'package covered' '' 'import "testing"' '' 'func TestValue(t *testing.T) {' '  if Value() != 1 {' '    t.Fatal("Value() != 1")' '  }' '}' > "$fixture_dir/covered/value_test.go"
printf '%s\n' 'package uncovered' '' 'func Sign(value int) int {' '  if value < 0 {' '    return -1' '  }' '  return 1' '}' > "$fixture_dir/uncovered/sign.go"
printf '%s\n' 'package uncovered' '' 'import "testing"' '' 'func TestSignPositive(t *testing.T) {' '  if Sign(1) != 1 {' '    t.Fatal("Sign(1) != 1")' '  }' '}' > "$fixture_dir/uncovered/sign_test.go"
printf '%s\n' 'package rogue' '' 'func Value() int { return 1 }' > "$fixture_dir/node_modules/rogue/value.go"
chmod 0600 "$fixture_dir/go.mod" "$fixture_dir"/*/*.go
chmod 0600 "$fixture_dir/node_modules/rogue/value.go"

run_gate() {
  local expected_status="$1"
  local expected_text="$2"
  shift 2

  local output
  local status
  set +e
  output="$(
    cd "$fixture_dir"
    MTS_COVERAGE_MIN=90 MTS_COVERAGE_PACKAGE_TIMEOUT="${MTS_TEST_PACKAGE_TIMEOUT:-60}" "$gate" "$@" 2>&1
  )"
  status=$?
  set -e

  if [[ "$status" -ne "$expected_status" ]]; then
    printf 'unexpected exit status: got=%s want=%s\n%s\n' "$status" "$expected_status" "$output" >&2
    exit 1
  fi
  if [[ "$output" != *"$expected_text"* ]]; then
    printf 'missing output %q:\n%s\n' "$expected_text" "$output" >&2
    exit 1
  fi
}

run_gate 0 'coverage skipped: package=./empty reason=no-statements' ./empty
run_gate 0 'coverage ok: package=./covered got=100.0% min=90%' ./covered
run_gate 1 'coverage below threshold: package=./uncovered' ./uncovered
MTS_TEST_PACKAGE_TIMEOUT=60s run_gate 0 'coverage ok: package=./covered got=100.0% min=90%' ./covered

dynamic_output="$(
  cd "$fixture_dir"
  MTS_COVERAGE_MIN=0 MTS_COVERAGE_PACKAGE_TIMEOUT=60 "$gate" 2>&1
)"
if [[ "$dynamic_output" == *node_modules* ]]; then
  printf 'dynamic package discovery included node_modules:\n%s\n' "$dynamic_output" >&2
  exit 1
fi

echo "coverage gate contract tests passed"
