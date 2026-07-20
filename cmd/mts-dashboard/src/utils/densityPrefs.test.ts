import assert from 'node:assert/strict'
import test from 'node:test'
import {
  applyUiDensity,
  loadUiDensity,
  normalizeUiDensity,
  saveUiDensity,
} from './densityPrefs.ts'

function mem() {
  const m = new Map<string, string>()
  return {
    getItem: (k: string) => m.get(k) ?? null,
    setItem: (k: string, v: string) => { m.set(k, v) },
    removeItem: (k: string) => { m.delete(k) },
  }
}

test('normalizeUiDensity', () => {
  assert.equal(normalizeUiDensity('compact'), 'compact')
  assert.equal(normalizeUiDensity('comfortable'), 'comfortable')
  assert.equal(normalizeUiDensity('x'), 'comfortable')
})

test('load/save density', () => {
  const s = mem()
  assert.equal(loadUiDensity(s), 'comfortable')
  saveUiDensity(s, 'compact')
  assert.equal(loadUiDensity(s), 'compact')
  saveUiDensity(s, 'comfortable')
  assert.equal(loadUiDensity(s), 'comfortable')
})

test('applyUiDensity sets attribute', () => {
  const attrs = new Map<string, string>()
  const root = {
    setAttribute: (n: string, v: string) => { attrs.set(n, v) },
    removeAttribute: (n: string) => { attrs.delete(n) },
  }
  applyUiDensity('compact', root)
  assert.equal(attrs.get('data-density'), 'compact')
  applyUiDensity('comfortable', root)
  assert.equal(attrs.has('data-density'), false)
})
