/** 库级权限展示标签（API 值不变，仅 UI 本地化） */

import type { LocaleCode } from './localizedText.ts'

export type DbPermission = 'read' | 'write' | 'admin' | string

const labels: Record<'read' | 'write' | 'admin', Record<LocaleCode, string>> = {
  read: { zh: '读', en: 'Read' },
  write: { zh: '写', en: 'Write' },
  admin: { zh: '管理', en: 'Admin' },
}

export function permissionLabel(perm: string | null | undefined, locale: LocaleCode = 'zh'): string {
  if (!perm) return locale === 'en' ? '—' : '—'
  const key = perm as 'read' | 'write' | 'admin'
  if (key in labels) return labels[key][locale === 'en' ? 'en' : 'zh']
  return perm
}

export const DB_PERMISSIONS = ['read', 'write', 'admin'] as const
