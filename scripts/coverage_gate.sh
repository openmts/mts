#!/usr/bin/env bash
set -euo pipefail

coverage_min="${MTS_COVERAGE_MIN:-90.0}"
package_timeout="${MTS_COVERAGE_PACKAGE_TIMEOUT:-300}"
if [[ "$package_timeout" =~ ^[0-9]+([.][0-9]+)?$ ]]; then
  package_timeout="${package_timeout}s"
fi
tmp_dir="$(mktemp -d "${TMPDIR:-/tmp}/mts-coverage.XXXXXX")"

cleanup() {
  rm -rf "$tmp_dir"
}
trap cleanup EXIT

chmod 0700 "$tmp_dir"

packages=("$@")
if [[ "${#packages[@]}" -eq 0 ]]; then
  if ! package_list="$(go list ./... 2>&1)"; then
    printf '%s\n' "$package_list" >&2
    exit 1
  fi
  while IFS= read -r package; do
    case "$package" in
      */tests/* | */internal/bench | */node_modules/*)
        continue
        ;;
    esac
    packages+=("$package")
  done <<< "$package_list"
fi

failed=0
for index in "${!packages[@]}"; do
  package="${packages[$index]}"
  profile="$tmp_dir/package-$index.cover"
  if ! output="$(timeout "$package_timeout" go test "$package" -coverprofile="$profile" -count=1 -timeout 5m 2>&1)"; then
    printf '%s\n' "$output" >&2
    exit 1
  fi
  chmod 0600 "$profile"
  printf '%s\n' "$output"

  if ! awk 'NR > 1 { found = 1 } END { exit !found }' "$profile"; then
    printf 'coverage skipped: package=%s reason=no-statements\n' "$package"
    continue
  fi

  coverage="$(go tool cover -func="$profile" | awk '/^total:/ {gsub("%", "", $3); print $3}')"
  if [[ -z "$coverage" ]]; then
    printf 'coverage unavailable: package=%s\n' "$package" >&2
    exit 1
  fi
  if awk -v got="$coverage" -v min="$coverage_min" 'BEGIN { exit !(got + 0 >= min + 0) }'; then
    printf 'coverage ok: package=%s got=%s%% min=%s%%\n' "$package" "$coverage" "$coverage_min"
    continue
  fi
  printf 'coverage below threshold: package=%s got=%s%% min=%s%%\n' "$package" "$coverage" "$coverage_min" >&2
  failed=1
done

exit "$failed"
