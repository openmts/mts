import test from 'node:test'
import assert from 'node:assert/strict'
import {
  anyRetentionPolicyDraftDirty,
  isDatabaseCreateDraftDirty,
  isDownsampleCreateDraftDirty,
  isPasswordDraftDirty,
  isRetentionPolicyDraftDirty,
  isUserCreateDraftDirty,
  shouldBlockLeaveAdminCreate,
} from './adminFormDirty.ts'

test('isUserCreateDraftDirty', () => {
  assert.equal(isUserCreateDraftDirty({ name: '', display_name: '', password: '', role: 'user' }), false)
  assert.equal(isUserCreateDraftDirty({ name: 'a' }), true)
  assert.equal(isUserCreateDraftDirty({ role: 'admin' }), true)
  assert.equal(isUserCreateDraftDirty({ password: 'x' }), true)
})

test('isPasswordDraftDirty', () => {
  assert.equal(isPasswordDraftDirty(''), false)
  assert.equal(isPasswordDraftDirty('secret'), true)
  assert.equal(isPasswordDraftDirty('', 'old'), true)
})

test('isDownsampleCreateDraftDirty', () => {
  assert.equal(
    isDownsampleCreateDraftDirty({
      name: '',
      source_database: '',
      interval_human: '1m',
      enabled: true,
      functions_json: JSON.stringify([{ function: 'mean', field: 'value', as: 'mean_value' }]),
    }),
    false,
  )
  assert.equal(isDownsampleCreateDraftDirty({ name: 'p1' }), true)
  assert.equal(isDownsampleCreateDraftDirty({ source_database: 'db' }), true)
  assert.equal(isDownsampleCreateDraftDirty({ interval_human: '5m' }), true)
})

test('shouldBlockLeaveAdminCreate', () => {
  assert.equal(shouldBlockLeaveAdminCreate(true, true), true)
  assert.equal(shouldBlockLeaveAdminCreate(true, false), false)
  assert.equal(shouldBlockLeaveAdminCreate(false, true), false)
})

test('database create and RP drafts', () => {
  assert.equal(isDatabaseCreateDraftDirty({ name: '' }), false)
  assert.equal(isDatabaseCreateDraftDirty({ name: 'metrics' }), true)
  assert.equal(isRetentionPolicyDraftDirty({ name: '', duration: '' }), false)
  assert.equal(isRetentionPolicyDraftDirty({ name: 'rp', duration: '' }), true)
  assert.equal(isRetentionPolicyDraftDirty({ name: '', duration: '7d' }), true)
  assert.equal(anyRetentionPolicyDraftDirty([{ name: '' }, { name: 'x', duration: '1d' }]), true)
  assert.equal(anyRetentionPolicyDraftDirty([{ name: '', duration: '' }]), false)
})
