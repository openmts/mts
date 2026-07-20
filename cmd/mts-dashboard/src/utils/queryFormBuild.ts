/** 查询表单 → API Query 纯函数（校验错误走注入 t） */

import { parseHumanDurationToNs } from './duration.ts'
import type { FormErrorT } from './formErrors.ts'

export interface QueryFormInput {
  database: string
  retention_policy: string
  measurement: string
  start_time: string
  end_time: string
  fields: string
  tags: string
  order: string
  offset: string
  limit: string
  aggregates: string
  window: string
  group_tags: string
}

function parseTimeInt(raw: string): number | null {
  const s = String(raw || '').trim()
  if (!s || !/^-?\d+$/.test(s)) return null
  const n = Number(s)
  if (!Number.isSafeInteger(n)) return null
  return n
}

export function parseAggregates(text: string, t: FormErrorT): { function: string; field: string }[] {
  const out: { function: string; field: string }[] = []
  for (const part of text.split(',')) {
    const s = part.trim()
    if (!s) continue
    const colon = s.indexOf(':')
    if (colon <= 0) throw new Error(t('queryErrAggFormat', { value: s }))
    const fn = s.slice(0, colon).trim().toLowerCase()
    const field = s.slice(colon + 1).trim()
    if (!fn || !field) throw new Error(t('queryErrAggEmpty', { value: s }))
    out.push({ function: fn, field })
  }
  return out
}

export function parseTags(text: string, t: FormErrorT): Record<string, string> {
  const tags: Record<string, string> = {}
  for (const part of text.split(',')) {
    const s = part.trim()
    if (!s) continue
    const eq = s.indexOf('=')
    if (eq <= 0) throw new Error(t('queryErrTagFormat', { value: s }))
    const k = s.slice(0, eq).trim()
    const v = s.slice(eq + 1).trim()
    if (!k) throw new Error(t('queryErrTagKeyEmpty', { value: s }))
    tags[k] = v
  }
  return tags
}

export function buildQueryFromForm(form: QueryFormInput, t: FormErrorT): Record<string, unknown> {
  const query: Record<string, unknown> = {
    precision: 'ms',
  }
  if (form.database) query.database = form.database
  if (form.retention_policy) query.retention_policy = form.retention_policy
  if (form.measurement) query.measurement = form.measurement
  if (form.start_time) {
    const v = parseTimeInt(form.start_time)
    if (v === null) throw new Error(t('queryErrStartTime'))
    query.start_time = v
  }
  if (form.end_time) {
    const v = parseTimeInt(form.end_time)
    if (v === null) throw new Error(t('queryErrEndTime'))
    query.end_time = v
  }
  if (form.fields) {
    query.fields = form.fields.split(',').map((s) => s.trim()).filter(Boolean)
  }
  if (form.tags.trim()) {
    query.tags = parseTags(form.tags, t)
  }
  if (form.order === 'asc' || form.order === 'desc') {
    query.order = {
      by: 1,
      direction: form.order === 'desc' ? 2 : 1,
    }
  }
  if (form.offset) {
    const off = parseTimeInt(form.offset)
    if (off === null || off < 0) throw new Error(t('queryErrOffset'))
    query.offset = off
  }
  if (form.limit) {
    const lim = parseTimeInt(form.limit)
    if (lim === null || lim <= 0) throw new Error(t('queryErrLimit'))
    query.limit = lim
  }
  if (form.aggregates.trim()) {
    query.aggregates = parseAggregates(form.aggregates, t)
  }
  if (form.window.trim()) {
    const ns = parseHumanDurationToNs(form.window)
    query.window = ns
    const groupTags = form.group_tags
      .split(',')
      .map((s) => s.trim())
      .filter(Boolean)
    query.group = { tags: groupTags, window: ns }
  } else if (form.group_tags.trim()) {
    query.group = {
      tags: form.group_tags.split(',').map((s) => s.trim()).filter(Boolean),
    }
  }
  return query
}
