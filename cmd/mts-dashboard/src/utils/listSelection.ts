/** 列表多选纯函数（按主键字符串） */

export function toggleSelection(
  selected: readonly string[],
  id: string,
  checked: boolean,
): string[] {
  const key = String(id || '')
  if (!key) return selected.slice()
  const set = new Set(selected)
  if (checked) set.add(key)
  else set.delete(key)
  return [...set]
}

/** 全选/取消全选：仅影响 visibleIds 集合，保留其他已选 */
export function toggleSelectAll(
  selected: readonly string[],
  visibleIds: readonly string[],
  checked: boolean,
): string[] {
  if (!checked) {
    const hide = new Set(visibleIds.map(String))
    return selected.filter((id) => !hide.has(id))
  }
  const set = new Set(selected)
  for (const id of visibleIds) {
    const k = String(id || '')
    if (k) set.add(k)
  }
  return [...set]
}

export function clearSelection(): string[] {
  return []
}

export function isAllSelected(
  selected: readonly string[],
  visibleIds: readonly string[],
): boolean {
  if (!visibleIds.length) return false
  const set = new Set(selected)
  return visibleIds.every((id) => set.has(String(id)))
}

export function isSomeSelected(
  selected: readonly string[],
  visibleIds: readonly string[],
): boolean {
  if (!visibleIds.length || !selected.length) return false
  const set = new Set(selected)
  const hits = visibleIds.filter((id) => set.has(String(id))).length
  return hits > 0 && hits < visibleIds.length
}

/** 有选择则取交集后的有序可见子集；无选择返回全部 visible */
export function resolveExportIds(
  selected: readonly string[],
  visibleIds: readonly string[],
): string[] {
  if (!selected.length) return visibleIds.map(String)
  const set = new Set(selected)
  return visibleIds.map(String).filter((id) => set.has(id))
}

export function filterRowsByIds<T>(
  rows: readonly T[],
  ids: readonly string[],
  idOf: (row: T) => string,
): T[] {
  const set = new Set(ids.map(String))
  return rows.filter((r) => set.has(String(idOf(r))))
}
