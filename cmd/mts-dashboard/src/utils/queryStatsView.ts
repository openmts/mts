/** Query stats 展示纯函数 */

import type { QueryStatsData } from '@/api/types'

export type QueryStatCard = {
  key: string
  labelKey: string
  value: string
  tone?: 'default' | 'blue' | 'green' | 'amber' | 'rose'
}

function n(v: number | undefined | null): number {
  return typeof v === 'number' && Number.isFinite(v) ? v : 0
}

export function durationMs(stats?: QueryStatsData | null): number {
  return n(stats?.duration_nanos) / 1e6
}

export function formatDurationMs(stats?: QueryStatsData | null): string {
  const ms = durationMs(stats)
  if (!Number.isFinite(ms)) return '0.0ms'
  if (ms >= 1000) return `${(ms / 1000).toFixed(2)}s`
  return `${ms.toFixed(1)}ms`
}

/** 主卡片（与历史 5 卡布局兼容并扩展 parts） */
export function primaryStatCards(stats: QueryStatsData): QueryStatCard[] {
  return [
    { key: 'scan', labelKey: 'queryStatScan', value: String(n(stats.shards_scanned)), tone: 'default' },
    { key: 'skip', labelKey: 'queryStatSkip', value: String(n(stats.shards_skipped)), tone: 'default' },
    { key: 'read', labelKey: 'queryStatRead', value: String(n(stats.samples_read)), tone: 'blue' },
    { key: 'return', labelKey: 'queryStatReturn', value: String(n(stats.samples_returned)), tone: 'green' },
    { key: 'duration', labelKey: 'queryStatDuration', value: formatDurationMs(stats), tone: 'amber' },
  ]
}

/** 扩展诊断指标 */
export function detailStatCards(stats: QueryStatsData): QueryStatCard[] {
  return [
    { key: 'cand', labelKey: 'queryStatCandidates', value: String(n(stats.candidate_shards)) },
    { key: 'parts_scan', labelKey: 'queryStatPartsScan', value: String(n(stats.parts_scanned)) },
    { key: 'parts_skip', labelKey: 'queryStatPartsSkip', value: String(n(stats.parts_skipped)) },
    { key: 'idx_read', labelKey: 'queryStatIndexRead', value: String(n(stats.index_rows_read)) },
    { key: 'idx_skip', labelKey: 'queryStatIndexSkip', value: String(n(stats.index_rows_skipped)) },
    { key: 'tblocks', labelKey: 'queryStatTimeBlocks', value: String(n(stats.time_blocks_read)) },
    { key: 'vblocks', labelKey: 'queryStatValueBlocks', value: String(n(stats.value_blocks_read)) },
    { key: 'pages_read', labelKey: 'queryStatPagesRead', value: String(n(stats.value_pages_read)) },
    { key: 'pages_skip', labelKey: 'queryStatPagesSkip', value: String(n(stats.value_pages_skipped)) },
    { key: 'errors', labelKey: 'queryStatErrors', value: String(n(stats.errors)), tone: n(stats.errors) ? 'rose' : 'default' },
    { key: 'budget', labelKey: 'queryStatBudgetErrors', value: String(n(stats.budget_errors)), tone: n(stats.budget_errors) ? 'rose' : 'default' },
    { key: 'cancel', labelKey: 'queryStatCancellations', value: String(n(stats.cancellations)) },
    { key: 'epoch', labelKey: 'queryStatReadEpoch', value: String(n(stats.read_epoch)) },
  ]
}

export function isEmptyStats(stats?: QueryStatsData | null): boolean {
  if (!stats) return true
  return (
    n(stats.samples_read) === 0 &&
    n(stats.samples_returned) === 0 &&
    n(stats.shards_scanned) === 0 &&
    n(stats.duration_nanos) === 0 &&
    n(stats.errors) === 0
  )
}

export function toneClass(tone?: QueryStatCard['tone']): string {
  switch (tone) {
    case 'blue':
      return 'text-blue-600 dark:text-blue-400'
    case 'green':
      return 'text-green-600 dark:text-green-400'
    case 'amber':
      return 'text-amber-600 dark:text-amber-400'
    case 'rose':
      return 'text-rose-600 dark:text-rose-400'
    default:
      return 'text-slate-900 dark:text-slate-100'
  }
}
