/** 全局快捷键帮助目录（纯数据） */

export interface ShortcutEntry {
  id: string
  keys: string
  labelKey: string
}

export const DASHBOARD_SHORTCUTS: ShortcutEntry[] = [
  { id: 'palette', keys: 'Ctrl/⌘ + K', labelKey: 'shortcutPalette' },
  { id: 'help', keys: '?', labelKey: 'shortcutHelp' },
  { id: 'skip', keys: 'Tab', labelKey: 'shortcutSkip' },
]

export function matchShortcutHelpOpen(e: KeyboardEvent, editable: boolean): boolean {
  if (editable) return false
  if (e.metaKey || e.ctrlKey || e.altKey) return false
  return e.key === '?' || (e.shiftKey && e.key === '/')
}

export function isEditableTarget(target: EventTarget | null): boolean {
  if (target == null || typeof target !== 'object') return false
  if (typeof HTMLElement !== 'undefined' && target instanceof HTMLElement) {
    const tag = target.tagName
    if (tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT') return true
    if (target.isContentEditable) return true
    return target.closest('[contenteditable="true"]') != null
  }
  // Node 单测环境无 DOM：duck-type
  const el = target as { tagName?: string; isContentEditable?: boolean }
  const tag = String(el.tagName || '').toUpperCase()
  if (tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT') return true
  return !!el.isContentEditable
}
