<script setup lang="ts">
import { computed } from 'vue'
import { useRouter } from 'vue-router'
import { useNotify } from '@/composables/useNotify'
import { useI18n } from '@/composables/useI18n'
import { notifyDisplayText } from '@/utils/notifyQueue'
import {
  hasOpenNotifyHistory,
  requestOpenNotifyHistory,
} from '@/utils/notifyHistoryBridge'

const { items, dismiss } = useNotify()
const { t } = useI18n()
const router = useRouter()
// items 变化时重算，确保 DashboardLayout 挂载后按钮出现
const canOpenHistory = computed(() => {
  void items.value.length
  return hasOpenNotifyHistory()
})

function openHistory(id: number) {
  dismiss(id)
  requestOpenNotifyHistory()
}

function runAction(id: number, path: string) {
  dismiss(id)
  const target = String(path || '').trim()
  if (!target) return
  void router.push(target)
}
</script>

<template>
  <div
    class="pointer-events-none fixed right-4 top-4 z-[100] flex w-80 max-w-[calc(100vw-2rem)] flex-col gap-2"
    data-testid="notify-host"
    aria-live="polite"
    aria-relevant="additions text"
  >
    <div
      v-for="n in items"
      :key="n.id"
      class="pointer-events-auto rounded-lg border px-3 py-2 text-sm shadow-lg"
      :data-testid="`notify-${n.kind}`"
      :role="n.kind === 'error' ? 'alert' : 'status'"
      :class="{
        'border-emerald-200 bg-emerald-50 text-emerald-800 dark:border-emerald-900 dark:bg-emerald-950/50 dark:text-emerald-200': n.kind === 'success',
        'border-red-200 bg-red-50 text-red-800 dark:border-red-900 dark:bg-red-950/50 dark:text-red-200': n.kind === 'error',
        'border-amber-200 bg-amber-50 text-amber-900 dark:border-amber-900 dark:bg-amber-950/40 dark:text-amber-100': n.kind === 'warn',
        'border-slate-200 bg-white text-slate-700 dark:border-slate-700 dark:bg-slate-900 dark:text-slate-200': n.kind === 'info',
      }"
    >
      <div class="flex items-start justify-between gap-2">
        <p class="flex-1 break-words">{{ notifyDisplayText(n) }}</p>
        <div class="flex shrink-0 flex-col items-end gap-1">
          <button
            type="button"
            class="mts-focus-ring rounded text-xs opacity-60 hover:opacity-100"
            :aria-label="t('close')"
            :title="t('close')"
            data-testid="notify-dismiss"
            @click="dismiss(n.id)"
          >{{ t('close') }}</button>
          <button
            v-if="n.action?.path"
            type="button"
            class="mts-focus-ring rounded text-[11px] font-medium underline-offset-2 hover:underline opacity-80 hover:opacity-100"
            data-testid="notify-action"
            :aria-label="n.action.label"
            @click="runAction(n.id, n.action.path)"
          >{{ n.action.label }}</button>
          <button
            v-if="canOpenHistory && (n.kind === 'error' || n.kind === 'warn')"
            type="button"
            class="mts-focus-ring rounded text-[11px] font-medium underline-offset-2 hover:underline opacity-80 hover:opacity-100"
            data-testid="notify-open-history"
            :aria-label="t('notifyOpenHistory')"
            @click="openHistory(n.id)"
          >{{ t('notifyOpenHistory') }}</button>
        </div>
      </div>
    </div>
  </div>
</template>
