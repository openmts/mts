/** 侧栏导航过滤（按 label / path） */

export function filterNavItems<T extends { label: string; to: string }>(
  items: readonly T[],
  query: string,
): T[] {
  const q = String(query || '').trim().toLowerCase()
  if (!q) return items.slice()
  return items.filter((item) => {
    const label = String(item.label || '').toLowerCase()
    const to = String(item.to || '').toLowerCase()
    return label.includes(q) || to.includes(q)
  })
}
