import assert from 'node:assert/strict'
import test from 'node:test'
import {
  filterPrometheusFamilies,
  formatSampleLabels,
  parsePrometheusText,
  summarizeFamilies,
} from './prometheus.ts'

const SAMPLE = `
# HELP http_requests_total Total HTTP requests.
# TYPE http_requests_total counter
http_requests_total{method="GET",code="200"} 12
http_requests_total{method="POST",code="500"} 3
# HELP mts_ready Ready gauge
# TYPE mts_ready gauge
mts_ready 1
`

test('parsePrometheusText extracts families and labels', () => {
  const fams = parsePrometheusText(SAMPLE)
  assert.equal(fams.length, 2)
  const http = fams.find((f) => f.name === 'http_requests_total')
  assert.ok(http)
  assert.equal(http.type, 'counter')
  assert.match(http.help, /Total HTTP/)
  assert.equal(http.samples.length, 2)
  assert.equal(http.samples[0].labels.method, 'GET')
  assert.equal(http.samples[0].value, 12)
  const ready = fams.find((f) => f.name === 'mts_ready')
  assert.equal(ready?.samples[0].value, 1)
})

test('filter and summarize', () => {
  const fams = parsePrometheusText(SAMPLE)
  const filtered = filterPrometheusFamilies(fams, 'POST')
  assert.equal(filtered.length, 1)
  assert.equal(filtered[0].samples.length, 1)
  const sum = summarizeFamilies(fams)
  assert.equal(sum.families, 2)
  assert.equal(sum.samples, 3)
  assert.equal(formatSampleLabels({ b: '2', a: '1' }), 'a="1", b="2"')
})
