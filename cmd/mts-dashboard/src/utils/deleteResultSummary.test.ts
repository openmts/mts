import assert from 'node:assert/strict'
import test from 'node:test'
import {
  acceptedDeletePath,
  formatDeleteSuccessMessage,
} from './deleteResultSummary.ts'

test('acceptedDeletePath prefers server path', () => {
  assert.equal(acceptedDeletePath({ path: '/api/v1/data/delete' }, '/x'), '/api/v1/data/delete')
  assert.equal(acceptedDeletePath({}, '/api/v1/data/delete'), '/api/v1/data/delete')
})

test('formatDeleteSuccessMessage uses path/measurement/database', () => {
  const msg = formatDeleteSuccessMessage({
    server: { path: '/api/v1/data/delete', measurement: 'cpu', database: 'metrics' },
    template: 'deleted {measurement}@{database} via {path}',
    format: (tpl, vars) =>
      tpl
        .replace('{measurement}', String(vars.measurement))
        .replace('{database}', String(vars.database))
        .replace('{path}', String(vars.path)),
  })
  assert.equal(msg, 'deleted cpu@metrics via /api/v1/data/delete')
})
