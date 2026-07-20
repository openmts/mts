/** 通知历史导出（纯函数） */

import type { NotifyHistoryEntry } from './notifyHistory.ts'

export interface NotifyHistoryExportPayload {
  generated_at: string
  count: number
  items: Array<{
    kind: string
    message: string
    count: number
    at: string
    at_ms: number
  }>
}

export function buildNotifyHistoryExport(
  entries: readonly NotifyHistoryEntry[],
  now = new Date(),
): NotifyHistoryExportPayload {
  return {
    generated_at: now.toISOString(),
    count: entries.length,
    items: entries.map((e) => ({
      kind: e.kind,
      message: e.message,
      count: e.count,
      at: new Date(e.at).toISOString(),
      at_ms: e.at,
    })),
  }
}

export function formatNotifyHistoryExportPretty(
  entries: readonly NotifyHistoryEntry[],
  now = new Date(),
): string {
  return JSON.stringify(buildNotifyHistoryExport(entries, now), null, 2)
}

function csvEscape(v: string): string {
  const s = String(v ?? '')
  if (/[",\n\r]/.test(s)) return `"${s.replace(/"/g, '""')}"`
  return s
}

export function notifyHistoryToCSV(entries: readonly NotifyHistoryEntry[]): string {
  const header = ['at', 'kind', 'count', 'message']
  const lines = [header.join(',')]
  for (const e of entries) {
    lines.push(
      [
        csvEscape(new Date(e.at).toISOString()),
        csvEscape(e.kind),
        String(e.count),
        csvEscape(e.message),
      ].join(','),
    )
  }
  return lines.join('\n') + (lines.length > 1 ? '\n' : '')
}
