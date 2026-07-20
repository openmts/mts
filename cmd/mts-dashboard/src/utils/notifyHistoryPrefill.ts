/** 通知历史面板筛选深链（query；不含消息正文/token） */

export type NotifyHistoryPrefillKind = 'all' | 'success' | 'error' | 'warn' | 'info'
export type NotifyHistoryPrefillRange = 'all' | '1h' | '24h' | '7d' | '30d'

export type NotifyHistoryPrefill = {
  open?: boolean
  kind?: NotifyHistoryPrefillKind
  q?: string
  range?: NotifyHistoryPrefillRange
}

const KINDS = new Set<NotifyHistoryPrefillKind>(['all', 'success', 'error', 'warn', 'info'])
const RANGES = new Set<NotifyHistoryPrefillRange>(['all', '1h', '24h', '7d', '30d'])

function firstQueryValue(v: unknown): string | undefined {
  if (Array.isArray(v)) {
    const x = v[0]
    return typeof x === 'string' && x.trim() ? x.trim() : undefined
  }
  if (typeof v === 'string' && v.trim()) return v.trim()
  return undefined
}

function truthyFlag(v: unknown): boolean {
  const s = firstQueryValue(v)
  if (!s) return false
  const n = s.toLowerCase()
  return n === '1' || n === 'true' || n === 'yes' || n === 'open'
}

export function isNotifyHistoryKind(v: unknown): v is NotifyHistoryPrefillKind {
  return typeof v === 'string' && KINDS.has(v as NotifyHistoryPrefillKind)
}

export function isNotifyHistoryRange(v: unknown): v is NotifyHistoryPrefillRange {
  return typeof v === 'string' && RANGES.has(v as NotifyHistoryPrefillRange)
}

/** 从 route.query / hash 解析通知历史预填 */
export function parseNotifyHistoryPrefill(
  query: Record<string, unknown> | { [key: string]: unknown },
  hash?: string,
): NotifyHistoryPrefill {
  const out: NotifyHistoryPrefill = {}
  if (truthyFlag(query.notify ?? query.nh_open ?? query.notify_history)) {
    out.open = true
  }
  const h = (hash || '').replace(/^#/, '')
  if (h === 'notify-history' || h === 'notify') out.open = true

  const kind = firstQueryValue(query.nh_kind ?? query.notify_kind)
  if (kind && isNotifyHistoryKind(kind)) out.kind = kind

  const q = firstQueryValue(query.nh_q ?? query.notify_q ?? query.nh_query)
  if (q) out.q = q

  const range = firstQueryValue(query.nh_range ?? query.notify_range)
  if (range && isNotifyHistoryRange(range)) out.range = range

  // 有筛选时也视为希望打开面板（协作深链）
  if (out.kind || out.q || (out.range && out.range !== 'all')) {
    out.open = true
  }
  return out
}

export function buildNotifyHistoryPrefillPath(
  opts: NotifyHistoryPrefill & { path?: string },
): string {
  const base = opts.path && opts.path.startsWith('/') ? opts.path.split('?')[0].split('#')[0] : '/'
  const params = new URLSearchParams()
  // 始终带 open 标记，便于落地即开面板
  params.set('notify', '1')
  if (opts.kind && opts.kind !== 'all') params.set('nh_kind', opts.kind)
  if (opts.q?.trim()) params.set('nh_q', opts.q.trim())
  if (opts.range && opts.range !== 'all') params.set('nh_range', opts.range)
  const qs = params.toString()
  return `${base}?${qs}#notify-history`
}

export function notifyHistoryFormToPrefill(form: {
  kind?: string
  q?: string
  range?: string
}, opts?: { path?: string }): string {
  const kind = isNotifyHistoryKind(form.kind) ? form.kind : 'all'
  const range = isNotifyHistoryRange(form.range) ? form.range : 'all'
  return buildNotifyHistoryPrefillPath({
    open: true,
    kind,
    q: form.q?.trim() || undefined,
    range,
    path: opts?.path,
  })
}
