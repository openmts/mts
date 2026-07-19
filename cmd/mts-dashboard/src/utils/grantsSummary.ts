/** 用户库级授权汇总（纯函数，便于单测） */

export interface GrantRow {
  user: string
  role?: string
  disabled?: boolean
  database: string
  permission: string
}

export interface UserGrantBundle {
  user: string
  role?: string
  disabled?: boolean
  grants: Array<{ database: string; permission: string }>
}

export function flattenUserGrants(bundles: UserGrantBundle[]): GrantRow[] {
  const out: GrantRow[] = []
  for (const b of bundles) {
    for (const g of b.grants || []) {
      out.push({
        user: b.user,
        role: b.role,
        disabled: b.disabled,
        database: g.database,
        permission: g.permission,
      })
    }
  }
  return out.sort((a, b) => {
    const u = a.user.localeCompare(b.user)
    if (u !== 0) return u
    const d = a.database.localeCompare(b.database)
    if (d !== 0) return d
    return a.permission.localeCompare(b.permission)
  })
}

export function filterGrantRows(
  rows: GrantRow[],
  opts: { user?: string; database?: string; permission?: string; q?: string } = {},
): GrantRow[] {
  const user = (opts.user || '').trim().toLowerCase()
  const database = (opts.database || '').trim().toLowerCase()
  const permission = (opts.permission || '').trim().toLowerCase()
  const q = (opts.q || '').trim().toLowerCase()
  return rows.filter((r) => {
    if (user && r.user.toLowerCase() !== user) return false
    if (database && r.database.toLowerCase() !== database) return false
    if (permission && r.permission.toLowerCase() !== permission) return false
    if (q) {
      const hay = `${r.user} ${r.role || ''} ${r.database} ${r.permission}`.toLowerCase()
      if (!hay.includes(q)) return false
    }
    return true
  })
}

export function grantCoverage(rows: GrantRow[]): {
  users: number
  databases: number
  grants: number
} {
  return {
    users: new Set(rows.map((r) => r.user)).size,
    databases: new Set(rows.map((r) => r.database)).size,
    grants: rows.length,
  }
}
