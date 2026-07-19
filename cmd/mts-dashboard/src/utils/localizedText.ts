/** 前后端无关的双语文案结构（清单、矩阵等纯数据共用） */

export type LocaleCode = 'zh' | 'en'

export interface LocalizedText {
  zh: string
  en: string
}

export function textForLocale(text: LocalizedText | undefined, locale: LocaleCode = 'zh'): string {
  if (!text) return ''
  return locale === 'en' ? text.en : text.zh
}
