/** 健康检查 status 展示标签 */

export type HealthStatusLocale = 'zh' | 'en'

const MAP: Record<string, { zh: string; en: string }> = {
  ok: { zh: '正常', en: 'OK' },
  passed: { zh: '通过', en: 'Passed' },
  pass: { zh: '通过', en: 'Pass' },
  healthy: { zh: '健康', en: 'Healthy' },
  ready: { zh: '就绪', en: 'Ready' },
  fail: { zh: '失败', en: 'Fail' },
  failed: { zh: '失败', en: 'Failed' },
  error: { zh: '错误', en: 'Error' },
  warn: { zh: '警告', en: 'Warn' },
  warning: { zh: '警告', en: 'Warning' },
  unknown: { zh: '未知', en: 'Unknown' },
  skipped: { zh: '跳过', en: 'Skipped' },
}

export function healthStatusLabel(
  status: string | null | undefined,
  locale: HealthStatusLocale = 'zh',
): string {
  const raw = String(status || '').trim()
  if (!raw) return locale === 'en' ? '—' : '—'
  const key = raw.toLowerCase()
  const hit = MAP[key]
  if (hit) return hit[locale === 'en' ? 'en' : 'zh']
  return raw
}

export function healthStatusToneClass(status: string | null | undefined): string {
  const key = String(status || '').trim().toLowerCase()
  if (['ok', 'passed', 'pass', 'healthy', 'ready'].includes(key)) {
    return 'text-green-600 dark:text-green-400'
  }
  if (['warn', 'warning', 'skipped'].includes(key)) {
    return 'text-amber-600 dark:text-amber-300'
  }
  if (['fail', 'failed', 'error'].includes(key)) {
    return 'text-red-600 dark:text-red-400'
  }
  return 'text-slate-600 dark:text-slate-300'
}
