import test from 'node:test'
import assert from 'node:assert/strict'
import {
  beginExportJob,
  cancelExportJob,
  createExportJobState,
  exportProgressPercent,
  failExportJob,
  finishExportJob,
  isExportJobBusy,
  joinTextWithExportProgress,
  exportYieldMs,
  mapWithExportProgress,
  progressExportJob,
} from './exportJob.ts'

test('export job lifecycle and percent', () => {
  assert.equal(isExportJobBusy(createExportJobState()), false)
  let s = beginExportJob('csv', 10)
  assert.equal(isExportJobBusy(s), true)
  assert.equal(exportProgressPercent(s), 0)
  s = progressExportJob(s, 5)
  assert.equal(exportProgressPercent(s), 50)
  s = finishExportJob(s)
  assert.equal(s.status, 'done')
  assert.equal(exportProgressPercent(s), 100)
  s = cancelExportJob(beginExportJob('x', 3))
  assert.equal(s.status, 'cancelled')
  s = failExportJob(beginExportJob('x'), 'boom')
  assert.equal(s.status, 'error')
  assert.equal(s.error, 'boom')
})

test('mapWithExportProgress completes and can cancel', async () => {
  const items = [1, 2, 3, 4]
  const out = await mapWithExportProgress(items, (x) => x * 2, {
    cancelled: () => false,
    chunkSize: 2,
  })
  assert.deepEqual(out, [2, 4, 6, 8])

  let n = 0
  const cancelled = await mapWithExportProgress(items, (x) => {
    n++
    return x
  }, {
    cancelled: () => n >= 2,
    chunkSize: 1,
  })
  assert.equal(cancelled, null)
})

test('joinTextWithExportProgress', async () => {
  const text = await joinTextWithExportProgress('h', ['a', 'b'], {
    cancelled: () => false,
    chunkSize: 1,
  })
  assert.equal(text, 'h\na\nb')
  const cancelled = await joinTextWithExportProgress('h', ['a', 'b', 'c'], {
    cancelled: () => true,
  })
  assert.equal(cancelled, null)
})

test('exportYieldMs resolves', async () => {
  const t0 = Date.now()
  await exportYieldMs(0)
  await exportYieldMs(5)
  assert.ok(Date.now() - t0 >= 0)
})
