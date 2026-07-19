/** 就绪/验收导出：将清单数据解析为指定 locale 的可序列化条目 */

import type { LocaleCode } from './localizedText.ts'
import { textForLocale } from './localizedText.ts'
import { PRODUCTION_CHECKLIST } from './productionChecklist.ts'
import { EDGE_HTTPS_ACCEPTANCE_STEPS } from './edgeHttpsAcceptance.ts'
import { BACKUP_SCHEDULE_STEPS } from './backupSchedule.ts'
import { BACKUP_DRILL_STEPS } from './backupDrill.ts'

export interface CatalogItem {
  id: string
  title: string
  detail: string
  severity: 'required' | 'recommended'
  done: boolean
  automated?: boolean
  partialAutomated?: boolean
  inDashboard?: boolean
  example?: string
}

function doneSet(ids: Iterable<string>): Set<string> {
  return new Set(ids)
}

export function productionCatalog(
  doneIds: Iterable<string>,
  locale: LocaleCode = 'zh',
): CatalogItem[] {
  const done = doneSet(doneIds)
  return PRODUCTION_CHECKLIST.map((item) => ({
    id: item.id,
    title: textForLocale(item.title, locale),
    detail: textForLocale(item.detail, locale),
    severity: item.severity,
    done: done.has(item.id),
    automated: item.automated,
  }))
}

export function edgeHttpsCatalog(
  doneIds: Iterable<string>,
  locale: LocaleCode = 'zh',
): CatalogItem[] {
  const done = doneSet(doneIds)
  return EDGE_HTTPS_ACCEPTANCE_STEPS.map((item) => ({
    id: item.id,
    title: textForLocale(item.title, locale),
    detail: textForLocale(item.detail, locale),
    severity: item.severity,
    done: done.has(item.id),
    partialAutomated: item.partialAutomated,
  }))
}

export function backupScheduleCatalog(
  doneIds: Iterable<string>,
  locale: LocaleCode = 'zh',
): CatalogItem[] {
  const done = doneSet(doneIds)
  return BACKUP_SCHEDULE_STEPS.map((item) => ({
    id: item.id,
    title: textForLocale(item.title, locale),
    detail: textForLocale(item.detail, locale),
    severity: item.severity,
    done: done.has(item.id),
    example: item.example,
  }))
}

export function backupDrillCatalog(
  doneIds: Iterable<string>,
  locale: LocaleCode = 'zh',
): CatalogItem[] {
  const done = doneSet(doneIds)
  return BACKUP_DRILL_STEPS.map((item) => ({
    id: item.id,
    title: textForLocale(item.title, locale),
    detail: textForLocale(item.detail, locale),
    severity: item.severity,
    done: done.has(item.id),
    inDashboard: item.inDashboard,
  }))
}

export function titlesById(items: CatalogItem[]): Record<string, string> {
  const out: Record<string, string> = {}
  for (const item of items) out[item.id] = item.title
  return out
}
