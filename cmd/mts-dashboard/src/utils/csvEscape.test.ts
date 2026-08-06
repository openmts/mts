import assert from 'node:assert/strict'
import test from 'node:test'
import { escapeCSVCell } from './csvEscape.ts'

test('escapeCSVCell neutralizes spreadsheet formulas and leading controls', () => {
  for (const value of [
    '=1+1',
    '+cmd',
    '-2+3',
    '@SUM(A1:A2)',
    String.fromCharCode(9) + '=1+1',
    String.fromCharCode(13) + '@cmd',
    String.fromCharCode(0) + 'safe',
  ]) {
    const escaped = escapeCSVCell(value)
    assert.ok(
      escaped.startsWith("'") || escaped.startsWith("\"'"),
      JSON.stringify(value) + ' was not neutralized: ' + escaped,
    )
  }
})

test('escapeCSVCell preserves CSV quoting after neutralization', () => {
  assert.equal(escapeCSVCell('=1,2'), '"\'=1,2"')
  assert.equal(escapeCSVCell('say "hi"'), '"say ""hi"""')
  assert.equal(escapeCSVCell('plain'), 'plain')
})
