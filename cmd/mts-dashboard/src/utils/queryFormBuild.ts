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
  /** 谓词 DSL：每行或逗号分隔，见 parsePredicates */
  predicates?: string
}

/** 与 mts.QueryPredicateKind 数值对齐（iota+1） */
export const QueryPredicateKind = {
  TimeRange: 1,
  TagEq: 2,
  TagNe: 3,
  TagExists: 4,
  TagIn: 5,
  FieldEq: 6,
  FieldNe: 7,
  FieldGT: 8,
  FieldGTE: 9,
  FieldLT: 10,
  FieldLTE: 11,
} as const

const FIELD_TYPE = { float64: 1, int64: 2, string: 3, bool: 4 } as const

export type ParsedQueryPredicate = {
  kind: number
  name: string
  string_values?: string[]
  value?: {
    type: number
    float64?: number
    int64?: number
    string?: string
    bool?: boolean
  }
  start?: number
  end?: number
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

function parsePredicateValue(raw: string): ParsedQueryPredicate['value'] {
  const s = raw.trim()
  if (s === 'true' || s === 'false') {
    return { type: FIELD_TYPE.bool, bool: s === 'true' }
  }
  if (/^-?\d+$/.test(s)) {
    const n = Number(s)
    if (Number.isSafeInteger(n)) return { type: FIELD_TYPE.int64, int64: n }
  }
  if (/^-?\d+(\.\d+)?([eE][+-]?\d+)?$/.test(s)) {
    const n = Number(s)
    if (Number.isFinite(n)) return { type: FIELD_TYPE.float64, float64: n }
  }
  return { type: FIELD_TYPE.string, string: s }
}

const KIND_ALIASES: Record<string, number> = {
  tag: QueryPredicateKind.TagEq,
  tag_eq: QueryPredicateKind.TagEq,
  tageq: QueryPredicateKind.TagEq,
  tag_ne: QueryPredicateKind.TagNe,
  tagne: QueryPredicateKind.TagNe,
  tag_exists: QueryPredicateKind.TagExists,
  tagexists: QueryPredicateKind.TagExists,
  tag_in: QueryPredicateKind.TagIn,
  tagin: QueryPredicateKind.TagIn,
  field_eq: QueryPredicateKind.FieldEq,
  fieldeq: QueryPredicateKind.FieldEq,
  field_ne: QueryPredicateKind.FieldNe,
  fieldne: QueryPredicateKind.FieldNe,
  field_gt: QueryPredicateKind.FieldGT,
  fieldgt: QueryPredicateKind.FieldGT,
  field_gte: QueryPredicateKind.FieldGTE,
  fieldgte: QueryPredicateKind.FieldGTE,
  field_lt: QueryPredicateKind.FieldLT,
  fieldlt: QueryPredicateKind.FieldLT,
  field_lte: QueryPredicateKind.FieldLTE,
  fieldlte: QueryPredicateKind.FieldLTE,
  eq: QueryPredicateKind.FieldEq,
  ne: QueryPredicateKind.FieldNe,
  gt: QueryPredicateKind.FieldGT,
  gte: QueryPredicateKind.FieldGTE,
  lt: QueryPredicateKind.FieldLT,
  lte: QueryPredicateKind.FieldLTE,
}

/**
 * 解析谓词 DSL。
 * 支持：
 * - tag:host=a / tag_ne:env=prod / tag_exists:region / tag_in:host=a|b
 * - field_gt:usage=0.5 / gt:usage=0.5 / usage>0.5 / usage>=1 / usage==x / usage!=y
 * 多条用换行、分号或逗号分隔。
 */
export function parsePredicates(text: string, t: FormErrorT): ParsedQueryPredicate[] {
  const parts: string[] = []
  for (const line of text.split(/\n|;/)) {
    for (const piece of line.split(',')) {
      const s = piece.trim()
      if (s) parts.push(s)
    }
  }
  const out: ParsedQueryPredicate[] = []
  for (const part of parts) {
    const cmp = part.match(/^([A-Za-z_][\w.-]*)\s*(>=|<=|==|!=|>|<)\s*(.+)$/)
    if (cmp) {
      const name = cmp[1]
      const op = cmp[2]
      const val = cmp[3].trim()
      const kind =
        op === '>='
          ? QueryPredicateKind.FieldGTE
          : op === '<='
            ? QueryPredicateKind.FieldLTE
            : op === '>'
              ? QueryPredicateKind.FieldGT
              : op === '<'
                ? QueryPredicateKind.FieldLT
                : op === '!='
                  ? QueryPredicateKind.FieldNe
                  : QueryPredicateKind.FieldEq
      out.push({ kind, name, value: parsePredicateValue(val) })
      continue
    }
    const colon = part.indexOf(':')
    if (colon <= 0) throw new Error(t('queryErrPredFormat', { value: part }))
    const head = part.slice(0, colon).trim().toLowerCase()
    const body = part.slice(colon + 1).trim()
    const kind = KIND_ALIASES[head]
    if (!kind) throw new Error(t('queryErrPredKind', { value: head }))
    if (kind === QueryPredicateKind.TagExists) {
      if (!body) throw new Error(t('queryErrPredName', { value: part }))
      out.push({ kind, name: body })
      continue
    }
    if (kind === QueryPredicateKind.TagIn) {
      const eq = body.indexOf('=')
      if (eq <= 0) throw new Error(t('queryErrPredFormat', { value: part }))
      const name = body.slice(0, eq).trim()
      const vals = body
        .slice(eq + 1)
        .split('|')
        .map((x) => x.trim())
        .filter(Boolean)
      if (!name || !vals.length) throw new Error(t('queryErrPredFormat', { value: part }))
      out.push({ kind, name, string_values: vals })
      continue
    }
    if (kind === QueryPredicateKind.TagEq || kind === QueryPredicateKind.TagNe) {
      const eq = body.indexOf('=')
      if (eq <= 0) throw new Error(t('queryErrPredFormat', { value: part }))
      const name = body.slice(0, eq).trim()
      const v = body.slice(eq + 1).trim()
      if (!name) throw new Error(t('queryErrPredName', { value: part }))
      out.push({ kind, name, string_values: [v] })
      continue
    }
    const eq = body.indexOf('=')
    if (eq <= 0) throw new Error(t('queryErrPredFormat', { value: part }))
    const name = body.slice(0, eq).trim()
    const v = body.slice(eq + 1).trim()
    if (!name) throw new Error(t('queryErrPredName', { value: part }))
    out.push({ kind, name, value: parsePredicateValue(v) })
  }
  return out
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
    query.fields = form.fields
      .split(',')
      .map((s) => s.trim())
      .filter(Boolean)
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
      tags: form.group_tags
        .split(',')
        .map((s) => s.trim())
        .filter(Boolean),
    }
  }
  if (form.predicates && form.predicates.trim()) {
    query.predicates = parsePredicates(form.predicates, t)
  }
  return query
}
