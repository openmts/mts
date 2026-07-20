import assert from 'node:assert/strict'
import test from 'node:test'
import { makeFormErrorT } from './formErrors.ts'
import { buildQueryFromForm, parseAggregates, parsePredicates, parseTags, QueryPredicateKind } from './queryFormBuild.ts'

const t = makeFormErrorT({
  queryErrAggFormat: 'agg-format:{value}',
  queryErrAggEmpty: 'agg-empty:{value}',
  queryErrTagFormat: 'tag-format:{value}',
  queryErrTagKeyEmpty: 'tag-key:{value}',
  queryErrStartTime: 'start',
  queryErrEndTime: 'end',
  queryErrOffset: 'offset',
  queryErrLimit: 'limit',
  queryErrPredFormat: 'pred-format:{value}',
  queryErrPredKind: 'pred-kind:{value}',
  queryErrPredName: 'pred-name:{value}',
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

test('parsePredicates tag and field ops', () => {
  const preds = parsePredicates('tag:host=a, field_gt:usage=0.5, usage>=1, tag_in:zone=z1|z2', t)
  assert.equal(preds.length, 4)
  assert.equal(preds[0].kind, QueryPredicateKind.TagEq)
  assert.deepEqual(preds[0].string_values, ['a'])
  assert.equal(preds[1].kind, QueryPredicateKind.FieldGT)
  assert.equal(preds[1].value?.type, 1)
  assert.equal(preds[1].value?.float64, 0.5)
  assert.equal(preds[2].kind, QueryPredicateKind.FieldGTE)
  assert.equal(preds[3].kind, QueryPredicateKind.TagIn)
  assert.deepEqual(preds[3].string_values, ['z1', 'z2'])
})

test('buildQueryFromForm includes predicates', () => {
  const q = buildQueryFromForm({ ...base, predicates: 'tag:host=e2e\nusage>0.1' }, t)
  const preds = q.predicates as Array<{ kind: number; name: string }>
  assert.equal(preds.length, 2)
  assert.equal(preds[0].name, 'host')
  assert.equal(preds[1].name, 'usage')
})

test('parsePredicates rejects bad kind', () => {
  assert.throws(() => parsePredicates('nope:x=1', t), /pred-kind:nope/)
})
