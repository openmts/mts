import assert from 'node:assert/strict'
import test from 'node:test'
import { useExportJob } from './useExportJob.ts'

test('stale cancelled export does not overwrite the newer failed export', async () => {
  const browser = globalThis as unknown as { window?: Record<string, unknown> }
  const previousWindow = browser.window
  browser.window = { __MTS_E2E_SLOW_EXPORT_MS: 100 }

  try {
    const job = useExportJob()
    const stale = job.runTextExport({
      label: 'stale',
      filename: 'stale.txt',
      build: async () => 'stale',
    })
    job.cancelExport()

    delete browser.window.__MTS_E2E_SLOW_EXPORT_MS
    browser.window.__MTS_E2E_FAIL_EXPORT = true
    const current = await job.runTextExport({
      label: 'current',
      filename: 'current.txt',
      build: async () => 'current',
    })

    assert.equal(current, 'error')
    assert.equal(await stale, 'cancelled')
    assert.equal(job.exportJob.value.status, 'error')
  } finally {
    if (previousWindow === undefined) delete browser.window
    else browser.window = previousWindow
  }
})
