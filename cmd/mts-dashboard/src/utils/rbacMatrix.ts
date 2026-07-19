/** Dashboard 能力矩阵：与路由 adminOnly / 后端鉴权语义对齐（纯数据，便于单测） */

export type RoleName = 'admin' | 'user'

export type AccessLevel = 'full' | 'self' | 'data_scoped' | 'none'

export interface CapabilityRow {
  id: string
  area: string
  capability: string
  admin: AccessLevel
  user: AccessLevel
  route?: string
  notes?: string
}

export const ACCESS_LEVEL_LABEL: Record<AccessLevel, { zh: string; en: string }> = {
  full: { zh: '全部', en: 'Full' },
  self: { zh: '仅自身', en: 'Self only' },
  data_scoped: { zh: '库级授权', en: 'DB-scoped' },
  none: { zh: '无', en: 'None' },
}

/** 控制台能力矩阵（角色 × 能力） */
export const RBAC_CAPABILITY_MATRIX: CapabilityRow[] = [
  {
    id: 'overview-health',
    area: '概览',
    capability: '基础健康 /readyz /healthz',
    admin: 'full',
    user: 'full',
    route: '/',
  },
  {
    id: 'overview-admin-stats',
    area: '概览',
    capability: '管理统计 /admin/health /maintenance',
    admin: 'full',
    user: 'none',
    route: '/',
  },
  {
    id: 'query',
    area: '数据面',
    capability: '查询 rows/columns/stream/explain',
    admin: 'full',
    user: 'data_scoped',
    route: '/query',
    notes: '非 admin 需对目标库持有 read 权限',
  },
  {
    id: 'write',
    area: '数据面',
    capability: '写入 points/typed/line/prom',
    admin: 'full',
    user: 'data_scoped',
    route: '/write',
    notes: '非 admin 需对目标库持有 write 权限',
  },
  {
    id: 'delete-range',
    area: '数据面',
    capability: '范围删除',
    admin: 'full',
    user: 'data_scoped',
    route: '/query',
    notes: '走 data/delete，受库级 write 约束',
  },
  {
    id: 'users-list',
    area: '用户',
    capability: '用户列表与管理',
    admin: 'full',
    user: 'self',
    route: '/users',
    notes: '普通用户可改自身密码；创建/授权仅 admin',
  },
  {
    id: 'users-grant',
    area: '用户',
    capability: '库级读写授权',
    admin: 'full',
    user: 'none',
    route: '/users',
  },
  {
    id: 'databases',
    area: '管理面',
    capability: '数据库 / RP 管理',
    admin: 'full',
    user: 'none',
    route: '/databases',
  },
  {
    id: 'operations',
    area: '管理面',
    capability: 'Flush / Compact / Retention',
    admin: 'full',
    user: 'none',
    route: '/operations',
  },
  {
    id: 'downsample',
    area: '管理面',
    capability: '降采样策略与动作',
    admin: 'full',
    user: 'none',
    route: '/downsample',
  },
  {
    id: 'storage',
    area: '管理面',
    capability: '快照 / 校验 / 导出',
    admin: 'full',
    user: 'none',
    route: '/storage',
  },
  {
    id: 'config',
    area: '管理面',
    capability: '配置查看 / reload / schema',
    admin: 'full',
    user: 'none',
    route: '/config',
  },
  {
    id: 'audit',
    area: '管理面',
    capability: '审计日志浏览',
    admin: 'full',
    user: 'none',
    route: '/audit',
  },
  {
    id: 'api-spec',
    area: '管理面',
    capability: 'API Spec 浏览',
    admin: 'full',
    user: 'none',
    route: '/api-spec',
  },
  {
    id: 'authz-check',
    area: '鉴权',
    capability: 'authz/database/check 预检',
    admin: 'full',
    user: 'self',
    route: '/query',
    notes: '前端查询/写入前可对当前用户做权限预检',
  },
]

export function levelForRole(row: CapabilityRow, role: RoleName): AccessLevel {
  return role === 'admin' ? row.admin : row.user
}

export function capabilitiesForRole(role: RoleName, rows = RBAC_CAPABILITY_MATRIX): CapabilityRow[] {
  return rows.filter((r) => levelForRole(r, role) !== 'none')
}

export function matrixAreas(rows = RBAC_CAPABILITY_MATRIX): string[] {
  const seen = new Set<string>()
  const out: string[] = []
  for (const r of rows) {
    if (!seen.has(r.area)) {
      seen.add(r.area)
      out.push(r.area)
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
