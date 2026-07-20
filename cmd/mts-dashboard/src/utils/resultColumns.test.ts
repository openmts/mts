import assert from 'node:assert/strict'
import test from 'node:test'
import {
  DEFAULT_RESULT_COLUMNS,
  gridColClass,
  parseResultColumns,
  toggleResultColumn,
  visibleColumnCount,
  resultColumnLabel,
} from './resultColumns.ts'

test('parseResultColumns fills defaults and keeps one column', () => {
  assert.deepEqual(parseResultColumns(null), DEFAULT_RESULT_COLUMNS)
  assert.deepEqual(parseResultColumns({ time: false, measurement: false, tags: false, fields: false }).time, true)
  assert.equal(parseResultColumns({ tags: false }).tags, false)
})

test('toggleResultColumn refuses empty set', () => {
  const onlyTime = { time: true, measurement: false, tags: false, fields: false }
  assert.deepEqual(toggleResultColumn(onlyTime, 'time'), onlyTime)
  assert.equal(toggleResultColumn(DEFAULT_RESULT_COLUMNS, 'tags').tags, false)
})

test('gridColClass maps count', () => {
  assert.equal(gridColClass(DEFAULT_RESULT_COLUMNS), 'grid-cols-4')
  assert.equal(gridColClass({ time: true, measurement: false, tags: false, fields: false }), 'grid-cols-1')
  assert.equal(visibleColumnCount(DEFAULT_RESULT_COLUMNS), 4)
})

test('resultColumnLabel locale', () => {
  assert.equal(resultColumnLabel('time', 'zh'), '时间')
  assert.equal(resultColumnLabel('time', 'en'), 'Time')
  assert.equal(resultColumnLabel('measurement', 'en'), 'Measurement')
})
