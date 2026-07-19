#!/usr/bin/env bash
# 校验 mts-backup.sh 结构与语法（不访问真实服务）
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
SCRIPT="${ROOT}/scripts/mts-backup.sh"

[[ -x "$SCRIPT" ]] || { echo "not executable: $SCRIPT" >&2; exit 1; }
bash -n "$SCRIPT"

for needle in \
  'api/v1/admin/storage/data-snapshot' \
  'api/v1/admin/storage/restore-drill' \
  'rsync' \
  '--dry-run' \
  'MTS_BASE_URL' \
  'MTS_ADMIN_TOKEN'
do
  if ! grep -Fq -- "$needle" "$SCRIPT"; then
    echo "missing expected content: $needle" >&2
    exit 1
  fi
done

# dry-run 应成功退出（不访问网络）
MTS_BASE_URL='http://127.0.0.1:9' \
MTS_ADMIN_TOKEN='selfcheck-token' \
  "$SCRIPT" --dry-run --skip-remote >/tmp/mts-backup-selfcheck.out 2>/tmp/mts-backup-selfcheck.err

grep -Fq 'backup orchestration completed' /tmp/mts-backup-selfcheck.err
grep -Fq 'data-snapshot' /tmp/mts-backup-selfcheck.err
grep -Fq 'restore-drill' /tmp/mts-backup-selfcheck.err

echo "mts-backup selfcheck ok"
