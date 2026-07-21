/** 导出任务进度 / 协作式取消（纯函数） */

export type ExportJobStatus = 'idle' | 'running' | 'done' | 'cancelled' | 'error'

export type ExportJobState = {
  status: ExportJobStatus
  label: string
  done: number
  total: number
  error?: string
}

export function createExportJobState(label = ''): ExportJobState {
  return { status: 'idle', label, done: 0, total: 0 }
}

export function isExportJobBusy(state: ExportJobState | null | undefined): boolean {
  return state?.status === 'running'
}

export function exportProgressPercent(state: ExportJobState | null | undefined): number {
  if (!state || state.total <= 0) {
    return state?.status === 'done' ? 100 : 0
  }
  const p = Math.floor((state.done / state.total) * 100)
  if (p < 0) return 0
  if (p > 100) return 100
  return p
}

export function beginExportJob(label: string, total = 0): ExportJobState {
  return {
    status: 'running',
    label: label || '',
    done: 0,
    total: total > 0 ? total : 0,
  }
}

export function progressExportJob(
  state: ExportJobState,
  done: number,
  total?: number,
): ExportJobState {
  const nextTotal = total != null && total > 0 ? total : state.total
  const nextDone = done < 0 ? 0 : nextTotal > 0 && done > nextTotal ? nextTotal : done
  return {
    ...state,
    status: 'running',
    done: nextDone,
    total: nextTotal,
  }
}

export function finishExportJob(state: ExportJobState): ExportJobState {
  return {
    ...state,
    status: 'done',
    done: state.total > 0 ? state.total : state.done,
  }
}

export function cancelExportJob(state: ExportJobState): ExportJobState {
  return { ...state, status: 'cancelled' }
}

export function failExportJob(state: ExportJobState, error: string): ExportJobState {
  return { ...state, status: 'error', error: error || 'error' }
}

export function resetExportJob(label = ''): ExportJobState {
  return createExportJobState(label)
}

export type MapExportProgressOpts = {
  cancelled: () => boolean
  onProgress?: (done: number, total: number) => void
  /** 每处理多少条让出事件循环，默认 500 */
  chunkSize?: number
}

/**
 * 协作式分块 map：可在大批量序列化前取消。
 * 返回 null 表示已取消。
 */
export async function mapWithExportProgress<T, R>(
  items: readonly T[],
  mapFn: (item: T, index: number) => R,
  opts: MapExportProgressOpts,
): Promise<R[] | null> {
  const total = items.length
  const chunk = opts.chunkSize && opts.chunkSize > 0 ? opts.chunkSize : 500
  const out: R[] = []
  opts.onProgress?.(0, total)
  if (opts.cancelled()) return null
  for (let i = 0; i < total; i++) {
    if (opts.cancelled()) return null
    out.push(mapFn(items[i], i))
    const done = i + 1
    if (done === total || done % chunk === 0) {
      opts.onProgress?.(done, total)
      if (done < total) {
        await yieldExportTick()
        if (opts.cancelled()) return null
      }
    }
  }
  return out
}

/** 将多段文本协作式拼接（header + lines） */
export async function joinTextWithExportProgress(
  header: string,
  lines: readonly string[],
  opts: MapExportProgressOpts,
): Promise<string | null> {
  const total = lines.length
  const chunk = opts.chunkSize && opts.chunkSize > 0 ? opts.chunkSize : 500
  opts.onProgress?.(0, total)
  if (opts.cancelled()) return null
  let body = ''
  for (let i = 0; i < total; i++) {
    if (opts.cancelled()) return null
    body += (i === 0 && !header ? '' : '') + lines[i] + (i + 1 < total ? '\n' : '')
    const done = i + 1
    if (done === total || done % chunk === 0) {
      opts.onProgress?.(done, total)
      if (done < total) {
        await yieldExportTick()
        if (opts.cancelled()) return null
      }
    }
  }
  if (!header) return body
  if (!body) return header
  return `${header}\n${body}`
}

function yieldExportTick(): Promise<void> {
  return new Promise((resolve) => {
    if (typeof setTimeout === 'function') setTimeout(resolve, 0)
    else resolve()
  })
}
