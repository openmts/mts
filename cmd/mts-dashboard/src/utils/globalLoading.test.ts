import assert from 'node:assert/strict'
import test from 'node:test'
import {
  beginRequest,
  endRequest,
  setRouteLoading,
  useGlobalLoading,
} from '../composables/useGlobalLoading.ts'

test('inflight counter and route loading', () => {
  // reset by draining
  while (useGlobalLoading().requestCount.value > 0) endRequest()
  setRouteLoading(false)

  beginRequest()
  beginRequest()
  assert.equal(useGlobalLoading().requestCount.value, 2)
  assert.equal(useGlobalLoading().busy.value, true)
  endRequest()
  assert.equal(useGlobalLoading().requestCount.value, 1)
  endRequest()
  assert.equal(useGlobalLoading().requestCount.value, 0)
  assert.equal(useGlobalLoading().busy.value, false)
  setRouteLoading(true)
  assert.equal(useGlobalLoading().busy.value, true)
  setRouteLoading(false)
  assert.equal(useGlobalLoading().busy.value, false)
  endRequest() // underfloor
  assert.equal(useGlobalLoading().requestCount.value, 0)
})
