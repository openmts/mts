#!/usr/bin/env bash
# mts-backup.sh — 可商用 data_dir 备份编排样例
# 依赖: curl, python3；异地拷贝可选 rsync
set -euo pipefail

usage() {
  cat <<'USAGE'
用法: mts-backup.sh [选项]

环境变量（也可用 flag 覆盖）:
  MTS_BASE_URL      服务根地址，如 http://127.0.0.1:8080
  MTS_ADMIN_TOKEN   管理员 Bearer Token 或 admin_token
  MTS_BACKUP_REMOTE rsync 目标，如 user@host:/backups/mts （可选）
  MTS_BACKUP_KEEP   本地保留 data-snapshot 天数，默认 7
  MTS_BACKUP_DIR    本地备份目录（仅清理用；默认空=跳过本地清理）

选项:
  --base-url URL
  --token TOKEN
  --remote DEST
  --keep DAYS
  --backup-dir DIR
  --skip-flush          快照时不请求 flush
  --skip-remote         跳过 rsync
  --skip-restore-drill  跳过 restore-drill
  --dry-run             只打印将执行的动作
  -h, --help
USAGE
}

BASE_URL="${MTS_BASE_URL:-}"
TOKEN="${MTS_ADMIN_TOKEN:-}"
REMOTE="${MTS_BACKUP_REMOTE:-}"
KEEP_DAYS="${MTS_BACKUP_KEEP:-7}"
BACKUP_DIR="${MTS_BACKUP_DIR:-}"
FLUSH=1
DO_REMOTE=1
DO_DRILL=1
DRY_RUN=0

log() { printf '[mts-backup] %s\n' "$*" >&2; }
die() { printf '[mts-backup] ERROR: %s\n' "$*" >&2; exit 1; }

while [[ $# -gt 0 ]]; do
  case "$1" in
    --base-url) BASE_URL="${2:-}"; shift 2 ;;
    --token) TOKEN="${2:-}"; shift 2 ;;
    --remote) REMOTE="${2:-}"; shift 2 ;;
    --keep) KEEP_DAYS="${2:-}"; shift 2 ;;
    --backup-dir) BACKUP_DIR="${2:-}"; shift 2 ;;
    --skip-flush) FLUSH=0; shift ;;
    --skip-remote) DO_REMOTE=0; shift ;;
    --skip-restore-drill) DO_DRILL=0; shift ;;
    --dry-run) DRY_RUN=1; shift ;;
    -h|--help) usage; exit 0 ;;
    *) die "unknown arg: $1" ;;
  esac
done

[[ -n "$BASE_URL" ]] || die "MTS_BASE_URL / --base-url required"
[[ -n "$TOKEN" ]] || die "MTS_ADMIN_TOKEN / --token required"
command -v curl >/dev/null 2>&1 || die "curl is required"
command -v python3 >/dev/null 2>&1 || die "python3 is required"

BASE_URL="${BASE_URL%/}"
AUTH_HEADER="Authorization: Bearer ${TOKEN}"
TMP_DIR="$(mktemp -d "${TMPDIR:-/tmp}/mts-backup.XXXXXX")"
chmod 0700 "$TMP_DIR"
cleanup() { rm -rf "$TMP_DIR"; }
trap cleanup EXIT

