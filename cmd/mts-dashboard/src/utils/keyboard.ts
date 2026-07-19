/** 键盘快捷键辅助 */

export function isEditableTarget(target: EventTarget | null): boolean {
  if (target == null || typeof target !== 'object') return false
  const el = target as {
    tagName?: string
    isContentEditable?: boolean
    closest?: (sel: string) => unknown
  }
  const tag = typeof el.tagName === 'string' ? el.tagName.toUpperCase() : ''
  if (tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT') return true
  if (el.isContentEditable) return true
  if (typeof el.closest === 'function') {
    return Boolean(el.closest('[contenteditable="true"]'))
  }
  return false
}

export function isModKey(e: KeyboardEvent): boolean {
  return e.metaKey || e.ctrlKey
}

/**
 * 查询页快捷键：
 * - Ctrl/Cmd+Enter: run
 * - Escape: cancel / close history
 * - Ctrl/Cmd+Shift+C: copy results
 * - Ctrl/Cmd+H: toggle history
 */
export type QueryShortcutAction = 'run' | 'cancel' | 'copy' | 'toggle-history'

export function matchQueryShortcut(e: KeyboardEvent): QueryShortcutAction | null {
  if (e.key === 'Escape') return 'cancel'
  if (!isModKey(e)) return null
  if (e.key === 'Enter') return 'run'
  if (e.key === 'h' || e.key === 'H') return 'toggle-history'
  if ((e.key === 'c' || e.key === 'C') && e.shiftKey) return 'copy'
  return null
}
