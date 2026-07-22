import assert from 'node:assert/strict'
import test from 'node:test'
import { extractMetricsHealthSignals } from './metricsHealthSignals.ts'
import type { PrometheusFamily } from './prometheus.ts'

function fam(name: string, value: number, extra: { value: number }[] = []): PrometheusFamily {
  return {
    name,
    help: '',
    type: 'gauge',
    samples: [{ name, labels: {}, value, raw: '' }, ...extra.map((e) => ({ name, labels: {}, value: e.value, raw: '' }))],
  }
}

test('extractMetricsHealthSignals ok tone', () => {
  const s = extractMetricsHealthSignals([
    fam('mts_health_healthy', 1),
    fam('mts_health_ready', 1),
    fam('mts_compaction_backlog', 0),
    fam('mts_maintenance_error_count', 0),
    { name: 'mts_server_requests_total', help: '', type: 'counter', samples: [
      { name: 'mts_server_requests_total', labels: {}, value: 10, raw: '' },
      { name: 'mts_server_requests_total', labels: {}, value: 5, raw: '' },
    ]},
    { name: 'mts_server_request_errors_total', help: '', type: 'counter', samples: [
      { name: 'mts_server_request_errors_total', labels: {}, value: 0, raw: '' },
    ]},
  ])
  assert.equal(s.tone, 'ok')
  assert.equal(s.healthy, 1)
  assert.equal(s.request_total, 15)
  assert.equal(s.request_errors_total, 0)
})

test('extractMetricsHealthSignals bad when unhealthy', () => {
  const s = extractMetricsHealthSignals([
    fam('mts_health_healthy', 0),
    fam('mts_health_ready', 1),
  ])
  assert.equal(s.tone, 'bad')
})

test('extractMetricsHealthSignals warn on backlog', () => {
  const s = extractMetricsHealthSignals([
    fam('mts_health_healthy', 1),
    fam('mts_health_ready', 1),
    fam('mts_compaction_backlog', 3),
  ])
  assert.equal(s.tone, 'warn')
  assert.equal(s.compaction_backlog, 3)
})

test('extractMetricsHealthSignals empty unknown', () => {
  assert.equal(extractMetricsHealthSignals([]).tone, 'unknown')
  assert.equal(extractMetricsHealthSignals(null).tone, 'unknown')
})
