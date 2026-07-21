<script setup lang="ts">
import { computed } from 'vue'
import type { ExportJobState } from '@/utils/exportJob'
import { exportProgressPercent, isExportJobBusy } from '@/utils/exportJob'
import { useI18n } from '@/composables/useI18n'
import { formatMessage } from '@/utils/formatMessage'
import { X } from 'lucide-vue-next'

const props = defineProps<{
  job: ExportJobState
}>()

const emit = defineEmits<{
  cancel: []
  dismiss: []
}>()

const { t } = useI18n()
const busy = computed(() => isExportJobBusy(props.job))
const percent = computed(() => exportProgressPercent(props.job))
const visible = computed(() => props.job.status !== 'idle')
const title = computed(() => {
  if (props.job.status === 'running') {
    return formatMessage(t.value('exportJobRunning'), {
      label: props.job.label || t.value('exportJobDefaultLabel'),
      percent: String(percent.value),
    })
  }
  if (props.job.status === 'done') return t.value('exportJobDone')
  if (props.job.status === 'cancelled') return t.value('exportJobCancelled')
  if (props.job.status === 'error') {
    return formatMessage(t.value('exportJobError'), { error: props.job.error || '' })
  }
  return ''
})

/** 完成/取消/失败视觉分流，避免全部同色灰条 */
const toneClass = computed(() => {
  switch (props.job.status) {
    case 'done':
      return 'border-emerald-200 bg-emerald-50/80 dark:border-emerald-900 dark:bg-emerald-950/40'
    case 'cancelled':
      return 'border-slate-300 bg-slate-100 dark:border-slate-600 dark:bg-slate-800/70'
    case 'error':
      return 'border-red-200 bg-red-50 dark:border-red-900 dark:bg-red-950/40'
    default:
      return 'border-slate-200 bg-slate-50 dark:border-slate-700 dark:bg-slate-900/60'
  }
})

const titleToneClass = computed(() => {
  switch (props.job.status) {
    case 'done':
      return 'text-emerald-900 dark:text-emerald-100'
    case 'error':
      return 'text-red-800 dark:text-red-100'
    case 'cancelled':
      return 'text-slate-700 dark:text-slate-200'
    default:
      return 'text-slate-800 dark:text-slate-100'
  }
})
</script>

<template>
  <div
    v-if="visible"
    class="flex flex-wrap items-center gap-3 rounded-lg border px-3 py-2 text-sm"
    :class="toneClass"
    data-testid="export-job-banner"
    :data-export-status="job.status"
    role="status"
    aria-live="polite"
  >
    <div class="min-w-0 flex-1">
      <div class="font-medium" :class="titleToneClass" data-testid="export-job-title">{{ title }}</div>
      <div
        v-if="busy || job.total > 0"
        class="mt-1 h-1.5 overflow-hidden rounded bg-slate-200 dark:bg-slate-700"
        role="progressbar"
        :aria-valuenow="percent"
        aria-valuemin="0"
        aria-valuemax="100"
        data-testid="export-job-progress"
      >
        <div
          class="h-full bg-sky-500 transition-all dark:bg-sky-400"
          :style="{ width: `${percent}%` }"
        />
      </div>
      <div v-if="job.total > 0" class="mt-0.5 text-[11px] mts-muted" data-testid="export-job-count">
        {{ job.done }}/{{ job.total }}
      </div>
    </div>
    <button
      v-if="busy"
      type="button"
      class="mts-btn"
      data-testid="export-job-cancel"
      @click="emit('cancel')"
    >{{ t('exportJobCancel') }}</button>
    <button
      v-else
      type="button"
      class="mts-focus-ring rounded p-1 opacity-60 hover:opacity-100"
      data-testid="export-job-dismiss"
      :title="t('dismiss')"
      :aria-label="t('dismiss')"
      @click="emit('dismiss')"
    >
      <X class="h-3.5 w-3.5" />
    </button>
  </div>
</template>
