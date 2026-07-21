import { computed, ref } from 'vue'
import { downloadJSON, downloadText } from '@/utils/download'
import { formatCaughtError } from '@/utils/apiError'
import {
  beginExportJob,
  cancelExportJob,
  createExportJobState,
  exportProgressPercent,
  failExportJob,
  finishExportJob,
  isExportJobBusy,
  progressExportJob,
  type ExportJobState,
} from '@/utils/exportJob'

export type ExportBuildHelpers = {
  isCancelled: () => boolean
  progress: (done: number, total: number) => void
}

export type ExportBundleFile =
  | { kind: 'json'; filename: string; payload: unknown }
  | { kind: 'text'; filename: string; text: string; mime?: string }

export type ExportOutcome = 'done' | 'cancelled' | 'error'

/**
 * 页面级导出任务：协作式构建 + 统一进度/取消/失败重试。
 */

function readE2ESlowExportMs(): number {
  try {
    const w = window as unknown as { __MTS_E2E_SLOW_EXPORT_MS?: unknown }
    const n = Number(w.__MTS_E2E_SLOW_EXPORT_MS)
    if (!Number.isFinite(n) || n <= 0) return 0
    return Math.min(Math.trunc(n), 10_000)
  } catch {
    return 0
  }
}

function shouldE2EFailExport(): boolean {
  try {
    const w = window as unknown as { __MTS_E2E_FAIL_EXPORT?: unknown }
    return w.__MTS_E2E_FAIL_EXPORT === true || w.__MTS_E2E_FAIL_EXPORT === 1
  } catch {
    return false
  }
}

async function maybeSlowExport(isCancelled: () => boolean): Promise<boolean> {
  const ms = readE2ESlowExportMs()
  if (ms <= 0) return false
  const step = Math.max(50, Math.floor(ms / 4))
  let waited = 0
  while (waited < ms) {
    if (isCancelled()) return true
    await new Promise((r) => setTimeout(r, step))
    waited += step
  }
  return isCancelled()
}

export function useExportJob() {
  const state = ref<ExportJobState>(createExportJobState())
  let token = 0
  let cancelled = false
  let lastRetry: (() => Promise<ExportOutcome>) | null = null

  const busy = computed(() => isExportJobBusy(state.value))
  const percent = computed(() => exportProgressPercent(state.value))
  const canRetry = computed(() => state.value.status === 'error' && !!lastRetry)

  function cancelExport() {
    if (!isExportJobBusy(state.value)) return
    cancelled = true
    state.value = cancelExportJob(state.value)
  }

  function resetExport() {
    cancelled = false
    state.value = createExportJobState()
  }

  async function retryLastExport(): Promise<ExportOutcome> {
    if (!lastRetry || isExportJobBusy(state.value)) return 'error'
    return lastRetry()
  }

  async function runGuarded(
    label: string,
    total: number | undefined,
    work: (h: ExportBuildHelpers, my: number) => Promise<ExportOutcome>,
  ): Promise<ExportOutcome> {
    const my = ++token
    cancelled = false
    state.value = beginExportJob(label, total ?? 0)
    try {
      if (await maybeSlowExport(() => cancelled || my !== token)) {
        state.value = cancelExportJob(state.value)
        return 'cancelled'
      }
      if (shouldE2EFailExport()) {
        throw new Error('e2e export fail')
      }
      return await work(
        {
          isCancelled: () => cancelled || my !== token,
          progress: (done, tot) => {
            if (my !== token) return
            state.value = progressExportJob(state.value, done, tot)
          },
        },
        my,
      )
    } catch (e) {
      if (my !== token) return 'cancelled'
      const msg = formatCaughtError(e)
      state.value = failExportJob(state.value, msg)
      return 'error'
    }
  }

  async function runTextExport(input: {
    label: string
    filename: string
    mime?: string
    total?: number
    build: (h: ExportBuildHelpers) => Promise<string | null>
  }): Promise<ExportOutcome> {
    lastRetry = () => runTextExport(input)
    return runGuarded(input.label, input.total, async (h, my) => {
      const text = await input.build(h)
      if (my !== token) return 'cancelled'
      if (text == null || cancelled) {
        state.value = cancelExportJob(state.value)
        return 'cancelled'
      }
      downloadText(input.filename, text, input.mime)
      state.value = finishExportJob(state.value)
      return 'done'
    })
  }

  async function runJSONExport(input: {
    label: string
    filename: string
    total?: number
    build: (h: ExportBuildHelpers) => Promise<unknown | null>
  }): Promise<ExportOutcome> {
    lastRetry = () => runJSONExport(input)
    return runGuarded(input.label, input.total, async (h, my) => {
      const payload = await input.build(h)
      if (my !== token) return 'cancelled'
      if (payload == null || cancelled) {
        state.value = cancelExportJob(state.value)
        return 'cancelled'
      }
      downloadJSON(input.filename, payload)
      state.value = finishExportJob(state.value)
      return 'done'
    })
  }

  async function runBundleExport(input: {
    label: string
    total?: number
    build: (h: ExportBuildHelpers) => Promise<ExportBundleFile[] | null>
  }): Promise<ExportOutcome> {
    lastRetry = () => runBundleExport(input)
    return runGuarded(input.label, input.total, async (h, my) => {
      const files = await input.build(h)
      if (my !== token) return 'cancelled'
      if (files == null || cancelled) {
        state.value = cancelExportJob(state.value)
        return 'cancelled'
      }
      const total = files.length
      if (total > 0) state.value = progressExportJob(state.value, 0, total)
      for (let i = 0; i < files.length; i++) {
        if (cancelled || my !== token) {
          state.value = cancelExportJob(state.value)
          return 'cancelled'
        }
        const f = files[i]
        if (f.kind === 'json') downloadJSON(f.filename, f.payload)
        else downloadText(f.filename, f.text, f.mime)
        state.value = progressExportJob(state.value, i + 1, total)
        if (i + 1 < total) await new Promise((r) => setTimeout(r, 0))
      }
      if (my !== token || cancelled) {
        state.value = cancelExportJob(state.value)
        return 'cancelled'
      }
      state.value = finishExportJob(state.value)
      return 'done'
    })
  }

  return {
    exportJob: state,
    exportBusy: busy,
    exportPercent: percent,
    canRetryExport: canRetry,
    cancelExport,
    resetExport,
    retryLastExport,
    runTextExport,
    runJSONExport,
    runBundleExport,
  }
}
