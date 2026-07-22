import assert from 'node:assert/strict'
import test from 'node:test'
import {
  queryAdminOpPayload,
  queryResultPath,
  queryResultRowCount,
  queryResultSeriesCount,
} from './queryResultMeta.ts'

test('queryResultPath/count prefer server', () => {
  assert.equal(queryResultPath({ path: '/api/v1/data/query/rows' }, '/x'), '/api/v1/data/query/rows')
  assert.equal(queryResultPath({}, '/api/v1/data/query/rows'), '/api/v1/data/query/rows')
  assert.equal(queryResultRowCount({ row_count: 3 }, 9), 3)
  assert.equal(queryResultRowCount({}, 9), 9)
  assert.equal(queryResultSeriesCount({ series_count: 2 }, 5), 2)
})

test('queryAdminOpPayload null when idle empty', () => {
  assert.equal(queryAdminOpPayload({}), null)
  assert.equal(queryAdminOpPayload(null), null)
  const p = queryAdminOpPayload({ admin_op_busy: false, op: '', started_at_unix: 0, last: null })
  assert.ok(p)
  assert.equal(p!.admin_op_busy, false)
})
