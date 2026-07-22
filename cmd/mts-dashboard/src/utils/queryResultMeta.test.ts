import assert from 'node:assert/strict'
import test from 'node:test'
import {
  queryResultPath,
  queryResultDatabase,
  queryResultMeasurement,
  queryResultRowCount,
  queryResultSeriesCount,
  queryAdminOpPayload,
  streamEndPath,
  streamEndRecordCount,
} from './queryResultMeta.ts'

test('queryResultPath/count prefer server', () => {
  assert.equal(queryResultPath({ path: '/api/v1/data/query/rows' }, '/x'), '/api/v1/data/query/rows')
  assert.equal(queryResultPath({}, '/api/v1/data/query/rows'), '/api/v1/data/query/rows')
  assert.equal(queryResultRowCount({ row_count: 9 }, 1), 9)
  assert.equal(queryResultRowCount({}, 4), 4)
  assert.equal(queryResultSeriesCount({ series_count: 2 }, 0), 2)
})

test('queryAdminOpPayload only when present', () => {
  assert.equal(queryAdminOpPayload({}), null)
  assert.deepEqual(queryAdminOpPayload({ admin_op_busy: true, op: 'flush' }), {
    admin_op_busy: true,
    op: 'flush',
    started_at_unix: undefined,
    last: undefined,
  })
})

test('streamEndPath/record_count prefer server', () => {
  assert.equal(streamEndPath({ path: '/api/v1/data/query/stream' }), '/api/v1/data/query/stream')
  assert.equal(streamEndPath({}, '/api/v1/data/query/stream'), '/api/v1/data/query/stream')
  assert.equal(streamEndRecordCount({ record_count: 7 }, 1), 7)
  assert.equal(streamEndRecordCount({}, 3), 3)
})

test('queryResultDatabase and measurement', () => {
  assert.equal(queryResultDatabase({ database: 'db1' }, 'x'), 'db1')
  assert.equal(queryResultDatabase({}, 'fallback'), 'fallback')
  assert.equal(queryResultMeasurement({ measurement: 'cpu' }, 'x'), 'cpu')
  assert.equal(queryResultMeasurement({}, 'm'), 'm')
})
