import assert from 'node:assert/strict'
import test from 'node:test'
import {
  assessSignoffCompleteness,
  composeSignoffArchiveNote,
  confirmExportWithMissingSignoff,
  formatMissingSignoffMessage,
  signoffFieldLabel,
} from './signoffExport.ts'

test('assessSignoffCompleteness reports missing fields', () => {
  const empty = assessSignoffCompleteness({})
  assert.equal(empty.complete, false)
  assert.equal(empty.filledCount, 0)
  assert.deepEqual(empty.missing, ['edgeHttps', 'backupOffsite', 'backupAlert'])

  const partial = assessSignoffCompleteness({ edgeHttps: 'cert ok', backupAlert: 'webhook' })
  assert.equal(partial.complete, false)
  assert.deepEqual(partial.filled.sort(), ['backupAlert', 'edgeHttps'])
  assert.deepEqual(partial.missing, ['backupOffsite'])

  const full = assessSignoffCompleteness({
    edgeHttps: 'a',
    backupOffsite: 'b',
    backupAlert: 'c',
  })
  assert.equal(full.complete, true)
  assert.equal(full.filledCount, 3)
  assert.deepEqual(full.missing, [])
})

test('composeSignoffArchiveNote merges existing note', () => {
  const note = composeSignoffArchiveNote(
    { edgeHttps: 'openssl ok', backupOffsite: 'rsync host' },
    { existingNote: 'handoff for release', locale: 'zh' },
  )
  assert.match(note, /handoff for release/)
  assert.match(note, /部署侧签核证据摘要/)
  assert.match(note, /边缘证书\/HSTS: openssl ok/)
  assert.match(note, /异地备份: rsync host/)
})

test('composeSignoffArchiveNote english and empty', () => {
  const en = composeSignoffArchiveNote({ backupAlert: 'pager' }, { locale: 'en' })
  assert.match(en, /Deployment-side sign-off evidence summary/)
  assert.match(en, /Backup failure alerting: pager/)
  assert.equal(composeSignoffArchiveNote({}), '')
  assert.equal(composeSignoffArchiveNote({}, { existingNote: ' only ' }), 'only')
})

test('formatMissingSignoffMessage and labels', () => {
  assert.match(formatMissingSignoffMessage(['edgeHttps', 'backupAlert'], 'zh'), /边缘证书/)
  assert.match(formatMissingSignoffMessage(['backupOffsite'], 'en'), /Off-host backup/)
  assert.equal(formatMissingSignoffMessage([], 'zh'), '')
  assert.equal(signoffFieldLabel('backupAlert', 'en'), 'Backup failure alerting')
})

test('confirmExportWithMissingSignoff respects confirm result', () => {
  const missing = assessSignoffCompleteness({})
  assert.equal(
    confirmExportWithMissingSignoff(missing, 'zh', () => false),
    false,
  )
  assert.equal(
    confirmExportWithMissingSignoff(missing, 'en', () => true),
    true,
  )
  const full = assessSignoffCompleteness({
    edgeHttps: 'a',
    backupOffsite: 'b',
    backupAlert: 'c',
  })
  let called = false
  assert.equal(
    confirmExportWithMissingSignoff(full, 'zh', () => {
      called = true
      return false
    }),
    true,
  )
  assert.equal(called, false)
})
