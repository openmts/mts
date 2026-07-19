#!/usr/bin/env bash
set -euo pipefail

project_name="${PROJECT_NAME:-github.com/openmts/mts}"
coverage_min="${MTS_COVERAGE_MIN:-90.0}"
tmp_dir="${TMPDIR:-/tmp}/mts-ci-gate-$$"
export GOSUMDB="${MTS_GOSUMDB:-sum.golang.org}"
go_cmd=(go)

cleanup() {
  rm -rf "$tmp_dir"
}
trap cleanup EXIT

mkdir -p "$tmp_dir"
chmod 0700 "$tmp_dir"

echo "== format =="
timeout 300s goimports-reviser -project-name "$project_name" -recursive -format -rm-unused .

echo "== tests =="
timeout 600s "${go_cmd[@]}" test ./... -count=1 -timeout 10m

echo "== lint =="
timeout 720s golangci-lint run ./...

echo "== backup script selfcheck =="
timeout 60s bash scripts/mts-backup-selfcheck.sh

echo "== race (core storage) =="
timeout 900s "${go_cmd[@]}" test -race -count=1 -timeout 15m \
  ./internal/engine ./internal/memtable ./internal/sstable ./internal/wal ./internal/catalog ./internal/runtime

echo "== coverage =="
core_packages=(
  .
  ./cmd/mts-server
  ./cmd/mts-storage
  ./internal/catalog
  ./internal/codec
  ./internal/engine
  ./internal/faultinject
  ./internal/memtable
  ./internal/model
  ./internal/observability
  ./internal/queryanalyzer
  ./internal/queryexec
  ./internal/querylang
  ./internal/queryoptimizer
  ./internal/queryphysical
  ./internal/queryplanner
  ./internal/queryservice
  ./internal/service
  ./internal/sstable
  ./internal/storagecheck
  ./internal/storagefs
  ./internal/wal
)

failed=0
for pkg in "${core_packages[@]}"; do
  profile="$tmp_dir/$(echo "$pkg" | tr '/.' '__').cover"
  output="$(timeout 300s "${go_cmd[@]}" test "$pkg" -coverprofile="$profile" -count=1 -timeout 5m)"
  echo "$output"
  coverage="$("${go_cmd[@]}" tool cover -func="$profile" | awk '/^total:/ {gsub("%", "", $3); print $3}')"
  awk -v got="$coverage" -v min="$coverage_min" 'BEGIN { exit !(got + 0 >= min + 0) }' || {
    echo "coverage below threshold: package=$pkg got=${coverage}% min=${coverage_min}%"
    failed=1
  }
done
if [[ "$failed" -ne 0 ]]; then
  exit 1
fi

echo "== regression smoke =="
timeout 900s "${go_cmd[@]}" test ./tests/fault/storage_fault_matrix ./tests/scale/storage_matrix ./tests/pprof/storage_engine -count=1 -timeout 15m

echo "== artifact scan =="
artifacts="$(timeout 60s find . -type f \( -name '*.test' -o -name '*.prof' -o -name '*.pprof' -o -name 'coverage.out' -o -name '*.coverprofile' -o -name 'cpu.out' -o -name 'mem.out' \) -not -path './.git/*' -print)"
if [[ -n "$artifacts" ]]; then
  echo "$artifacts"
  exit 1
fi
if [[ -f ./mts-server || -f ./mts-storage ]]; then
  echo "unexpected root binaries present"
  ls -la ./mts-server ./mts-storage 2>/dev/null || true
  exit 1
fi
