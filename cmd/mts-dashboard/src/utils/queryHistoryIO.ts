/** 查询历史导入/导出载荷（纯函数） */

import {
  mergeHistoryCap,
  normalizeHistoryItems,
  type QueryHistoryRecord,
} from './queryHistory.ts'

export const HISTORY_EXPORT_VERSION = 1 as const

export interface HistoryExportPayload {
  version: typeof HISTORY_EXPORT_VERSION
  exported_at: number
  items: QueryHistoryRecord[]
}

export function buildHistoryExport(items: QueryHistoryRecord[], now = Date.now()): HistoryExportPayload {
  return {
    version: HISTORY_EXPORT_VERSION,
    exported_at: now,
    items: items.map((x) => ({ ...x, form: { ...x.form } })),
  }
}

export function parseHistoryImport(raw: unknown): { ok: true; items: QueryHistoryRecord[] } | { ok: false; error: string } {
  if (Array.isArray(raw)) {
    return { ok: true, items: normalizeHistoryItems(raw) }
  }
  if (raw == null || typeof raw !== 'object') {
    return { ok: false, error: '无效的历史文件' }
  }
  const o = raw as Record<string, unknown>
  if (Array.isArray(o.items)) {
    return { ok: true, items: normalizeHistoryItems(o.items) }
  }
  return { ok: false, error: '缺少 items 字段' }
}

/** merge=true：按 id 去重后合并；false：整表替换 */
export function mergeImportedHistory(
  current: QueryHistoryRecord[],
  incoming: QueryHistoryRecord[],
  opts: { merge: boolean; max: number },
): QueryHistoryRecord[] {
  if (!opts.merge) {
    return mergeHistoryCap(incoming, opts.max)
  }
  const byId = new Map<string, QueryHistoryRecord>()
  for (const x of current) byId.set(x.id, x)
  for (const x of incoming) {
    const prev = byId.get(x.id)
    if (!prev || x.at >= prev.at) byId.set(x.id, x)
  }
  return mergeHistoryCap([...byId.values()], opts.max)
}
