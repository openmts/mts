import assert from 'node:assert/strict'
import test from 'node:test'
import { BACKUP_SCHEDULE_STEPS, backupScheduleProgress } from './backupSchedule.ts'

test('backup schedule has required orchestration steps', () => {
  const ids = BACKUP_SCHEDULE_STEPS.map((s) => s.id)
  for (const need of ['define-rpo-rto', 'local-data-snapshot', 'offbox-copy', 'cron-schedule', 'restore-drill-weekly']) {
    assert.ok(ids.includes(need), need)
  }
  assert.ok(BACKUP_SCHEDULE_STEPS.some((s) => s.example))
})

test('backupScheduleProgress counts required', () => {
  const p = backupScheduleProgress(['define-rpo-rto', 'local-data-snapshot'])
  assert.equal(p.requiredDone, 2)
  assert.ok(p.requiredTotal >= 4)
  assert.ok(p.ratio > 0 && p.ratio < 1)
})
