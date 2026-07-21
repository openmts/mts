import { computed, ref } from 'vue'
import { downloadJSON, downloadText } from '@/utils/download'
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
      const msg = e instanceof Error ? e.message : String(e)
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
      const msg = e instanceof Error ? e.message : String(e)
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
  }
}
