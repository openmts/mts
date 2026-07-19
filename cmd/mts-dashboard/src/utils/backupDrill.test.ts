import assert from 'node:assert/strict'
import test from 'node:test'
import {
  BACKUP_DRILL_STEPS,
  dashboardDrillSteps,
  drillProgress,
  requiredDrillSteps,
} from './backupDrill.ts'

test('backup drill has core required steps', () => {
  const ids = BACKUP_DRILL_STEPS.map((s) => s.id)
  for (const need of ['validate', 'snapshot', 'data-snapshot', 'copy-offbox', 'restore-side']) {
    assert.ok(ids.includes(need), need)
  }
  assert.ok(requiredDrillSteps().length >= 3)
  assert.ok(dashboardDrillSteps().length >= 2)
})

test('drillProgress counts required and overall', () => {
  const p = drillProgress(['validate', 'snapshot'])
  assert.equal(p.completed, 2)
  assert.ok(p.requiredCompleted >= 2)
  assert.ok(p.ratio > 0 && p.ratio < 1)
})
