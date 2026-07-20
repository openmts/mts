/** Dashboard 能力矩阵：与路由 adminOnly / 后端鉴权语义对齐（纯数据，便于单测） */

export type RoleName = 'admin' | 'user'

export type AccessLevel = 'full' | 'self' | 'data_scoped' | 'none'

export type { LocaleCode, LocalizedText } from './localizedText.ts'
export { textForLocale } from './localizedText.ts'
import type { LocaleCode, LocalizedText } from './localizedText.ts'

export interface CapabilityRow {
  id: string
  /** 稳定区域键，用于筛选 */
  areaKey: string
  area: LocalizedText
  capability: LocalizedText
  admin: AccessLevel
  user: AccessLevel
  route?: string
  notes?: LocalizedText
}

export const ACCESS_LEVEL_LABEL: Record<AccessLevel, LocalizedText> = {
  full: { zh: '全部', en: 'Full' },
  self: { zh: '仅自身', en: 'Self only' },
  data_scoped: { zh: '库级授权', en: 'DB-scoped' },
  none: { zh: '无', en: 'None' },
}

/** 控制台能力矩阵（角色 × 能力） */
export const RBAC_CAPABILITY_MATRIX: CapabilityRow[] = [
  {
    id: 'overview-health',
    areaKey: 'overview',
    area: { zh: '概览', en: 'Overview' },
    capability: { zh: '基础健康 /readyz /healthz', en: 'Basic health /readyz /healthz' },
    admin: 'full',
    user: 'full',
    route: '/',
  },
  {
    id: 'overview-admin-stats',
    areaKey: 'overview',
    area: { zh: '概览', en: 'Overview' },
    capability: {
      zh: '管理统计 /admin/health /maintenance',
      en: 'Admin stats /admin/health /maintenance',
    },
    admin: 'full',
    user: 'none',
    route: '/',
  },
  {
    id: 'query',
    areaKey: 'data',
    area: { zh: '数据面', en: 'Data plane' },
    capability: {
      zh: '查询 rows/columns/stream/explain',
      en: 'Query rows/columns/stream/explain',
    },
    admin: 'full',
    user: 'data_scoped',
    route: '/query',
    notes: {
      zh: '非 admin 需对目标库持有 read 权限',
      en: 'Non-admin needs read grant on target database',
    },
  },
  {
    id: 'write',
    areaKey: 'data',
    area: { zh: '数据面', en: 'Data plane' },
    capability: {
      zh: '写入 points/typed/line/prom',
      en: 'Write points/typed/line/prom',
    },
    admin: 'full',
    user: 'data_scoped',
    route: '/write',
    notes: {
      zh: '非 admin 需对目标库持有 write 权限',
      en: 'Non-admin needs write grant on target database',
    },
  },
  {
    id: 'delete-range',
    areaKey: 'data',
    area: { zh: '数据面', en: 'Data plane' },
    capability: { zh: '范围删除', en: 'Range delete' },
    admin: 'full',
    user: 'data_scoped',
    route: '/query',
    notes: {
      zh: '走 data/delete，受库级 write 约束',
      en: 'Uses data/delete; constrained by DB write grant',
    },
  },
  {
    id: 'users-list',
    areaKey: 'users',
    area: { zh: '用户', en: 'Users' },
    capability: { zh: '用户列表与管理', en: 'User list and management' },
    admin: 'full',
    user: 'self',
    route: '/users',
    notes: {
      zh: '普通用户可改自身密码；创建/授权仅 admin',
      en: 'Users may change own password; create/grants are admin-only',
    },
  },
  {
    id: 'users-grant',
    areaKey: 'users',
    area: { zh: '用户', en: 'Users' },
    capability: { zh: '库级读写授权', en: 'DB read/write grants' },
    admin: 'full',
    user: 'none',
    route: '/users',
  },
  {
    id: 'databases-browse',
    areaKey: 'data',
    area: { zh: '数据面', en: 'Data plane' },
    capability: { zh: '数据库元数据只读浏览', en: 'Read-only database metadata browse' },
    admin: 'full',
    user: 'data_scoped',
    route: '/databases',
    notes: {
      zh: '非 admin 仅见有 read 权限的库；不可创建删除库或新建 RP',
      en: 'Non-admin sees only databases with read grant; create/delete DB and new RP are admin-only',
    },
  },
  {
    id: 'databases',
    areaKey: 'admin',
    area: { zh: '管理面', en: 'Admin plane' },
    capability: { zh: '数据库 / RP 管理', en: 'Database / RP management' },
    admin: 'full',
    user: 'none',
    route: '/databases',
    notes: {
      zh: '创建/删除库与新建 RP 仅 admin',
      en: 'Create/delete databases and new RP are admin-only',
    },
  },
  {
    id: 'operations',
    areaKey: 'admin',
    area: { zh: '管理面', en: 'Admin plane' },
    capability: { zh: 'Flush / Compact / Retention', en: 'Flush / Compact / Retention' },
    admin: 'full',
    user: 'none',
    route: '/operations',
  },
  {
    id: 'downsample',
    areaKey: 'admin',
    area: { zh: '管理面', en: 'Admin plane' },
    capability: { zh: '降采样策略与动作', en: 'Downsample policies and actions' },
    admin: 'full',
    user: 'none',
    route: '/downsample',
  },
  {
    id: 'storage',
    areaKey: 'admin',
    area: { zh: '管理面', en: 'Admin plane' },
    capability: { zh: '快照 / 校验 / 导出', en: 'Snapshot / validate / export' },
    admin: 'full',
    user: 'none',
    route: '/storage',
  },
  {
    id: 'config',
    areaKey: 'admin',
    area: { zh: '管理面', en: 'Admin plane' },
    capability: {
      zh: '配置查看 / reload / schema',
      en: 'Config view / reload / schema',
    },
    admin: 'full',
    user: 'none',
    route: '/config',
  },
  {
    id: 'audit-self',
    areaKey: 'access',
    area: { zh: '访问控制', en: 'Access' },
    capability: { zh: '自身审计事件', en: 'Own audit events' },
    admin: 'full',
    user: 'self',
    route: '/audit',
    notes: {
      zh: '非 admin 仅可读自己的审计记录',
      en: 'Non-admin may only read own audit events',
    },
  },
  {
    id: 'audit',
    areaKey: 'admin',
    area: { zh: '管理面', en: 'Admin plane' },
    capability: { zh: '全站审计日志', en: 'Global audit browse' },
    admin: 'full',
    user: 'none',
    route: '/audit',
    notes: {
      zh: 'admin 可浏览全部用户；非 admin 见 audit-self',
      en: 'Admin global browse; non-admin uses audit-self',
    },
  },
  {
    id: 'api-spec',
    areaKey: 'admin',
    area: { zh: '管理面', en: 'Admin plane' },
    capability: { zh: 'API Spec 浏览', en: 'API Spec browse' },
    admin: 'full',
    user: 'none',
    route: '/api-spec',
  },
  {
    id: 'authz-check',
    areaKey: 'authz',
    area: { zh: '鉴权', en: 'Authz' },
    capability: {
      zh: 'authz/database/check 预检',
      en: 'authz/database/check preflight',
    },
    admin: 'full',
    user: 'self',
    route: '/query',
    notes: {
      zh: '前端查询/写入前可对当前用户做权限预检',
      en: 'Dashboard may preflight current user grants before query/write',
    },
  },
]

export function levelForRole(row: CapabilityRow, role: RoleName): AccessLevel {
  return role === 'admin' ? row.admin : row.user
}

export function capabilitiesForRole(role: RoleName, rows = RBAC_CAPABILITY_MATRIX): CapabilityRow[] {
  return rows.filter((r) => levelForRole(r, role) !== 'none')
}

export interface MatrixAreaOption {
  key: string
  label: LocalizedText
}

export function matrixAreas(rows = RBAC_CAPABILITY_MATRIX): MatrixAreaOption[] {
  const seen = new Set<string>()
  const out: MatrixAreaOption[] = []
  for (const r of rows) {
    if (!seen.has(r.areaKey)) {
      seen.add(r.areaKey)
      out.push({ key: r.areaKey, label: r.area })
    }
  }
  return out
}

export function countByLevel(
  role: RoleName,
  rows = RBAC_CAPABILITY_MATRIX,
): Record<AccessLevel, number> {
  const counts: Record<AccessLevel, number> = { full: 0, self: 0, data_scoped: 0, none: 0 }
  for (const r of rows) {
    counts[levelForRole(r, role)]++
  }
  return counts
}
