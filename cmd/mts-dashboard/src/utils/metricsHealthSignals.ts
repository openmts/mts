/** 从 Prometheus families 提取运维健康信号（纯函数） */

import type { PrometheusFamily } from './prometheus.ts'

export interface MetricsHealthSignals {
  healthy: number | null
  ready: number | null
  compaction_backlog: number | null
  maintenance_errors: number | null
  request_errors_total: number | null
  request_total: number | null
  tone: 'ok' | 'warn' | 'bad' | 'unknown'
}

function gaugeValue(families: PrometheusFamily[], name: string): number | null {
  const fam = families.find((f) => f.name === name)
  if (!fam || !fam.samples.length) return null
  const v = fam.samples[0]?.value
  return typeof v === 'number' && Number.isFinite(v) ? v : null
}

function sumSamples(families: PrometheusFamily[], name: string): number | null {
  const fam = families.find((f) => f.name === name)
  if (!fam) return null
  if (!fam.samples.length) return 0
  let sum = 0
  for (const s of fam.samples) {
    if (typeof s.value === 'number' && Number.isFinite(s.value)) sum += s.value
  }
  return sum
}

function resolveTone(s: Omit<MetricsHealthSignals, 'tone'>): MetricsHealthSignals['tone'] {
  const hasAny = [s.healthy, s.ready, s.compaction_backlog, s.maintenance_errors, s.request_errors_total]
    .some((x) => x !== null)
  if (!hasAny) return 'unknown'
  if (s.healthy === 0 || s.ready === 0) return 'bad'
  if ((s.maintenance_errors ?? 0) > 0) return 'bad'
  if ((s.compaction_backlog ?? 0) > 0) return 'warn'
  if ((s.request_errors_total ?? 0) > 0) return 'warn'
  return 'ok'
}

/** 关键健康/积压/错误计数扫视摘要 */
export function extractMetricsHealthSignals(families: PrometheusFamily[] | null | undefined): MetricsHealthSignals {
  const list = Array.isArray(families) ? families : []
  const base = {
    healthy: gaugeValue(list, 'mts_health_healthy'),
    ready: gaugeValue(list, 'mts_health_ready'),
    compaction_backlog: gaugeValue(list, 'mts_compaction_backlog'),
    maintenance_errors: gaugeValue(list, 'mts_maintenance_error_count'),
    request_errors_total: sumSamples(list, 'mts_server_request_errors_total'),
    request_total: sumSamples(list, 'mts_server_requests_total'),
  }
  return { ...base, tone: resolveTone(base) }
}
