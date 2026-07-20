import assert from 'node:assert/strict'
import test from 'node:test'
import {
  defaultSelectedSnapshotPath,
  selectableDataSnapshots,
  snapshotLabel,
} from './storageSnapshots.ts'

const items = [
  { name: 'restore-drill-1', kind: 'restore-drill', path: '/b/r1' },
  { name: 'data-snapshot-2', kind: 'data-snapshot', path: '/b/d2' },
  { name: 'data-snapshot-1', kind: 'data-snapshot', path: '/b/d1' },
]

test('selectableDataSnapshots keeps only data-snapshot', () => {
  assert.deepEqual(
    selectableDataSnapshots(items).map((s) => s.name),
    ['data-snapshot-2', 'data-snapshot-1'],
  )
})

test('defaultSelectedSnapshotPath prefers first selectable or preferred', () => {
  assert.equal(defaultSelectedSnapshotPath(items), '/b/d2')
  assert.equal(defaultSelectedSnapshotPath(items, '/b/d1'), '/b/d1')
  assert.equal(defaultSelectedSnapshotPath([]), '')
})

test('snapshotLabel', () => {
  assert.match(snapshotLabel({ name: 'a', size_bytes: 12 }), /a \(12B\)/)
})
