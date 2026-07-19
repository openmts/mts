/** 命令面板导航项与过滤（纯函数） */

export interface CommandNavItem {
  id: string
  path: string
  /** i18n key or raw label fallback */
  labelKey: string
  keywords: string[]
  adminOnly?: boolean
}

export const COMMAND_NAV_ITEMS: CommandNavItem[] = [
  { id: 'overview', path: '/', labelKey: 'overview', keywords: ['home', 'dashboard', '概览', '健康'] },
  { id: 'databases', path: '/databases', labelKey: 'databases', keywords: ['db', '数据库', 'rp'], adminOnly: true },
  { id: 'query', path: '/query', labelKey: 'query', keywords: ['select', '查询', 'history'] },
  { id: 'write', path: '/write', labelKey: 'write', keywords: ['insert', '写入', 'line protocol', 'typed'] },
  { id: 'users', path: '/users', labelKey: 'users', keywords: ['user', '用户', '权限'] },
  { id: 'access', path: '/access', labelKey: 'accessMatrix', keywords: ['rbac', '矩阵', 'matrix'] },
  { id: 'grants', path: '/access/grants', labelKey: 'accessGrants', keywords: ['grant', '授权'], adminOnly: true },
  { id: 'metrics', path: '/observability/metrics', labelKey: 'metrics', keywords: ['prometheus', '指标'], adminOnly: true },
  { id: 'config', path: '/config', labelKey: 'config', keywords: ['配置', 'reload', 'token'], adminOnly: true },
  { id: 'operations', path: '/operations', labelKey: 'operations', keywords: ['flush', 'compact', '运维'], adminOnly: true },
  { id: 'downsample', path: '/downsample', labelKey: 'downsample', keywords: ['rollup', '降采样'], adminOnly: true },
  { id: 'audit', path: '/audit', labelKey: 'audit', keywords: ['log', '审计', 'trail'], adminOnly: true },
  { id: 'api-spec', path: '/api-spec', labelKey: 'apiSpec', keywords: ['openapi', '契约', 'spec'], adminOnly: true },
  { id: 'storage', path: '/storage', labelKey: 'storage', keywords: ['snapshot', 'backup', '存储', 'restore'], adminOnly: true },
  { id: 'readiness', path: '/ops/readiness', labelKey: 'readiness', keywords: ['go-live', '就绪', 'doctor'], adminOnly: true },
  { id: 'about', path: '/about', labelKey: 'about', keywords: ['version', '关于', 'build'] },
  { id: 'account', path: '/account', labelKey: 'account', keywords: ['password', '账户', 'profile'] },
]

export function visibleCommandItems(
  items: CommandNavItem[],
  isAdmin: boolean,
): CommandNavItem[] {
  return items.filter((i) => !i.adminOnly || isAdmin)
}

export function filterCommandItems(
  items: CommandNavItem[],
  query: string,
  resolveLabel: (key: string) => string,
): CommandNavItem[] {
  const q = query.trim().toLowerCase()
  if (!q) return items
  return items.filter((item) => {
    const label = resolveLabel(item.labelKey).toLowerCase()
    if (label.includes(q)) return true
    if (item.path.toLowerCase().includes(q)) return true
    if (item.id.toLowerCase().includes(q)) return true
    return item.keywords.some((k) => k.toLowerCase().includes(q))
  })
}

export function matchCommandPaletteOpen(e: KeyboardEvent, editable: boolean): boolean {
  // Ctrl/Cmd+K；在输入框内也允许打开（可商用后台常见）
  if (!(e.metaKey || e.ctrlKey)) return false
  if (e.altKey || e.shiftKey) return false
  return e.key === 'k' || e.key === 'K'
}

export function matchCommandPaletteClose(e: KeyboardEvent): boolean {
  return e.key === 'Escape'
}

export type AuditQuickRange = '1h' | '24h' | '7d' | '30d' | 'clear'

/** 返回 datetime-local 字符串（本地时区） */
export function auditRangeToLocalInputs(
  range: AuditQuickRange,
  now = new Date(),
): { since: string; until: string } {
  if (range === 'clear') return { since: '', until: '' }
  const untilMs = now.getTime()
  const hours =
    range === '1h' ? 1 : range === '24h' ? 24 : range === '7d' ? 24 * 7 : 24 * 30
  const sinceMs = untilMs - hours * 3600_000
  return {
    since: toDatetimeLocalValue(new Date(sinceMs)),
    until: toDatetimeLocalValue(new Date(untilMs)),
  }
}

export function toDatetimeLocalValue(d: Date): string {
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`
}

export function filterAuditEvents<T extends { action?: string; detail?: string; user_name?: string; database?: string }>(
  events: T[],
  clientQuery: string,
): T[] {
  const q = clientQuery.trim().toLowerCase()
  if (!q) return events
  return events.filter((e) => {
    const hay = [e.action, e.detail, e.user_name, e.database]
      .map((x) => String(x ?? '').toLowerCase())
      .join(' ')
    return hay.includes(q)
  })
}
