/** 存储快照列表辅助（纯函数） */

export interface NamedSnapshot {
  name: string
  path?: string
  kind?: string
  size_bytes?: number
  mod_time?: string
}

/** 仅 data-snapshot 可作为 restore-drill 源 */
export function selectableDataSnapshots<T extends NamedSnapshot>(items: T[]): T[] {
  return (items || []).filter((s) => {
    const kind = String(s.kind || '')
    const name = String(s.name || '')
    if (kind === 'data-snapshot') return true
    if (kind === 'restore-drill') return false
    return name.startsWith('data-snapshot-')
  })
}

export function defaultSelectedSnapshotPath(items: NamedSnapshot[], preferred?: string | null): string {
  if (preferred && items.some((s) => s.path === preferred || s.name === preferred)) {
    const hit = items.find((s) => s.path === preferred || s.name === preferred)
    return hit?.path || hit?.name || ''
  }
  const list = selectableDataSnapshots(items)
  return list[0]?.path || list[0]?.name || ''
}

export function snapshotLabel(s: NamedSnapshot): string {
  const size = typeof s.size_bytes === 'number' && s.size_bytes > 0 ? ` (${s.size_bytes}B)` : ''
  return `${s.name}${size}`
}