api_post() {
  local path="$1"
  # 注意：不能用 ${2:-{}}，bash 会把参数里的 } 提前结束默认值展开
  local body="{}"
  if [[ $# -ge 2 ]]; then
    body="$2"
  fi
  local out="$TMP_DIR/resp.json"
  local code
  if [[ "$DRY_RUN" -eq 1 ]]; then
    log "DRY-RUN POST ${BASE_URL}${path} body=${body}"
    if [[ "$path" == *restore-drill* ]]; then
      printf '%s\n' '{"ok":true,"source":"/dry-run/data-snapshot","target":"/dry-run/restore-drill","check_fatals":0,"files":1,"bytes":1}' >"$out"
    else
      printf '%s\n' '{"ok":true,"path":"/dry-run/data-snapshot","source":"/dry-run","files":1,"bytes":1}' >"$out"
    fi
    cat "$out"
    return 0
  fi
  code="$(curl -sS -o "$out" -w '%{http_code}' \
    -X POST \
    -H "$AUTH_HEADER" \
    -H 'Content-Type: application/json' \
    -d "$body" \
    "${BASE_URL}${path}")"
  if [[ "$code" != "200" ]]; then
    die "POST ${path} http=${code} body=$(cat "$out" 2>/dev/null || true)"
  fi
  cat "$out"
}

json_get() {
  local file="$1"
  local key="$2"
  python3 - "$file" "$key" <<'PY'
import json, sys
path, key = sys.argv[1], sys.argv[2]
with open(path, "r", encoding="utf-8") as f:
    data = json.load(f)
cur = data
for part in key.split("."):
    if not isinstance(cur, dict) or part not in cur:
        print("")
        raise SystemExit(0)
    cur = cur[part]
if cur is None:
    print("")
elif isinstance(cur, bool):
    print("true" if cur else "false")
else:
    print(cur)
PY
}

# 1) data-snapshot
flush_json="true"
[[ "$FLUSH" -eq 1 ]] || flush_json="false"
log "creating data-snapshot (flush=${flush_json})"
body="$(python3 -c 'import json,sys; print(json.dumps({"flush": sys.argv[1]=="true"}))' "${flush_json}")"
snap_json="$(api_post "/api/v1/admin/storage/data-snapshot" "$body")"
printf '%s\n' "$snap_json" >"$TMP_DIR/snapshot.json"
snap_path="$(json_get "$TMP_DIR/snapshot.json" path)"
snap_ok="$(json_get "$TMP_DIR/snapshot.json" ok)"
[[ "$snap_ok" == "True" || "$snap_ok" == "true" || "$snap_ok" == "1" ]] || die "data-snapshot ok=false: $snap_json"
[[ -n "$snap_path" ]] || die "data-snapshot missing path: $snap_json"
log "data-snapshot path=${snap_path}"

# 2) optional remote rsync
if [[ "$DO_REMOTE" -eq 1 && -n "$REMOTE" ]]; then
  command -v rsync >/dev/null 2>&1 || die "rsync is required when MTS_BACKUP_REMOTE is set"
  dest="${REMOTE%/}/$(basename "$snap_path")"
  log "rsync ${snap_path}/ -> ${dest}/"
  if [[ "$DRY_RUN" -eq 1 ]]; then
    log "DRY-RUN rsync -a --delete ${snap_path}/ ${dest}/"
  else
    rsync -a --delete "${snap_path}/" "${dest}/"
  fi
elif [[ "$DO_REMOTE" -eq 1 ]]; then
  log "skip remote: MTS_BACKUP_REMOTE empty"
fi

# 3) optional restore-drill
if [[ "$DO_DRILL" -eq 1 ]]; then
  log "running restore-drill for source=${snap_path}"
  # JSON 编码 source_path
  body="$(python3 -c 'import json,sys; print(json.dumps({"source_path": sys.argv[1]}))' "$snap_path")"
  drill_json="$(api_post "/api/v1/admin/storage/restore-drill" "$body")"
  printf '%s\n' "$drill_json" >"$TMP_DIR/drill.json"
  drill_ok="$(json_get "$TMP_DIR/drill.json" ok)"
  fatals="$(json_get "$TMP_DIR/drill.json" check_fatals)"
  target="$(json_get "$TMP_DIR/drill.json" target)"
  if [[ "$drill_ok" != "True" && "$drill_ok" != "true" && "$drill_ok" != "1" ]]; then
    die "restore-drill ok=false: $drill_json"
  fi
  if [[ -n "$fatals" && "$fatals" != "0" ]]; then
    die "restore-drill check_fatals=${fatals}: $drill_json"
  fi
  log "restore-drill target=${target} fatals=${fatals:-0}"
else
  log "skip restore-drill"
fi

# 4) local retention cleanup
if [[ -n "$BACKUP_DIR" ]]; then
  log "cleanup data-snapshot older than ${KEEP_DAYS}d under ${BACKUP_DIR}"
  if [[ "$DRY_RUN" -eq 1 ]]; then
    log "DRY-RUN find ${BACKUP_DIR} -maxdepth 1 -name 'data-snapshot-*' -mtime +${KEEP_DAYS}"
  else
    find "$BACKUP_DIR" -maxdepth 1 -type d -name 'data-snapshot-*' -mtime "+${KEEP_DAYS}" -print -exec rm -rf {} + 2>/dev/null || true
  fi
fi

log "backup orchestration completed"
