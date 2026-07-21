import assert from 'node:assert/strict'
import test from 'node:test'
import {
  beginRequest,
  endRequest,
  LONG_REQUEST_THRESHOLD_MS,
  resetGlobalLoading,
  setRouteLoading,
  useGlobalLoading,
} from '../composables/useGlobalLoading.ts'

test('inflight counter and route loading', () => {
  resetGlobalLoading()
  const g = useGlobalLoading()
  assert.equal(g.requestCount.value, 0)
  beginRequest()
  beginRequest()
  assert.equal(g.requestCount.value, 2)
  assert.equal(g.busy.value, true)
  endRequest()
  assert.equal(g.requestCount.value, 1)
  endRequest()
  assert.equal(g.requestCount.value, 0)
  assert.equal(g.busy.value, false)
  setRouteLoading(true)
  assert.equal(g.busy.value, true)
  setRouteLoading(false)
  assert.equal(g.busy.value, false)
  endRequest()
  assert.equal(g.requestCount.value, 0)
})

test('longBusy becomes true after threshold', async () => {
  resetGlobalLoading()
  const g = useGlobalLoading({ longThresholdMs: 30 })
  assert.equal(LONG_REQUEST_THRESHOLD_MS > 0, true)
  beginRequest()
  assert.equal(g.busy.value, true)
  assert.equal(g.longBusy.value, false)
  await new Promise((r) => setTimeout(r, 350))
  assert.equal(g.longBusy.value, true)
  endRequest()
  assert.equal(g.longBusy.value, false)
  resetGlobalLoading()
})
