const FOCUSABLE_SELECTOR = [
  'a[href]',
  'button:not([disabled])',
  'textarea:not([disabled])',
  'input:not([disabled])',
  'select:not([disabled])',
  '[tabindex]:not([tabindex="-1"])',
].join(',')

export function getFocusableElements(root: HTMLElement): HTMLElement[] {
  return Array.from(root.querySelectorAll<HTMLElement>(FOCUSABLE_SELECTOR)).filter((el) => {
    if (el.hasAttribute('disabled')) return false
    if (el.getAttribute('aria-hidden') === 'true') return false
    // 隐藏元素（display:none / 不在布局中）
    if (el.offsetParent === null && el !== document.activeElement) {
      const style = window.getComputedStyle(el)
      if (style.display === 'none' || style.visibility === 'hidden') return false
      // fixed 定位时 offsetParent 可能为 null，仍视为可聚焦
      if (style.position !== 'fixed' && style.position !== 'sticky') return false
    }
    return true
  })
}

export interface FocusTrapHandle {
  focusFirst: () => void
  release: () => void
}

/**
 * 在 root 内拦截 Tab，形成焦点循环；release 时恢复打开前焦点。
 * 依赖浏览器 DOM，单测仅覆盖 getFocusable 的选择逻辑在有 jsdom 时才测；此处导出纯查询。
 */
export function createFocusTrap(root: HTMLElement): FocusTrapHandle {
  const previous = document.activeElement instanceof HTMLElement ? document.activeElement : null

  function onKeyDown(e: KeyboardEvent) {
    if (e.key !== 'Tab') return
    const list = getFocusableElements(root)
    if (list.length === 0) {
      e.preventDefault()
      root.focus()
      return
    }
    const first = list[0]!
    const last = list[list.length - 1]!
    const active = document.activeElement
    if (e.shiftKey) {
      if (active === first || !root.contains(active)) {
        e.preventDefault()
        last.focus()
      }
    } else if (active === last || !root.contains(active)) {
      e.preventDefault()
      first.focus()
    }
  }

  // 面板本身可聚焦，便于无焦点元素时兜底
  if (!root.hasAttribute('tabindex')) root.setAttribute('tabindex', '-1')
  root.addEventListener('keydown', onKeyDown)

  return {
    focusFirst() {
      const list = getFocusableElements(root)
      const target = list[0] ?? root
      target.focus()
    },
    release() {
      root.removeEventListener('keydown', onKeyDown)
      if (previous && typeof previous.focus === 'function') {
        try {
          previous.focus()
        } catch {
          /* ignore */
        }
      }
    },
  }
}
