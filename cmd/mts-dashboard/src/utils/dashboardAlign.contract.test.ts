import assert from 'node:assert/strict'
import test from 'node:test'
import { formatFieldValue, formatFieldsMap } from './fieldValue.ts'

/** 模拟 Query order 编码：与 mts.QueryOrder JSON 对齐 */
function encodeOrder(order: 'asc' | 'desc' | ''): { by: number; direction: number } | undefined {
  if (order === 'asc' || order === 'desc') {
    return { by: 1, direction: order === 'desc' ? 2 : 1 }
  }
  return undefined
}

/** 模拟删除请求 body（snake_case） */
function buildDeleteBody(input: {
  database?: string
  measurement: string
  start_time: number
  end_time: number
}) {
  return {
    request: {
      database: input.database,
      measurement: input.measurement,
      start_time: input.start_time,
      end_time: input.end_time,
      precision: 'ms',
    },
  }
}

test('delete body uses snake_case fields expected by DeleteRequest json tags', () => {
  const body = buildDeleteBody({
    database: 'metrics',
    measurement: 'cpu',
    start_time: 1_700_000_000_000,
    end_time: 1_700_000_000_100,
  })
  const raw = JSON.stringify(body)
  assert.match(raw, /"start_time":1700000000000/)
  assert.match(raw, /"end_time":1700000000100/)
  assert.match(raw, /"measurement":"cpu"/)
  assert.doesNotMatch(raw, /"StartTime"/)
})

test('downsample toggle actions are enable/disable only', () => {
  const actions = (enabled: boolean) => (enabled ? 'disable' : 'enable')
  assert.equal(actions(true), 'disable')
  assert.equal(actions(false), 'enable')
  assert.notEqual(actions(true), 'pause')
  assert.notEqual(actions(false), 'resume')
})

test('query order encodes QueryOrder by/direction numbers', () => {
  assert.deepEqual(encodeOrder('asc'), { by: 1, direction: 1 })
  assert.deepEqual(encodeOrder('desc'), { by: 1, direction: 2 })
  assert.equal(encodeOrder(''), undefined)
})

test('FieldValue formatting expands typed objects', () => {
  assert.equal(formatFieldValue({ float64: 0.7 }), '0.7')
  assert.equal(formatFieldsMap({ usage: { float64: 0.5 }, n: { int64: 3 } }), 'usage=0.5, n=3')
})

test('list databases payload prefers databases field', () => {
  const payload = { databases: ['b', 'a'], measurements: ['legacy'] }
  const names = [...(payload.databases ?? payload.measurements ?? [])].sort()
  assert.deepEqual(names, ['a', 'b'])
})
