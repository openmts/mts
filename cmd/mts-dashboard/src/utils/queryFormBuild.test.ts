import assert from 'node:assert/strict'
import test from 'node:test'
import { makeFormErrorT } from './formErrors.ts'
import { buildQueryFromForm, parseAggregates, parseTags } from './queryFormBuild.ts'

const t = makeFormErrorT({
  queryErrAggFormat: 'agg-format:{value}',
  queryErrAggEmpty: 'agg-empty:{value}',
  queryErrTagFormat: 'tag-format:{value}',
  queryErrTagKeyEmpty: 'tag-key:{value}',
  queryErrStartTime: 'start',
  queryErrEndTime: 'end',
  queryErrOffset: 'offset',
  queryErrLimit: 'limit',
})

const base = {
  database: 'db',
  retention_policy: 'autogen',
  measurement: 'cpu',
  start_time: '',
  end_time: '',
  fields: 'usage',
  tags: '',
  order: 'desc',
  offset: '',
  limit: '10',
  aggregates: '',
  window: '',
  group_tags: '',
}

test('buildQueryFromForm maps order limit fields', () => {
  const q = buildQueryFromForm(base, t)
  assert.equal(q.database, 'db')
  assert.equal(q.measurement, 'cpu')
  assert.deepEqual(q.fields, ['usage'])
  assert.deepEqual(q.order, { by: 1, direction: 2 })
  assert.equal(q.limit, 10)
  assert.equal(q.precision, 'ms')
})

test('parseAggregates and tags', () => {
  assert.deepEqual(parseAggregates('mean:usage,sum:x', t), [
    { function: 'mean', field: 'usage' },
    { function: 'sum', field: 'x' },
  ])
  assert.deepEqual(parseTags('host=a,region=b', t), { host: 'a', region: 'b' })
})

test('validation errors localized via t', () => {
  assert.throws(() => parseAggregates('bad', t), /agg-format:bad/)
  assert.throws(() => parseTags('=v', t), /tag-format/)
  assert.throws(() => buildQueryFromForm({ ...base, start_time: '1.5' }, t), /start/)
  assert.throws(() => buildQueryFromForm({ ...base, offset: '-1' }, t), /offset/)
  assert.throws(() => buildQueryFromForm({ ...base, limit: '0' }, t), /limit/)
})
