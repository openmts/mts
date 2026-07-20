/** 主内容区滚动与返回顶部（纯函数） */

export const DEFAULT_BACK_TO_TOP_THRESHOLD = 320

export function shouldShowBackToTop(
  scrollTop: number,
  threshold = DEFAULT_BACK_TO_TOP_THRESHOLD,
): boolean {
  const top = Number(scrollTop)
  const th = Number(threshold)
  if (!Number.isFinite(top) || !Number.isFinite(th)) return false
  return top >= Math.max(0, th)
}

export type ScrollTopLike = {
  scrollTop: number
  scrollTo?: (options: ScrollToOptions) => void
}

/** 将可滚动元素滚回顶部；无 scrollTo 时回退赋值 scrollTop */
export function scrollElementToTop(
  el: ScrollTopLike | null | undefined,
  behavior: ScrollBehavior = 'smooth',
): boolean {
  if (!el) return false
  try {
    if (typeof el.scrollTo === 'function') {
      el.scrollTo({ top: 0, left: 0, behavior })
      return true
    }
  } catch {
    /* fall through */
  }
  try {
    el.scrollTop = 0
    return true
  } catch {
    return false
  }
}


/** 路径变化时回顶；仅 hash 变化时保留滚动（深链锚点） */
export function shouldResetScrollOnRouteChange(
  fromPath: string,
  toPath: string,
): boolean {
  const a = String(fromPath || '')
  const b = String(toPath || '')
  if (!b) return false
  return a !== b
}
