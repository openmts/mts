/** Metrics 页面导出（纯函数） */

import type { PrometheusFamily } from './prometheus.ts'
import {
  normalizeDownsampleStatusSummary,
  type DownsampleStatusSummaryInput,
} from './downsampleStatusSummary.ts'

export function metricsFamiliesToJSON(
  families: PrometheusFamily[],
  opts?: {
    downsample_status_summary?: DownsampleStatusSummaryInput | null
  },
): {
  kind: 'mts.metrics.families'
  version: 2
  count: number
  families: PrometheusFamily[]
  downsample_status_summary: ReturnType<typeof normalizeDownsampleStatusSummary> | null
} {
  const list = families || []
  return {
    kind: 'mts.metrics.families',
    version: 2,
    count: list.length,
    families: list,
    downsample_status_summary: opts?.downsample_status_summary
      ? normalizeDownsampleStatusSummary(opts.downsample_status_summary)
      : null,
  }
}

export function metricsRefreshIntervalsMs(): number[] {
  return [0, 15_000, 30_000, 60_000]
}
