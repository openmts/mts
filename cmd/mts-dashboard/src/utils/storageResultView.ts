/** Storage 操作结果结构化摘要（data-snapshot / restore-drill） */

export interface DataSnapshotResultView {
  ok: boolean
  path: string
  source?: string
  files: number
  bytes: number
}

export interface RestoreDrillResultView {
  ok: boolean
  path: string
  source: string
  target: string
  files: number
  bytes: number
  check_issues: number
  check_fatals: number
  check_root?: string
  tone: 'ok' | 'warn' | 'bad'
}

export function normalizeDataSnapshotResult(
  raw: {
    ok?: boolean
    path?: string
    source?: string
    files?: number
    bytes?: number
  } | null | undefined,
  fallbackPath = '/api/v1/admin/storage/data-snapshot',
): DataSnapshotResultView | null {
  if (!raw) return null
  return {
    ok: raw.ok !== false,
    path: String(raw.path || fallbackPath),
    source: raw.source ? String(raw.source) : undefined,
    files: Number(raw.files) || 0,
    bytes: Number(raw.bytes) || 0,
  }
}

export function normalizeRestoreDrillResult(
  raw: {
    ok?: boolean
    path?: string
    source?: string
    target?: string
    files?: number
    bytes?: number
    check_issues?: number
    check_fatals?: number
    check_root?: string
  } | null | undefined,
  fallbackPath = '/api/v1/admin/storage/restore-drill',
): RestoreDrillResultView | null {
  if (!raw) return null
  const fatals = Number(raw.check_fatals) || 0
  const issues = Number(raw.check_issues) || 0
  const ok = raw.ok !== false && fatals === 0
  let tone: RestoreDrillResultView['tone'] = 'ok'
  if (!ok || fatals > 0) tone = 'bad'
  else if (issues > 0) tone = 'warn'
  return {
    ok,
    path: String(raw.path || fallbackPath),
    source: String(raw.source || ''),
    target: String(raw.target || ''),
    files: Number(raw.files) || 0,
    bytes: Number(raw.bytes) || 0,
    check_issues: issues,
    check_fatals: fatals,
    check_root: raw.check_root ? String(raw.check_root) : undefined,
    tone,
  }
}

export function formatStorageBytes(bytes: number): string {
  const n = Number(bytes) || 0
  if (n >= 1 << 30) return `${(n / (1 << 30)).toFixed(1)} GB`
  if (n >= 1 << 20) return `${(n / (1 << 20)).toFixed(1)} MB`
  if (n >= 1 << 10) return `${(n / (1 << 10)).toFixed(1)} KB`
  return `${n} B`
}
