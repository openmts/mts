/** 用户授权面板列表筛选（纯函数） */

export function filterDatabaseNames(names: readonly string[], query: string): string[] {
  const q = query.trim().toLowerCase()
  if (!q) return [...names]
  return names.filter((n) => n.toLowerCase().includes(q))
}

export function grantKey(database: string, permission: string): string {
  return `${database}\u0000${permission}`
}

export function sortGrants<T extends { database: string; permission: string }>(items: T[]): T[] {
  return [...items].sort((a, b) => {
    const db = a.database.localeCompare(b.database)
    if (db !== 0) return db
    return a.permission.localeCompare(b.permission)
  })
}
