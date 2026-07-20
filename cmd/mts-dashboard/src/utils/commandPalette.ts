/** 命令面板导航项与过滤（纯函数） */

import { buildAuditPrefillPath, buildQueryPrefillPath } from './routePrefill.ts'


export interface CommandNavItem {
  id: string
  path: string
  /** i18n key or raw label fallback */
  labelKey: string
  keywords: string[]
  adminOnly?: boolean
  /** nav 跳转；action 为页内动作 */
  kind?: 'nav' | 'action'
  action?: CommandActionId
}

export type CommandActionId =
  | 'toggle-theme'
  | 'toggle-locale'
  | 'toggle-density'
  | 'focus-sidebar-filter'
  | 'open-notify-history'
  | 'open-shortcuts'
  | 'toggle-sidebar-collapse'
  | 'scroll-main-to-top'
  | 'copy-page-url'
  | 'focus-main'
  | 'reload-page'

/** 页内快捷动作（不离开当前路由，除非动作本身导航） */
export const COMMAND_ACTION_ITEMS: CommandNavItem[] = [
  {
    id: 'action-toggle-theme',
    path: 'action:toggle-theme',
    labelKey: 'cmdActionToggleTheme',
    keywords: ['theme', 'dark', 'light', '主题', '暗色', '亮色'],
    kind: 'action',
    action: 'toggle-theme',
  },
  {
    id: 'action-toggle-locale',
    path: 'action:toggle-locale',
    labelKey: 'cmdActionToggleLocale',
    keywords: ['locale', 'language', 'i18n', '中文', 'english', '语言'],
    kind: 'action',
    action: 'toggle-locale',
  },
  {
    id: 'action-toggle-density',
    path: 'action:toggle-density',
    labelKey: 'cmdActionToggleDensity',
    keywords: ['density', 'compact', 'comfortable', '密度', '紧凑'],
    kind: 'action',
    action: 'toggle-density',
  },
  {
    id: 'action-focus-sidebar-filter',
    path: 'action:focus-sidebar-filter',
    labelKey: 'cmdActionFocusSidebarFilter',
    keywords: ['sidebar filter', 'nav filter', '侧栏过滤', '导航过滤', '/'],
    kind: 'action',
    action: 'focus-sidebar-filter',
  },
  {
    id: 'action-open-notify-history',
    path: 'action:open-notify-history',
    labelKey: 'cmdActionOpenNotifyHistory',
    keywords: ['notify history', 'toasts', '通知历史', '消息历史'],
    kind: 'action',
    action: 'open-notify-history',
  },
  {
    id: 'action-open-shortcuts',
    path: 'action:open-shortcuts',
    labelKey: 'cmdActionOpenShortcuts',
    keywords: ['shortcuts', 'hotkeys', '快捷键', '帮助'],
    kind: 'action',
    action: 'open-shortcuts',
  },
  {
    id: 'action-toggle-sidebar-collapse',
    path: 'action:toggle-sidebar-collapse',
    labelKey: 'cmdActionToggleSidebarCollapse',
    keywords: ['collapse sidebar', 'expand sidebar', '折叠侧栏', '展开侧栏'],
    kind: 'action',
    action: 'toggle-sidebar-collapse',
  },
  {
    id: 'action-scroll-main-to-top',
    path: 'action:scroll-main-to-top',
    labelKey: 'cmdActionScrollMainToTop',
    keywords: ['back to top', 'scroll top', '返回顶部', '回到顶部'],
    kind: 'action',
    action: 'scroll-main-to-top',
  },
  {
    id: 'action-copy-page-url',
    path: 'action:copy-page-url',
    labelKey: 'cmdActionCopyPageUrl',
    keywords: ['copy url', 'copy link', '复制链接', '复制地址', 'url'],
    kind: 'action',
    action: 'copy-page-url',
  },
  {
    id: 'action-focus-main',
    path: 'action:focus-main',
    labelKey: 'cmdActionFocusMain',
    keywords: ['focus main', 'main content', '聚焦主内容', '主区域'],
    kind: 'action',
    action: 'focus-main',
  },
  {
    id: 'action-reload-page',
    path: 'action:reload-page',
    labelKey: 'cmdActionReloadPage',
    keywords: ['reload', 'refresh', '刷新页面', '重新加载'],
    kind: 'action',
    action: 'reload-page',
  },
]

