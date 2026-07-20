import test from 'node:test'
import assert from 'node:assert/strict'
import {
  buildShortcutsPrefillPath,
  parseShortcutsPrefill,
  pathOpensShortcutsHelp,
} from './shortcutsPrefill.ts'

test('parseShortcutsPrefill', () => {
  assert.deepEqual(parseShortcutsPrefill({ shortcuts: '1' }), { open: true })
  assert.deepEqual(parseShortcutsPrefill({}, '#shortcuts-help'), { open: true })
  assert.deepEqual(parseShortcutsPrefill({}), {})
})

test('buildShortcutsPrefillPath', () => {
  assert.equal(buildShortcutsPrefillPath({ path: '/query' }), '/query?shortcuts=1#shortcuts-help')
  assert.equal(buildShortcutsPrefillPath(), '/?shortcuts=1#shortcuts-help')
})

test('pathOpensShortcutsHelp', () => {
  assert.equal(pathOpensShortcutsHelp('/?shortcuts=1#shortcuts-help'), true)
  assert.equal(pathOpensShortcutsHelp('/query?shortcuts=true'), true)
  assert.equal(pathOpensShortcutsHelp('/#shortcuts-help'), true)
  assert.equal(pathOpensShortcutsHelp('/query'), false)
  assert.equal(pathOpensShortcutsHelp('action:open-shortcuts'), false)
})
