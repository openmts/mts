import assert from 'node:assert/strict'
import test from 'node:test'
import {
  SIGNOFF_FIELD_GUIDES,
  applySignoffExample,
  localizedSignoffGuideSteps,
  signoffGuideExample,
  signoffGuideFor,
  signoffGuideSummary,
} from './signoffGuide.ts'

test('guides cover all three signoff fields', () => {
  assert.equal(SIGNOFF_FIELD_GUIDES.length, 3)
  assert.deepEqual(
    SIGNOFF_FIELD_GUIDES.map((g) => g.field).sort(),
    ['backupAlert', 'backupOffsite', 'edgeHttps'],
  )
})

test('signoffGuideFor returns steps and bilingual example', () => {
  const g = signoffGuideFor('edgeHttps')
  assert.ok(g.steps.length >= 3)
  assert.match(signoffGuideSummary('edgeHttps', 'zh'), /TLS|HSTS|验收/)
  assert.match(signoffGuideExample('backupAlert', 'en'), /PagerDuty|alert|Drill/i)
  const steps = localizedSignoffGuideSteps('backupOffsite', 'zh')
  assert.ok(steps.every((s) => s.title && s.detail))
})

test('applySignoffExample does not overwrite unless force', () => {
  assert.equal(applySignoffExample('already', 'edgeHttps', 'zh'), null)
  assert.match(applySignoffExample('', 'edgeHttps', 'zh') || '', /验收人|CHG/)
  assert.match(applySignoffExample('x', 'edgeHttps', 'en', { force: true }) || '', /Acceptor|CHG/)
})