export const COMMAND_NAV_ITEMS: CommandNavItem[] = [
  { id: 'overview', path: '/', labelKey: 'overview', keywords: ['home', 'dashboard', '概览', '健康'] },
  { id: 'databases', path: '/databases', labelKey: 'databases', keywords: ['db', '数据库', 'rp', 'schema', 'measurement'] },
  {
    id: 'databases-filter',
    path: '/databases#databases-filter-bar',
    labelKey: 'cmdDatabasesFilter',
    keywords: ['databases filter', '库筛选', 'db filter'],
  },
  {
    id: 'databases-detail',
    path: '/databases#databases-detail',
    labelKey: 'cmdDatabasesDetail',
    keywords: ['databases detail', '库详情', 'schema detail'],
  },
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
    id: 'query-stats',
    path: '/query#query-stats',
    labelKey: 'cmdQueryStats',
    keywords: ['query stats', '查询统计', 'engine stats', 'query/stats'],
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
  {
    id: 'query-range-1h',
    path: buildQueryPrefillPath({ range: '1h' }),
    labelKey: 'cmdQueryRange1h',
    keywords: ['last 1h', '最近1小时', 'range 1h', 'prefill query', '查询时间预填'],
  },
  {
    id: 'query-range-24h',
    path: buildQueryPrefillPath({ range: '24h' }),
    labelKey: 'cmdQueryRange24h',
    keywords: ['last 24h', '最近24小时', 'range 24h', 'prefill query'],
  },
  {
    id: 'query-range-7d',
    path: buildQueryPrefillPath({ range: '7d' }),
    labelKey: 'cmdQueryRange7d',
    keywords: ['last 7d', '最近7天', 'range 7d', 'prefill query'],
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
  {
    id: 'users-filter',
    path: '/users#users-filter-bar',
    labelKey: 'cmdUsersFilter',
    keywords: ['users filter', '用户筛选', 'role filter'],
    adminOnly: true,
  },
  { id: 'access', path: '/access', labelKey: 'accessMatrix', keywords: ['rbac', '矩阵', 'matrix'] },
  {
    id: 'access-filter',
    path: '/access#access-matrix-filter-bar',
    labelKey: 'cmdAccessFilter',
    keywords: ['access filter', '矩阵筛选', 'rbac filter'],
  },
  { id: 'grants', path: '/access/grants', labelKey: 'accessGrants', keywords: ['grant', '授权'], adminOnly: true },
  {
    id: 'access-grants-filter',
    path: '/access/grants#access-grants-filters',
    labelKey: 'cmdAccessGrantsFilter',
    keywords: ['grants filter', '授权筛选', 'permission filter'],
    adminOnly: true,
  },
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
    id: 'operations-filters',
    path: '/operations#ops-action-filter-bar',
    labelKey: 'cmdOpsFilters',
    keywords: ['ops filters', '运维筛选', 'action filter'],
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
  { id: 'audit', path: '/audit', labelKey: 'audit', keywords: ['log', '审计', 'trail', 'self'] },
  {
    id: 'audit-filters',
    path: '/audit#audit-filters',
    labelKey: 'cmdAuditFilters',
    keywords: ['audit filters', '审计筛选', 'time range'],
  },
  {
    id: 'audit-table',
    path: '/audit#audit-table',
    labelKey: 'cmdAuditTable',
    keywords: ['audit table', '审计表', 'events'],
  },
  {
    id: 'audit-range-1h',
    path: buildAuditPrefillPath({ range: '1h' }),
    labelKey: 'cmdAuditRange1h',
    keywords: ['audit 1h', '审计1小时', 'prefill audit', 'range 1h'],
  },
  {
    id: 'audit-range-24h',
    path: buildAuditPrefillPath({ range: '24h' }),
    labelKey: 'cmdAuditRange24h',
    keywords: ['audit 24h', '审计24小时', 'prefill audit'],
  },
  {
    id: 'audit-action-login',
    path: buildAuditPrefillPath({ range: '24h', action: 'login' }),
    labelKey: 'cmdAuditPrefillLogin',
    keywords: ['login audit', '登录审计', 'action login', 'prefill audit'],
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
  {
    id: 'readiness-export-preflight',
    path: '/ops/readiness#export-preflight',
    labelKey: 'cmdReadinessExportPreflight',
    keywords: ['preflight', 'export preflight', '导出预检', '预检'],
    adminOnly: true,
  },
  {
    id: 'readiness-deploy-drill',
    path: '/ops/readiness#deploy-runbook-drill',
    labelKey: 'cmdReadinessDeployDrill',
    keywords: ['runbook drill', '联调清单', 'deploy drill', '演练清单'],
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

/** 导航 + 页内动作（按角色过滤） */
export function allVisibleCommandItems(isAdmin: boolean): CommandNavItem[] {
  return [
    ...visibleCommandItems(COMMAND_NAV_ITEMS, isAdmin),
    ...visibleCommandItems(COMMAND_ACTION_ITEMS, isAdmin),
  ]
}

export function isCommandAction(item: CommandNavItem): boolean {
  return item.kind === 'action' || Boolean(item.action)
}

export type CommandItemGroupId = 'nav' | 'action'

export interface CommandItemGroup {
  id: CommandItemGroupId
  /** i18n MessageKey */
  labelKey: string
  items: CommandNavItem[]
}

/** 将过滤后的命令列表拆成导航 / 动作两组（保持组内相对顺序） */
export function groupCommandItems(items: readonly CommandNavItem[]): CommandItemGroup[] {
  const nav: CommandNavItem[] = []
  const action: CommandNavItem[] = []
  for (const item of items) {
    if (isCommandAction(item)) action.push(item)
    else nav.push(item)
  }
  const out: CommandItemGroup[] = []
  if (nav.length) {
    out.push({ id: 'nav', labelKey: 'commandPaletteGroupNav', items: nav })
  }
  if (action.length) {
    out.push({ id: 'action', labelKey: 'commandPaletteGroupActions', items: action })
  }
  return out
}

/** 组扁平化，供键盘索引与展示顺序一致 */
export function flattenCommandGroups(groups: readonly CommandItemGroup[]): CommandNavItem[] {
  const out: CommandNavItem[] = []
  for (const g of groups) out.push(...g.items)
  return out
}

/** 导航深链（hash 锚点）；动作项恒为 false */
export function isCommandDeepLink(item: CommandNavItem): boolean {
  if (isCommandAction(item)) return false
  return item.path.includes('#')
}

/**
 * 空查询时折叠导航：默认只保留主路由，隐藏 hash 深链。
 * deepLinkCount 始终统计深链数，便于展开后仍显示「收起」。
 */

export function collapseNavItemsForEmptyQuery(
  items: readonly CommandNavItem[],
  expanded: boolean,
): { items: CommandNavItem[]; hiddenCount: number; deepLinkCount: number } {
  const deepLinkCount = items.reduce((n, i) => n + (isCommandDeepLink(i) ? 1 : 0), 0)
  if (expanded || !deepLinkCount) {
    return { items: items.slice(), hiddenCount: 0, deepLinkCount }
  }
  const primary = items.filter((i) => !isCommandDeepLink(i))
  return { items: primary, hiddenCount: deepLinkCount, deepLinkCount }
}

/** 对分组结果应用空查询导航折叠；非 nav 组不动 */
export function applyEmptyQueryNavCollapse(
  groups: readonly CommandItemGroup[],
  navExpanded: boolean,
): { groups: CommandItemGroup[]; navHiddenCount: number; navDeepLinkCount: number } {
  let navHiddenCount = 0
  let navDeepLinkCount = 0
  const out: CommandItemGroup[] = []
  for (const g of groups) {
    if (g.id !== 'nav') {
      out.push({ ...g, items: g.items.slice() })
      continue
    }
    const collapsed = collapseNavItemsForEmptyQuery(g.items, navExpanded)
    navHiddenCount = collapsed.hiddenCount
    navDeepLinkCount = collapsed.deepLinkCount
    out.push({ ...g, items: collapsed.items })
  }
  return { groups: out, navHiddenCount, navDeepLinkCount }
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

export type CommandListKey =
  | 'next'
  | 'prev'
  | 'home'
  | 'end'
  | 'none'

/** 解析列表导航键（不处理打开/关闭） */
export function commandListKeyFromEvent(e: KeyboardEvent): CommandListKey {
  if (e.key === 'ArrowDown') return 'next'
  if (e.key === 'ArrowUp') return 'prev'
  if (e.key === 'Home') return 'home'
  if (e.key === 'End') return 'end'
  return 'none'
}

/** 列表 activeIndex 移动；len<=0 时返回 0 */
export function moveCommandActiveIndex(
  current: number,
  len: number,
  key: CommandListKey,
): number {
  if (len <= 0) return 0
  const cur = Number.isFinite(current) ? Math.trunc(current) : 0
  const clamped = ((cur % len) + len) % len
  switch (key) {
    case 'next':
      return (clamped + 1) % len
    case 'prev':
      return (clamped - 1 + len) % len
    case 'home':
      return 0
    case 'end':
      return len - 1
    default:
      return clamped
  }
}

/** id -> flat index，O(1) 查选中态 */
export function commandItemIndexMap(
  items: readonly { id: string }[],
): Map<string, number> {
  const m = new Map<string, number>()
  items.forEach((it, i) => {
    if (!m.has(it.id)) m.set(it.id, i)
  })
  return m
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
