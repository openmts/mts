/** 路由 hash 锚点定位（纯函数 + DOM 滚动分离，便于单测） */

export function hashTargetId(hash: string | null | undefined): string {
  const raw = String(hash ?? '').trim()
  if (!raw) return ''
  const body = raw.startsWith('#') ? raw.slice(1) : raw
  if (!body) return ''
  try {
    return decodeURIComponent(body)
  } catch {
    return body
  }
}

export type ScrollIntoViewLike = {
  scrollIntoView: (arg?: ScrollIntoViewOptions | boolean) => void
}

export type HashScrollRoot = {
  getElementById: (id: string) => ScrollIntoViewLike | null
}

/** 按 hash 滚动到目标元素；无 DOM / 无目标时返回 false */
export function scrollToHashTarget(
  hash: string | null | undefined,
  root: HashScrollRoot | null | undefined,
  opts: ScrollIntoViewOptions = { behavior: 'smooth', block: 'start' },
): boolean {
  const id = hashTargetId(hash)
  if (!id || root == null) return false
  const el = root.getElementById(id)
  if (!el || typeof el.scrollIntoView !== 'function') return false
  el.scrollIntoView(opts)
  return true
}

/** rAF 后再滚，避免路由切换后 DOM 未挂载 */
export function scheduleScrollToHash(
  hash: string | null | undefined,
  root: HashScrollRoot | null | undefined = typeof document !== 'undefined' ? document : null,
  schedule: (cb: () => void) => void = (cb) => {
    if (typeof requestAnimationFrame === 'function') requestAnimationFrame(() => cb())
    else cb()
  },
): void {
  schedule(() => {
    void scrollToHashTarget(hash, root)
  })
}
