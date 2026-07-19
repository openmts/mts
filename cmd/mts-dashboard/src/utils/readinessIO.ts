/** 就绪中心状态导入/导出（versioned JSON，便于交接与审计） */

import { downloadJSON, downloadText } from './download.ts'
import {
  emptyReadinessState,
  loadReadinessState,
  normalizeSignoffNotes,
  saveReadinessState,
  type ReadinessState,
} from './readinessState.ts'

export const READINESS_EXPORT_VERSION = 1 as const

export interface ReadinessExportPayload {
  version: typeof READINESS_EXPORT_VERSION
  kind: 'mts.readiness'
  exported_at: string
  state: ReadinessState
}

export function buildReadinessExport(
  state: ReadinessState,
  now = new Date().toISOString(),
): ReadinessExportPayload {
  return {
    version: READINESS_EXPORT_VERSION,
    kind: 'mts.readiness',
    exported_at: now,
    state: {
      production: { ...state.production },
      edgeHttps: { ...state.edgeHttps },
      backupSchedule: { ...state.backupSchedule },
      deployKit: { ...(state.deployKit ?? {}) },
      signoffNotes: normalizeSignoffNotes(state.signoffNotes),
      updatedAt: state.updatedAt,
    },
  }
}

export function parseReadinessImport(
  raw: unknown,
): { ok: true; state: ReadinessState } | { ok: false; error: string } {
  if (raw == null || typeof raw !== 'object') {
    return { ok: false, error: '无效的就绪状态文件' }
  }
  const o = raw as Record<string, unknown>
  // 兼容直接导出 state 本体
  if (isStateShape(o)) {
    return { ok: true, state: normalizeState(o) }
  }
  if (o.kind != null && o.kind !== 'mts.readiness') {
    return { ok: false, error: 'kind 不是 mts.readiness' }
  }
  if (o.version != null && o.version !== READINESS_EXPORT_VERSION && o.version !== 1) {
    return { ok: false, error: `不支持的 version: ${String(o.version)}` }
  }
  if (o.state == null || typeof o.state !== 'object') {
    return { ok: false, error: '缺少 state 字段' }
  }
  return { ok: true, state: normalizeState(o.state as Record<string, unknown>) }
}

function isStateShape(o: Record<string, unknown>): boolean {
  return (
    (o.production != null && typeof o.production === 'object') ||
    (o.edgeHttps != null && typeof o.edgeHttps === 'object') ||
    (o.backupSchedule != null && typeof o.backupSchedule === 'object') ||
    (o.deployKit != null && typeof o.deployKit === 'object')
  ) && o.state == null
}

function normalizeState(o: Record<string, unknown>): ReadinessState {
  return {
    production: boolMap(o.production),
    edgeHttps: boolMap(o.edgeHttps),
    backupSchedule: boolMap(o.backupSchedule),
    deployKit: boolMap(o.deployKit),
    signoffNotes: normalizeSignoffNotes(o.signoffNotes),
    updatedAt: typeof o.updatedAt === 'string' ? o.updatedAt : undefined,
  }
}

function boolMap(v: unknown): Record<string, boolean> {
  if (v == null || typeof v !== 'object') return {}
  const out: Record<string, boolean> = {}
  for (const [k, val] of Object.entries(v as Record<string, unknown>)) {
    if (val) out[k] = true
  }
  return out
}

/** merge=true：按 section 合并 true 标志；false：整表替换 */
export function applyReadinessImport(
  current: ReadinessState,
  incoming: ReadinessState,
  opts: { merge: boolean },
): ReadinessState {
  if (!opts.merge) {
    return {
      production: { ...incoming.production },
      edgeHttps: { ...incoming.edgeHttps },
      backupSchedule: { ...incoming.backupSchedule },
      deployKit: { ...(incoming.deployKit ?? {}) },
      signoffNotes: normalizeSignoffNotes(incoming.signoffNotes),
      updatedAt: incoming.updatedAt,
    }
  }
  return {
    production: { ...current.production, ...incoming.production },
    edgeHttps: { ...current.edgeHttps, ...incoming.edgeHttps },
    backupSchedule: { ...current.backupSchedule, ...incoming.backupSchedule },
    deployKit: { ...(current.deployKit ?? {}), ...(incoming.deployKit ?? {}) },
    signoffNotes: {
      ...normalizeSignoffNotes(current.signoffNotes),
      ...normalizeSignoffNotes(incoming.signoffNotes),
    },
    updatedAt: incoming.updatedAt ?? current.updatedAt,
  }
}


export function persistImportedReadiness(
  incoming: ReadinessState,
  opts: { merge: boolean },
  storage: Pick<Storage, 'getItem' | 'setItem'> | null = typeof localStorage !== 'undefined'
    ? localStorage
    : null,
): ReadinessState {
  const current = loadReadinessState(storage)
  const next = applyReadinessImport(current, incoming, opts)
  return saveReadinessState(next, storage)
}

export function emptyExport(): ReadinessExportPayload {
  return buildReadinessExport(emptyReadinessState())
}

export { downloadJSON, downloadText }
