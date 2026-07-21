/** 批量写 NDJSON 进度事件（对齐 mts-server batchProgressEvent） */

export type BatchItemStatus = 'ok' | 'skip' | 'error' | string

export interface BatchProgressState {
  done: number
  total: number
  ok: number
  skip: number
  fail: number
  lastName: string
  lastStatus: string
}

export interface BatchMutationSummary {
  ok: boolean
  ok_count: number
  skip_count: number
  fail_count: number
  items: Array<{ name: string; status: string; message?: string }>
}

export function emptyBatchProgress(): BatchProgressState {
  return { done: 0, total: 0, ok: 0, skip: 0, fail: 0, lastName: '', lastStatus: '' }
}

export function applyBatchProgressEvent(
  prev: BatchProgressState,
  record: unknown,
): { next: BatchProgressState; summary: BatchMutationSummary | null; error: string | null } {
  if (!record || typeof record !== 'object') {
    return { next: prev, summary: null, error: null }
  }
  const r = record as Record<string, unknown>
  const type = String(r.type || '')
  if (type === 'error') {
    return { next: prev, summary: null, error: String(r.message || 'batch stream error') }
  }
  if (type === 'item') {
    const status = String(r.status || '')
    const next: BatchProgressState = {
      done: Number(r.index) || prev.done,
      total: Number(r.total) || prev.total,
      ok: prev.ok + (status === 'ok' ? 1 : 0),
      skip: prev.skip + (status === 'skip' ? 1 : 0),
      fail: prev.fail + (status === 'error' ? 1 : 0),
      lastName: String(r.name || ''),
      lastStatus: status,
    }
    return { next, summary: null, error: null }
  }
  if (type === 'summary') {
    const items = Array.isArray(r.items)
      ? (r.items as Array<{ name: string; status: string; message?: string }>)
      : []
    const summary: BatchMutationSummary = {
      ok: Boolean(r.ok),
      ok_count: Number(r.ok_count) || 0,
      skip_count: Number(r.skip_count) || 0,
      fail_count: Number(r.fail_count) || 0,
      items,
    }
    const next: BatchProgressState = {
      done: Number(r.total) || summary.ok_count + summary.skip_count + summary.fail_count,
      total: Number(r.total) || summary.ok_count + summary.skip_count + summary.fail_count,
      ok: summary.ok_count,
      skip: summary.skip_count,
      fail: summary.fail_count,
      lastName: prev.lastName,
      lastStatus: prev.lastStatus,
    }
    return { next, summary, error: null }
  }
  return { next: prev, summary: null, error: null }
}

export function batchProgressPercent(state: BatchProgressState): number {
  if (!state.total || state.total <= 0) return 0
  return Math.min(100, Math.round((state.done / state.total) * 100))
}
