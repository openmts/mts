/** 计算虚拟列表可见区间 */
export function visibleRange(
  scrollTop: number,
  viewportHeight: number,
  rowHeight: number,
  total: number,
  overscan = 8,
): { start: number; end: number } {
  if (total <= 0 || rowHeight <= 0) return { start: 0, end: 0 }
  const start = Math.max(0, Math.floor(scrollTop / rowHeight) - overscan)
  const count = Math.ceil(viewportHeight / rowHeight) + overscan * 2
  const end = Math.min(total, start + count)
  return { start, end }
}
