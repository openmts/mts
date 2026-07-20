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
  {
    id: 'query-form',
    path: '/query#query-form',
    labelKey: 'cmdQueryForm',
    keywords: ['query form', '查询表单', 'builder'],
  },
  {
    id: 'query-history',
    path: '/query#query-history',
    labelKey: 'cmdQueryHistory',
    keywords: ['query history', '查询历史', 'history'],
  },
  {
    id: 'query-results',
    path: '/query#query-results',
    labelKey: 'cmdQueryResults',
    keywords: ['query results', '查询结果', 'rows'],
  },
  {
    id: 'query-chart',
    path: '/query#query-chart',
    labelKey: 'cmdQueryChart',
    keywords: ['query chart', '查询图表', 'chart'],
  },
  { id: 'write', path: '/write', labelKey: 'write', keywords: ['insert', '写入', 'line protocol', 'typed'] },
  {
    id: 'write-mode-typed',
    path: '/write#write-mode-typed',
    labelKey: 'cmdWriteTyped',
    keywords: ['typed batch', '列式写入', 'WriteTypedBatch', 'typed'],
  },
  {
    id: 'write-mode-line',
    path: '/write#write-mode-line',
    labelKey: 'cmdWriteLine',
    keywords: ['line protocol', '行协议', 'lp'],
  },
  {
    id: 'write-mode-form',
    path: '/write#write-mode-form',
    labelKey: 'cmdWriteForm',
    keywords: ['form write', '表单写入'],
  },
  {
    id: 'write-mode-prometheus',
    path: '/write#write-mode-prometheus',
    labelKey: 'cmdWritePrometheus',
    keywords: ['prometheus', 'prom write', 'remote write text'],
  },
  {
    id: 'write-actions',
    path: '/write#write-actions',
    labelKey: 'cmdWriteActions',
    keywords: ['write submit', '提交写入', 'export draft'],
  },
  { id: 'users', path: '/users', labelKey: 'users', keywords: ['user', '用户', '权限'] },
  { id: 'access', path: '/access', labelKey: 'accessMatrix', keywords: ['rbac', '矩阵', 'matrix'] },
  { id: 'grants', path: '/access/grants', labelKey: 'accessGrants', keywords: ['grant', '授权'], adminOnly: true },
  { id: 'metrics', path: '/observability/metrics', labelKey: 'metrics', keywords: ['prometheus', '指标'], adminOnly: true },
  {
    id: 'metrics-summary',
    path: '/observability/metrics#metrics-summary',
    labelKey: 'cmdMetricsSummary',
    keywords: ['metrics summary', '指标汇总', 'families'],
    adminOnly: true,
  },
  {
    id: 'metrics-list',
    path: '/observability/metrics#metrics-list',
    labelKey: 'cmdMetricsList',
    keywords: ['metrics list', '指标列表', 'prometheus list'],
    adminOnly: true,
  },
  { id: 'config', path: '/config', labelKey: 'config', keywords: ['配置', 'reload', 'token'], adminOnly: true },
  {
    id: 'config-effective',
    path: '/config#config-effective',
    labelKey: 'cmdConfigEffective',
    keywords: ['effective config', '生效配置', 'runtime config'],
    adminOnly: true,
  },
  {
    id: 'config-schema',
    path: '/config#config-schema',
    labelKey: 'cmdConfigSchema',
    keywords: ['config schema', '配置 schema', '字段说明'],
    adminOnly: true,
  },
  {
    id: 'config-error-codes',
    path: '/config#config-error-codes',
    labelKey: 'cmdConfigErrorCodes',
    keywords: ['error codes', '错误码', 'error code'],
    adminOnly: true,
  },
  { id: 'operations', path: '/operations', labelKey: 'operations', keywords: ['flush', 'compact', '运维'], adminOnly: true },
  {
    id: 'operations-flush',
    path: '/operations#ops-flush',
    labelKey: 'cmdOpsFlush',
    keywords: ['flush', 'memtable', '刷盘', '刷新'],
    adminOnly: true,
  },
  {
    id: 'operations-compact',
    path: '/operations#ops-compact',
    labelKey: 'cmdOpsCompact',
    keywords: ['compact', '合并', 'compaction'],
    adminOnly: true,
  },
  {
    id: 'operations-retention',
    path: '/operations#ops-retention',
    labelKey: 'cmdOpsRetention',
    keywords: ['retention', 'TTL', '过期', '清理'],
    adminOnly: true,
  },
  {
    id: 'operations-action-log',
    path: '/operations#ops-action-log',
    labelKey: 'cmdOpsActionLog',
    keywords: ['action log', '运维日志', 'ops log'],
    adminOnly: true,
  },
  {
    id: 'operations-maint-errors',
    path: '/operations#ops-maint-errors',
    labelKey: 'cmdOpsMaintErrors',
    keywords: ['maintenance errors', '维护错误', 'maint errors'],
    adminOnly: true,
  },
  { id: 'downsample', path: '/downsample', labelKey: 'downsample', keywords: ['rollup', '降采样'], adminOnly: true },
  {
    id: 'downsample-filters',
    path: '/downsample#downsample-filters',
    labelKey: 'cmdDownsampleFilters',
    keywords: ['downsample filter', '策略过滤', 'policy filter'],
    adminOnly: true,
  },
  {
    id: 'downsample-status',
    path: '/downsample#downsample-status',
    labelKey: 'cmdDownsampleStatus',
    keywords: ['downsample status', '降采样状态', 'watermark'],
    adminOnly: true,
  },
  { id: 'audit', path: '/audit', labelKey: 'audit', keywords: ['log', '审计', 'trail'], adminOnly: true },
  {
    id: 'audit-filters',
    path: '/audit#audit-filters',
    labelKey: 'cmdAuditFilters',
    keywords: ['audit filters', '审计筛选', 'time range'],
    adminOnly: true,
  },
  {
    id: 'audit-table',
    path: '/audit#audit-table',
    labelKey: 'cmdAuditTable',
    keywords: ['audit table', '审计表', 'events'],
    adminOnly: true,
  },
  { id: 'api-spec', path: '/api-spec', labelKey: 'apiSpec', keywords: ['openapi', '契约', 'spec'], adminOnly: true },
  { id: 'storage', path: '/storage', labelKey: 'storage', keywords: ['snapshot', 'backup', '存储', 'restore'], adminOnly: true },
  {
    id: 'storage-data-restore',
    path: '/storage#data-restore',
    labelKey: 'cmdStorageDataRestore',
    keywords: ['data_dir', 'restore-drill', '旁路恢复', 'snapshot'],
    adminOnly: true,
  },
  {
    id: 'storage-backup-drill',
    path: '/storage#backup-drill',
    labelKey: 'cmdStorageBackupDrill',
    keywords: ['backup drill', '备份演练', 'checklist'],
    adminOnly: true,
  },
  {
    id: 'storage-edge-https',
    path: '/storage#edge-https',
    labelKey: 'cmdStorageEdgeHttps',
    keywords: ['hsts', 'tls', 'edge', '边缘', 'https'],
    adminOnly: true,
  },
  { id: 'readiness', path: '/ops/readiness', labelKey: 'readiness', keywords: ['go-live', '就绪', 'doctor'], adminOnly: true },
  {
    id: 'readiness-deploy-kit',
    path: '/ops/readiness#deploy-kit',
    labelKey: 'cmdReadinessDeployKit',
    keywords: ['deploy kit', '部署材料', 'nginx', 'cron', 'systemd'],
    adminOnly: true,
  },
  {
    id: 'readiness-signoff',
    path: '/ops/readiness#signoff-notes',
    labelKey: 'cmdReadinessSignoff',
    keywords: ['signoff', '签核', 'evidence', '备注', 'offsite', 'alert'],
    adminOnly: true,
  },
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

export interface RecentCommandSource {
  path: string
  name?: string
  pinned?: boolean
  at?: number
}

export interface RecentCommandItem {
  id: string
  path: string
  name: string
  pinned: boolean
}

/** 命令面板空查询时展示的最近访问（固定优先，最多 max） */
export function recentCommandItems(
  recent: readonly RecentCommandSource[],
  max = 5,
): RecentCommandItem[] {
  const sorted = [...recent].sort((a, b) => {
    const ap = a.pinned ? 1 : 0
    const bp = b.pinned ? 1 : 0
    if (ap !== bp) return bp - ap
    return (b.at ?? 0) - (a.at ?? 0)
  })
  const out: RecentCommandItem[] = []
  const seen = new Set<string>()
  for (const r of sorted) {
    const path = String(r.path || '').trim()
    if (!path || path.startsWith('/login')) continue
    if (seen.has(path)) continue
    seen.add(path)
    out.push({
      id: `recent-${path}`,
      path,
      name: r.name ? String(r.name) : '',
      pinned: !!r.pinned,
    })
    if (out.length >= max) break
  }
  return out
}
