/** 命令面板：触发页内 ActionResultBanner 重试按钮 */

function isElementVisible(el: HTMLElement): boolean {
  if (el.hasAttribute('disabled')) return false
  if (el.getAttribute('aria-disabled') === 'true') return false
  if (el.hidden) return false
  if (typeof el.getClientRects === 'function') {
    const rects = el.getClientRects()
    if (rects.length === 0 && el.offsetParent === null && el.style?.display === 'none') {
      return false
    }
  }
  return true
}

/** 选取当前页第一个可用的 action-result-retry 按钮 */
export function pickActionResultRetryButton(
  root: ParentNode | null | undefined = typeof document !== 'undefined' ? document : null,
): HTMLElement | null {
  if (!root) return null
  const nodes = root.querySelectorAll('[data-testid="action-result-retry"]')
  for (let i = 0; i < nodes.length; i++) {
    const el = nodes[i] as HTMLElement
    if (el && typeof el.click === 'function' && isElementVisible(el)) {
      return el
    }
  }
  return null
}

export type TriggerActionRetryResult =
  | { kind: 'clicked' }
  | { kind: 'empty' }

export function resolveActionResultRetryAction(opts: {
  root?: ParentNode | null
}): TriggerActionRetryResult {
  const btn = pickActionResultRetryButton(opts.root)
  if (btn) return { kind: 'clicked' }
  return { kind: 'empty' }
}

export function clickActionResultRetryButton(btn: HTMLElement): void {
  btn.click()
}
