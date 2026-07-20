/** 部署侧签核备注：导出前完整性检查与归档 note 合成（不计入评分） */

import type { LocaleCode } from './localizedText.ts'
import type { SignoffNotes } from './readinessState.ts'
import { normalizeSignoffNotes } from './readinessState.ts'

export const SIGNOFF_NOTE_FIELDS = ['edgeHttps', 'backupOffsite', 'backupAlert'] as const
export type SignoffNoteField = (typeof SIGNOFF_NOTE_FIELDS)[number]

export interface SignoffCompleteness {
  filled: SignoffNoteField[]
  missing: SignoffNoteField[]
  complete: boolean
  filledCount: number
  total: number
}

const labels = {
  zh: {
    edgeHttps: '边缘证书/HSTS',
    backupOffsite: '异地备份',
    backupAlert: '备份失败告警',
    noteHead: '部署侧签核证据摘要',
    missingHead: '未填写签核备注字段',
  },
  en: {
    edgeHttps: 'Edge cert / HSTS',
    backupOffsite: 'Off-host backup',
    backupAlert: 'Backup failure alerting',
    noteHead: 'Deployment-side sign-off evidence summary',
    missingHead: 'Missing sign-off note fields',
  },
} as const

export function signoffFieldLabel(field: SignoffNoteField, locale: LocaleCode = 'zh'): string {
  return labels[locale === 'en' ? 'en' : 'zh'][field]
}

export function assessSignoffCompleteness(notes?: SignoffNotes | null): SignoffCompleteness {
  const n = normalizeSignoffNotes(notes)
  const filled: SignoffNoteField[] = []
  const missing: SignoffNoteField[] = []
  for (const field of SIGNOFF_NOTE_FIELDS) {
    if (n[field]) filled.push(field)
    else missing.push(field)
  }
  return {
    filled,
    missing,
    complete: missing.length === 0,
    filledCount: filled.length,
    total: SIGNOFF_NOTE_FIELDS.length,
  }
}

/** 0-100 进度（按已填字段数） */
export function signoffProgressPercent(notes?: SignoffNotes | null): number {
  const c = assessSignoffCompleteness(notes)
  if (!c.total) return 0
  return Math.round((c.filledCount / c.total) * 100)
}

export function signoffFieldAnchorId(field: SignoffNoteField): string {
  return `signoff-field-${field}`
}

export function formatSignoffMissingClipboard(
  missing: SignoffNoteField[],
  locale: LocaleCode = 'zh',
): string {
  if (!missing.length) {
    return locale === 'en'
      ? 'All sign-off note fields are filled (still not production acceptance).'
      : '三项签核备注均已填写（仍不等于生产验收完成）。'
  }
  return formatMissingSignoffMessage(missing, locale)
}

/** 将签核备注合成为归档顶层 note（可与已有 note 合并；不宣称验收完成） */
export function composeSignoffArchiveNote(
  notes?: SignoffNotes | null,
  opts?: { existingNote?: string; locale?: LocaleCode },
): string {
  const locale: LocaleCode = opts?.locale === 'en' ? 'en' : 'zh'
  const t = labels[locale]
  const n = normalizeSignoffNotes(notes)
  const parts: string[] = []
  for (const field of SIGNOFF_NOTE_FIELDS) {
    const v = n[field]
    if (v) parts.push(`${t[field]}: ${v}`)
  }
  const composed = parts.length ? `${t.noteHead}\n${parts.join('\n')}` : ''
  const existing = (opts?.existingNote ?? '').trim()
  if (existing && composed) return `${existing}\n\n${composed}`
  if (existing) return existing
  return composed
}

export function formatMissingSignoffMessage(
  missing: SignoffNoteField[],
  locale: LocaleCode = 'zh',
): string {
  if (!missing.length) return ''
  const t = labels[locale === 'en' ? 'en' : 'zh']
  const names = missing.map((f) => t[f]).join(locale === 'en' ? ', ' : '、')
  return `${t.missingHead}: ${names}`
}

/**
 * 导出前确认：缺失签核备注时提示仍可继续。
 * confirm 返回 true 才继续；无 window 时默认允许（SSR/测试）。
 */
export function confirmExportWithMissingSignoff(
  completeness: SignoffCompleteness,
  locale: LocaleCode = 'zh',
  confirmFn?: (msg: string) => boolean,
): boolean {
  if (completeness.complete) return true
  const msg =
    locale === 'en'
      ? `${formatMissingSignoffMessage(completeness.missing, 'en')}. Continue export? Notes do not complete production acceptance.`
      : `${formatMissingSignoffMessage(completeness.missing, 'zh')}。仍继续导出？备注不代表生产验收已完成。`
  if (confirmFn) return confirmFn(msg)
  if (typeof window !== 'undefined' && typeof window.confirm === 'function') {
    return window.confirm(msg)
  }
  return true
}
