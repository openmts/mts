import test from 'node:test'
import assert from 'node:assert/strict'
import {
  buildNotifyHistoryPrefillPath,
  notifyHistoryFormToPrefill,
  parseNotifyHistoryPrefill,
} from './notifyHistoryPrefill.ts'

test('parseNotifyHistoryPrefill open flags and filters', () => {
  assert.deepEqual(parseNotifyHistoryPrefill({ notify: '1', nh_kind: 'error', nh_q: 'fail', nh_range: '24h' }), {
    open: true,
    kind: 'error',
    q: 'fail',
    range: '24h',
  })
  assert.deepEqual(parseNotifyHistoryPrefill({}, '#notify-history'), { open: true })
  assert.deepEqual(parseNotifyHistoryPrefill({ nh_kind: 'warn' }), { open: true, kind: 'warn' })
  assert.deepEqual(parseNotifyHistoryPrefill({ notify: '0' }), {})
})

test('build and form helpers', () => {
  assert.equal(
    buildNotifyHistoryPrefillPath({ kind: 'error', q: 'x', range: '1h', path: '/query' }),
    '/query?notify=1&nh_kind=error&nh_q=x&nh_range=1h#notify-history',
  )
  assert.equal(
    notifyHistoryFormToPrefill({ kind: 'all', q: '  ', range: 'all' }, { path: '/audit' }),
    '/audit?notify=1#notify-history',
  )
})
