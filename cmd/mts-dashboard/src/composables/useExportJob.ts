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

/**
 * 页面级导出任务：协作式构建 + 统一进度/取消状态。
 * 同步小导出也可走同一路径，便于 UI 一致。
 */
export function useExportJob() {
  const state = ref<ExportJobState>(createExportJobState())
  let token = 0
  let cancelled = false

  const busy = computed(() => isExportJobBusy(state.value))
  const percent = computed(() => exportProgressPercent(state.value))

  function cancelExport() {
    if (!isExportJobBusy(state.value)) return
    cancelled = true
    state.value = cancelExportJob(state.value)
  }

  function resetExport() {
    cancelled = false
    state.value = createExportJobState()
  }

  async function runTextExport(input: {
    label: string
    filename: string
    mime?: string
    total?: number
    build: (h: ExportBuildHelpers) => Promise<string | null>
  }): Promise<'done' | 'cancelled' | 'error'> {
    const my = ++token
    cancelled = false
    state.value = beginExportJob(input.label, input.total ?? 0)
    try {
      const text = await input.build({
        isCancelled: () => cancelled || my !== token,
        progress: (done, total) => {
          if (my !== token) return
          state.value = progressExportJob(state.value, done, total)
        },
      })
      if (my !== token) return 'cancelled'
      if (text == null || cancelled) {
        state.value = cancelExportJob(state.value)
        return 'cancelled'
      }
      downloadText(input.filename, text, input.mime)
      state.value = finishExportJob(state.value)
      return 'done'
    } catch (e) {
      if (my !== token) return 'cancelled'
      const msg = formatCaughtError(e)
      state.value = failExportJob(state.value, msg)
      return 'error'
    }
  }

  async function runJSONExport(input: {
    label: string
    filename: string
    total?: number
    build: (h: ExportBuildHelpers) => Promise<unknown | null>
  }): Promise<'done' | 'cancelled' | 'error'> {
    const my = ++token
    cancelled = false
    state.value = beginExportJob(input.label, input.total ?? 0)
    try {
      const payload = await input.build({
        isCancelled: () => cancelled || my !== token,
        progress: (done, total) => {
          if (my !== token) return
          state.value = progressExportJob(state.value, done, total)
        },
      })
      if (my !== token) return 'cancelled'
      if (payload == null || cancelled) {
        state.value = cancelExportJob(state.value)
        return 'cancelled'
      }
      downloadJSON(input.filename, payload)
      state.value = finishExportJob(state.value)
      return 'done'
    } catch (e) {
      if (my !== token) return 'cancelled'
      const msg = formatCaughtError(e)
      state.value = failExportJob(state.value, msg)
      return 'error'
    }
  }

  /** 多文件导出：构建后按序触发下载（每步可取消） */
  async function runBundleExport(input: {
    label: string
    total?: number
    build: (h: ExportBuildHelpers) => Promise<ExportBundleFile[] | null>
  }): Promise<'done' | 'cancelled' | 'error'> {
    const my = ++token
    cancelled = false
    state.value = beginExportJob(input.label, input.total ?? 0)
    try {
      const files = await input.build({
        isCancelled: () => cancelled || my !== token,
        progress: (done, total) => {
          if (my !== token) return
          state.value = progressExportJob(state.value, done, total)
        },
      })
      if (my !== token) return 'cancelled'
      if (files == null || cancelled) {
        state.value = cancelExportJob(state.value)
        return 'cancelled'
      }
      const total = files.length
      if (total > 0) {
        state.value = progressExportJob(state.value, 0, total)
      }
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
    } catch (e) {
      if (my !== token) return 'cancelled'
      const msg = formatCaughtError(e)
      state.value = failExportJob(state.value, msg)
      return 'error'
    }
  }

  return {
    exportJob: state,
    exportBusy: busy,
    exportPercent: percent,
    cancelExport,
    resetExport,
    runTextExport,
    runJSONExport,
    runBundleExport,
  }
}
