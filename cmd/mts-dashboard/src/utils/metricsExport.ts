/** Metrics 页面导出（纯函数） */

import type { PrometheusFamily } from './prometheus.ts'

export function metricsFamiliesToJSON(families: PrometheusFamily[]): {
  kind: 'mts.metrics.families'
  version: 1
  count: number
  families: PrometheusFamily[]
} {
  const list = families || []
  return {
    kind: 'mts.metrics.families',
    version: 1,
    count: list.length,
    families: list,
  }
}

export function metricsRefreshIntervalsMs(): number[] {
  return [0, 15_000, 30_000, 60_000]
}
