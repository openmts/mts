import assert from 'node:assert/strict'
import test from 'node:test'
import {
  acceptedWritePath,
  acceptedWritePoints,
  formatWriteSuccessMessage,
} from './writeResultSummary.ts'

test('acceptedWritePoints prefers server', () => {
  assert.equal(acceptedWritePoints({ points: 3 }, 9), 3)
  assert.equal(acceptedWritePoints({}, 9), 9)
  assert.equal(acceptedWritePoints({ points: -1 }, 2), 2)
  assert.equal(acceptedWritePoints(null, 0), 0)
})

test('acceptedWritePath prefers server', () => {
  assert.equal(acceptedWritePath({ path: '/api/v1/data/write/typed' }, '/x'), '/api/v1/data/write/typed')
  assert.equal(acceptedWritePath({}, '/api/v1/data/write'), '/api/v1/data/write')
})

test('formatWriteSuccessMessage', () => {
  const format = (tpl: string, vars: Record<string, string | number>) =>
    tpl.replace(/\{(\w+)\}/g, (_, k) => String(vars[k] ?? ''))
  const typed = formatWriteSuccessMessage({
    mode: 'typed',
    server: { points: 5, path: '/api/v1/data/write/typed' },
    clientCount: 9,
    clientPath: '/typed',
    typedTemplate: 'TypedBatch 写入成功（{count} 点）',
    pointsTemplate: '写入成功（{count} 点，{path}）',
    format,
  })
  assert.equal(typed, 'TypedBatch 写入成功（5 点）')
  const pts = formatWriteSuccessMessage({
    mode: 'points',
    server: { points: 2, path: '/api/v1/data/write/points-typed' },
    clientCount: 4,
    clientPath: '/api/v1/data/write',
    typedTemplate: 'TypedBatch 写入成功（{count} 点）',
    pointsTemplate: '写入成功（{count} 点，{path}）',
    format,
  })
  assert.equal(pts, '写入成功（2 点，/api/v1/data/write/points-typed）')
})

test('formatWriteSuccessMessage typed with path template', () => {
  const fmt = (tpl: string, vars: Record<string, string | number>) =>
    tpl.replace(/\{(\w+)\}/g, (_, k) => String(vars[k] ?? ''))
  const typed = formatWriteSuccessMessage({
    mode: 'typed',
    server: { points: 3, path: '/api/v1/data/write/typed' },
    clientCount: 1,
    clientPath: '/fallback',
    typedTemplate: 'Typed OK {count}',
    typedWithPathTemplate: 'Typed OK {count} via {path}',
    pointsTemplate: 'Points OK {count} {path}',
    format: fmt,
  })
  assert.match(typed, /3/)
  assert.match(typed, /write\/typed/)
})

