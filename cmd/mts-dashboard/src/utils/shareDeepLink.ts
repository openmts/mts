/** 命令面板：触发页内「复制筛选/区块深链」按钮，或回退复制当前 URL */

const SENSITIVE_QUERY_RE = /token|password|secret|authorization|api[_-]?key|bearer/i

/** 去掉 URL 中疑似密钥 query（分享/复制安全兜底） */
export function stripSensitiveUrlParams(href: string): string {
  try {
    const u = new URL(href)
    for (const key of [...u.searchParams.keys()]) {
      if (SENSITIVE_QUERY_RE.test(key)) u.searchParams.delete(key)
    }
    // hash 中不解析敏感参数（hash 通常为区块锚点）
    return u.toString()
  } catch {
    return href
  }
}

function isElementVisible(el: HTMLElement): boolean {
  if (el.hasAttribute('disabled')) return false
  if (el.getAttribute('aria-disabled') === 'true') return false
  if (el.hidden) return false
  // 无布局时（jsdom/ssr）视为可见
  if (typeof el.getClientRects === 'function') {
    const rects = el.getClientRects()
    if (rects.length === 0 && el.offsetParent === null && el.style?.display === 'none') {
      return false
    }
  }
  return true
}

/** 选取当前页第一个可用的 * -share-link 按钮 */
export function pickShareLinkButton(
  root: ParentNode | null | undefined = typeof document !== 'undefined' ? document : null,
): HTMLElement | null {
  if (!root) return null
  const nodes = root.querySelectorAll('[data-testid$="-share-link"]')
  for (let i = 0; i < nodes.length; i++) {
    const el = nodes[i] as HTMLElement
    if (el && typeof (el as HTMLElement).click === 'function' && isElementVisible(el)) {
      return el
    }
  }
  return null
}

export type TriggerShareResult =
  | { kind: 'clicked'; testId: string }
  | { kind: 'fallback-url'; href: string }
  | { kind: 'empty' }

/**
 * 优先点击页内 share 按钮（触发与页面按钮一致的深链逻辑）；
 * 若无按钮则返回可复制的清洗后当前 URL。
 */
export function resolveShareDeepLinkAction(opts: {
  root?: ParentNode | null
  href?: string
}): TriggerShareResult {
  const btn = pickShareLinkButton(opts.root)
  if (btn) {
    const testId = btn.getAttribute('data-testid') || 'share-link'
    return { kind: 'clicked', testId }
  }
  const href = stripSensitiveUrlParams(opts.href || '')
  if (!href) return { kind: 'empty' }
  return { kind: 'fallback-url', href }
}

export function clickShareLinkButton(btn: HTMLElement): void {
  btn.click()
}
