/** 全局快捷键帮助目录（纯数据） */

export interface ShortcutEntry {
  id: string
  keys: string
  labelKey: string
}

export const DASHBOARD_SHORTCUTS: ShortcutEntry[] = [
  { id: 'palette', keys: 'Ctrl/⌘ + K', labelKey: 'shortcutPalette' },
  { id: 'notify-history', keys: 'Ctrl/⌘ + Shift + H', labelKey: 'shortcutNotifyHistory' },
  { id: 'downsample-errors', keys: 'g then d', labelKey: 'shortcutDownsampleErrors' },
  { id: 'nav-filter', keys: '/', labelKey: 'shortcutNavFilter' },
  { id: 'help', keys: '?', labelKey: 'shortcutHelp' },
  { id: 'skip', keys: 'Tab', labelKey: 'shortcutSkip' },
]

/** 序列快捷键 g 后 d：打开降采样错误状态（非输入态） */
export function matchSequenceChordStartG(e: KeyboardEvent, editable: boolean): boolean {
  if (editable) return false
  if (e.metaKey || e.ctrlKey || e.altKey || e.shiftKey) return false
  return e.key === 'g' || e.key === 'G'
}

export function matchSequenceChordD(e: KeyboardEvent, editable: boolean): boolean {
  if (editable) return false
  if (e.metaKey || e.ctrlKey || e.altKey || e.shiftKey) return false
  return e.key === 'd' || e.key === 'D'
}

export const DOWNSAMPLE_ERROR_JUMP_PATH = '/downsample?health=error#downsample-status'


export function matchShortcutHelpOpen(e: KeyboardEvent, editable: boolean): boolean {
  if (editable) return false
  if (e.metaKey || e.ctrlKey || e.altKey) return false
  return e.key === '?' || (e.shiftKey && e.key === '/')
}

/** 非输入态按 / 聚焦侧栏导航过滤（不含 Shift+/ 的 ?） */
export function matchSidebarFilterFocus(e: KeyboardEvent, editable: boolean): boolean {
  if (editable) return false
  if (e.metaKey || e.ctrlKey || e.altKey || e.shiftKey) return false
  return e.key === '/'
}

/** Ctrl/⌘+Shift+H 打开通知历史（输入框内也允许，运维常用） */
export function matchNotifyHistoryOpen(e: KeyboardEvent): boolean {
  if (!(e.metaKey || e.ctrlKey)) return false
  if (!e.shiftKey || e.altKey) return false
  return e.key === 'h' || e.key === 'H'
}

export function isEditableTarget(target: EventTarget | null): boolean {
  if (target == null || typeof target !== 'object') return false
  if (typeof HTMLElement !== 'undefined' && target instanceof HTMLElement) {
    const tag = target.tagName
    if (tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT') return true
    if (target.isContentEditable) return true
    return target.closest('[contenteditable="true"]') != null
  }
  const el = target as { tagName?: string; isContentEditable?: boolean }
  const tag = String(el.tagName || '').toUpperCase()
  if (tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT') return true
  return !!el.isContentEditable
}
