/** 通知历史导出（纯函数） */

import type { NotifyHistoryEntry } from './notifyHistory.ts'
import { escapeCSVCell } from './csvEscape.ts'

export interface NotifyHistoryExportPayload {
  generated_at: string
  count: number
  items: Array<{
    kind: string
    message: string
    count: number
    at: string
    at_ms: number
    action_label?: string
    action_path?: string
  }>
}

export function buildNotifyHistoryExport(
  entries: readonly NotifyHistoryEntry[],
  now = new Date(),
): NotifyHistoryExportPayload {
  return {
    generated_at: now.toISOString(),
    count: entries.length,
    items: entries.map((e) => {
      const item: NotifyHistoryExportPayload['items'][number] = {
        kind: e.kind,
        message: e.message,
        count: e.count,
        at: new Date(e.at).toISOString(),
        at_ms: e.at,
      }
      if (e.actionPath) {
        item.action_path = e.actionPath
        item.action_label = e.actionLabel || e.actionPath
      }
      return item
    }),
  }
}

export function formatNotifyHistoryExportPretty(
  entries: readonly NotifyHistoryEntry[],
  now = new Date(),
): string {
  return JSON.stringify(buildNotifyHistoryExport(entries, now), null, 2)
}

export function notifyHistoryToCSV(entries: readonly NotifyHistoryEntry[]): string {
  const header = ['at', 'kind', 'count', 'message', 'action_label', 'action_path']
  const lines = [header.join(',')]
  for (const e of entries) {
    lines.push(
      [
        escapeCSVCell(new Date(e.at).toISOString()),
        escapeCSVCell(e.kind),
        String(e.count),
        escapeCSVCell(e.message),
        escapeCSVCell(e.actionLabel || ''),
        escapeCSVCell(e.actionPath || ''),
      ].join(','),
    )
  }
  return lines.join('\n') + (lines.length > 1 ? '\n' : '')
}
