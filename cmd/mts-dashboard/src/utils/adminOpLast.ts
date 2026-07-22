/** ops-status last admin heavy result helpers（纯函数） */

export interface AdminHeavyLast {
  op: string
  ok: boolean
  error: string
  startedAtUnix: number | null
  finishedAtUnix: number | null
  durationMs: number | null
}

export function parseAdminHeavyLast(raw: unknown): AdminHeavyLast | null {
  if (raw == null || typeof raw !== 'object') return null
  const o = raw as Record<string, unknown>
  const op = typeof o.op === 'string' ? o.op.trim() : ''
  if (!op && o.ok == null && o.error == null && o.finished_at_unix == null) return null
  const num = (v: unknown): number | null =>
    typeof v === 'number' && Number.isFinite(v) && v > 0 ? Math.floor(v) : null
  const err = typeof o.error === 'string' ? o.error.trim() : ''
  return {
    op,
    ok: Boolean(o.ok),
    error: err,
    startedAtUnix: num(o.started_at_unix),
    finishedAtUnix: num(o.finished_at_unix),
    durationMs:
      typeof o.duration_ms === 'number' && Number.isFinite(o.duration_ms) && o.duration_ms >= 0
        ? Math.floor(o.duration_ms)
        : null,
  }
}

/** 运维条「最近一次」摘要文案（纯函数） */

function formatLastDuration(durationMs: number | null | undefined): string {
  if (durationMs == null || !Number.isFinite(durationMs) || durationMs < 0) return ''
  if (durationMs < 1000) return `${Math.max(1, Math.round(durationMs))}ms`
  const sec = durationMs / 1000
  if (sec < 60) return `${sec >= 10 ? Math.round(sec) : Math.round(sec * 10) / 10}s`
  const m = Math.floor(sec / 60)
  const s = Math.round(sec % 60)
  return `${m}m${String(s).padStart(2, '0')}s`
}

export function formatAdminHeavyLastSummary(
  last: AdminHeavyLast | null | undefined,
  opLabel: string,
): string {
  if (!last || !last.op) return ''
  const label = (opLabel || last.op).trim() || last.op
  const dur = formatLastDuration(last.durationMs)
  if (last.ok) {
    return dur ? `${label} · ok · ${dur}` : `${label} · ok`
  }
  const err = (last.error || '').trim()
  const base = dur ? `${label} · fail · ${dur}` : `${label} · fail`
  return err ? `${base}: ${err}` : base
}
