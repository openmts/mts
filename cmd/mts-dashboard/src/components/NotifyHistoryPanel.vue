<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from '@/composables/useI18n'
import { useNotify } from '@/composables/useNotify'
import { createFocusTrap, type FocusTrapHandle } from '@/utils/focusTrap'
import { Bell, X, Download, Copy } from 'lucide-vue-next'
import {
  filterNotifyHistory,
  notifyHistoryRangeBounds,
  parseNotifyTimeBound,
  type NotifyHistoryEntry,
  type NotifyHistoryKindFilter,
  type NotifyHistoryQuickRange,
} from '@/utils/notifyHistory'
import { toDatetimeLocalValue } from '@/utils/commandPalette'
import {
  buildNotifyHistoryExport,
  formatNotifyHistoryExportPretty,
  notifyHistoryToCSV,
} from '@/utils/notifyHistoryExport'
import { downloadJSON, downloadText, stampFilename } from '@/utils/download'
import { copyText } from '@/utils/clipboard'

const open = defineModel<boolean>('open', { default: false })
const { t, locale } = useI18n()
const { history, clearHistory, reloadHistory, success, error: notifyError } = useNotify()
const panelRef = ref<HTMLElement | null>(null)
let trap: FocusTrapHandle | null = null

const kindFilter = ref<NotifyHistoryKindFilter>('all')
const searchQuery = ref('')
const timeRange = ref<NotifyHistoryQuickRange>('all')
const sinceLocal = ref('')
const untilLocal = ref('')
const timeBounds = computed(() => {
  if (timeRange.value !== 'all') {
    return notifyHistoryRangeBounds(timeRange.value)
  }
  return {
    sinceMs: parseNotifyTimeBound(sinceLocal.value),
    untilMs: parseNotifyTimeBound(untilLocal.value),
  }
})
const entries = computed(() =>
  filterNotifyHistory(history.value, {
    kind: kindFilter.value,
    query: searchQuery.value,
    sinceMs: timeBounds.value.sinceMs,
    untilMs: timeBounds.value.untilMs,
  }),
)
const filterOptions: NotifyHistoryKindFilter[] = ['all', 'success', 'error', 'warn', 'info']
const timeRangeOptions: NotifyHistoryQuickRange[] = ['all', '1h', '24h', '7d', '30d']
function filterOptionLabel(k: NotifyHistoryKindFilter): string {
  if (k === 'all') return t.value('notifyHistoryFilterAll')
  return kindLabel(k)
}
function timeRangeLabel(r: NotifyHistoryQuickRange): string {
  if (r === 'all') return t.value('notifyHistoryTimeAll')
  if (r === '1h') return t.value('notifyHistoryTime1h')
  if (r === '24h') return t.value('notifyHistoryTime24h')
  if (r === '7d') return t.value('notifyHistoryTime7d')
  return t.value('notifyHistoryTime30d')
}
function onTimeRangeChange() {
  if (timeRange.value === 'all') return
  const b = notifyHistoryRangeBounds(timeRange.value)
  sinceLocal.value = b.sinceMs != null ? toDatetimeLocalValue(new Date(b.sinceMs)) : ''
  untilLocal.value = b.untilMs != null ? toDatetimeLocalValue(new Date(b.untilMs)) : ''
}
function onCustomTimeInput() {
  // 自定义时间时切回 all 语义（仍用 since/until）
  timeRange.value = 'all'
}
function clearTimeFilter() {
  timeRange.value = 'all'
  sinceLocal.value = ''
  untilLocal.value = ''
}
const hasActiveFilter = computed(
  () =>
    kindFilter.value !== 'all' ||
    !!searchQuery.value.trim() ||
    timeRange.value !== 'all' ||
    !!sinceLocal.value ||
    !!untilLocal.value,
)

function close() {
  open.value = false
}

function onClear() {
  clearHistory()
}

function onExportJSON() {
  if (!entries.value.length) {
    notifyError(t.value('notifyHistoryExportEmpty'))
    return
  }
  downloadJSON(
    stampFilename('mts-notify-history', 'json'),
    buildNotifyHistoryExport(entries.value),
  )
  success(t.value('notifyHistoryExported'))
}

function onExportCSV() {
  if (!entries.value.length) {
    notifyError(t.value('notifyHistoryExportEmpty'))
    return
  }
  downloadText(
    stampFilename('mts-notify-history', 'csv'),
    notifyHistoryToCSV(entries.value),
    'text/csv;charset=utf-8',
  )
  success(t.value('notifyHistoryExported'))
}

async function onCopy() {
  if (!entries.value.length) {
    notifyError(t.value('notifyHistoryExportEmpty'))
    return
  }
  const res = await copyText(formatNotifyHistoryExportPretty(entries.value))
  if (res.ok) success(t.value('notifyHistoryCopied'))
  else notifyError(res.error || t.value('failed'))
}

function kindLabel(kind: NotifyHistoryEntry['kind']): string {
  if (kind === 'success') return t.value('notifyKindSuccess')
  if (kind === 'error') return t.value('notifyKindError')
  if (kind === 'warn') return t.value('notifyKindWarn')
  return t.value('notifyKindInfo')
}

function formatAt(at: number): string {
  try {
    return new Date(at).toLocaleString(locale.value === 'en' ? 'en' : 'zh-CN')
  } catch {
    return String(at)
  }
}

watch(open, async (v) => {
  trap?.release()
  trap = null
  if (!v) return
  reloadHistory()
  await nextTick()
  if (panelRef.value) {
    trap = createFocusTrap(panelRef.value)
    trap.focusFirst()
  }
})

onBeforeUnmount(() => {
  trap?.release()
  trap = null
})
</script>

<template>
  <div
    v-if="open"
    class="fixed inset-0 z-[125] flex justify-end bg-black/30"
    data-testid="notify-history-overlay"
    @click.self="close"
    @keydown.esc.prevent="close"
  >
    <div
      ref="panelRef"
      role="dialog"
      aria-modal="true"
      :aria-label="t('notifyHistoryTitle')"
      class="flex h-full w-full max-w-md flex-col border-l border-slate-200 bg-white shadow-xl dark:border-slate-700 dark:bg-slate-900"
      data-testid="notify-history-panel"
    >
      <div class="flex items-center justify-between gap-2 border-b border-slate-200 px-4 py-3 dark:border-slate-700">
        <h2 class="flex items-center gap-2 text-sm font-semibold text-slate-800 dark:text-slate-100">
          <Bell class="h-4 w-4" aria-hidden="true" />
          {{ t('notifyHistoryTitle') }}
        </h2>
        <div class="flex flex-wrap items-center gap-1">
          <button
            type="button"
            class="mts-btn mts-focus-ring"
            data-testid="notify-history-export-json"
            :disabled="!entries.length"
            :title="t('notifyHistoryExportJSON')"
            @click="onExportJSON"
          >
            <Download class="h-3.5 w-3.5" aria-hidden="true" />
            JSON
          </button>
          <button
            type="button"
            class="mts-btn mts-focus-ring"
            data-testid="notify-history-export-csv"
            :disabled="!entries.length"
            :title="t('notifyHistoryExportCSV')"
            @click="onExportCSV"
          >
            <Download class="h-3.5 w-3.5" aria-hidden="true" />
            CSV
          </button>
          <button
            type="button"
            class="mts-btn mts-focus-ring"
            data-testid="notify-history-copy"
            :disabled="!entries.length"
            :title="t('notifyHistoryCopy')"
            @click="onCopy"
          >
            <Copy class="h-3.5 w-3.5" aria-hidden="true" />
            {{ t('notifyHistoryCopy') }}
          </button>
          <button
            type="button"
            class="mts-btn mts-focus-ring"
            data-testid="notify-history-clear"
            :disabled="!entries.length"
            @click="onClear"
          >
            {{ t('notifyHistoryClear') }}
          </button>
          <button
            type="button"
            class="mts-focus-ring rounded p-1 text-slate-400 hover:bg-slate-100 dark:hover:bg-slate-800"
            data-testid="notify-history-close"
            :aria-label="t('close')"
            :title="t('close')"
            @click="close"
          >
            <X class="h-4 w-4" aria-hidden="true" />
          </button>
        </div>
      </div>
      <p class="border-b border-slate-100 px-4 py-2 text-xs mts-muted dark:border-slate-800">
        {{ t('notifyHistoryHint') }}
      </p>
      <div class="space-y-2 border-b border-slate-100 px-4 py-2 dark:border-slate-800" data-testid="notify-history-filter-wrap">
        <div>
          <label class="mb-1 block text-[11px] font-medium text-slate-600 dark:text-slate-300" for="notify-history-filter">
            {{ t('notifyHistoryFilter') }}
          </label>
          <select
            id="notify-history-filter"
            v-model="kindFilter"
            class="mts-input mts-focus-ring text-xs"
            data-testid="notify-history-filter"
          >
            <option v-for="k in filterOptions" :key="k" :value="k">{{ filterOptionLabel(k) }}</option>
          </select>
        </div>
        <div>
          <label class="mb-1 block text-[11px] font-medium text-slate-600 dark:text-slate-300" for="notify-history-search">
            {{ t('notifyHistorySearch') }}
          </label>
          <input
            id="notify-history-search"
            v-model="searchQuery"
            type="search"
            class="mts-input mts-focus-ring text-xs"
            data-testid="notify-history-search"
            :placeholder="t('notifyHistorySearchPlaceholder')"
            autocomplete="off"
          />
        </div>
        <div>
          <label class="mb-1 block text-[11px] font-medium text-slate-600 dark:text-slate-300" for="notify-history-time-range">
            {{ t('notifyHistoryTimeRange') }}
          </label>
          <select
            id="notify-history-time-range"
            v-model="timeRange"
            class="mts-input mts-focus-ring text-xs"
            data-testid="notify-history-time-range"
            @change="onTimeRangeChange"
          >
            <option v-for="r in timeRangeOptions" :key="r" :value="r">{{ timeRangeLabel(r) }}</option>
          </select>
        </div>
        <div class="grid grid-cols-1 gap-2 sm:grid-cols-2">
          <div>
            <label class="mb-1 block text-[11px] font-medium text-slate-600 dark:text-slate-300" for="notify-history-since">
              {{ t('notifyHistorySince') }}
            </label>
            <input
              id="notify-history-since"
              v-model="sinceLocal"
              type="datetime-local"
              class="mts-input mts-focus-ring text-xs"
              data-testid="notify-history-since"
              @change="onCustomTimeInput"
            />
          </div>
          <div>
            <label class="mb-1 block text-[11px] font-medium text-slate-600 dark:text-slate-300" for="notify-history-until">
              {{ t('notifyHistoryUntil') }}
            </label>
            <input
              id="notify-history-until"
              v-model="untilLocal"
              type="datetime-local"
              class="mts-input mts-focus-ring text-xs"
              data-testid="notify-history-until"
              @change="onCustomTimeInput"
            />
          </div>
        </div>
        <button
          type="button"
          class="mts-btn mts-focus-ring w-full text-xs"
          data-testid="notify-history-time-clear"
          :disabled="!hasActiveFilter"
          @click="clearTimeFilter(); kindFilter = 'all'; searchQuery = ''"
        >
          {{ t('notifyHistoryClearFilters') }}
        </button>
      </div>
      <ul class="flex-1 space-y-2 overflow-auto p-3" data-testid="notify-history-list">
        <li
          v-if="!entries.length"
          class="px-2 py-8 text-center text-sm mts-muted"
          data-testid="notify-history-empty"
        >
          {{
            history.length && hasActiveFilter
              ? t('notifyHistoryFilterEmpty')
              : t('notifyHistoryEmpty')
          }}
        </li>
        <li
          v-for="e in entries"
          :key="e.id"
          class="rounded-lg border border-slate-200 px-3 py-2 text-sm dark:border-slate-700"
          :data-testid="`notify-history-item-${e.kind}`"
        >
          <div class="mb-1 flex items-center justify-between gap-2">
            <span
              class="rounded px-1.5 py-0.5 text-[10px] font-medium uppercase"
              :class="{
                'bg-emerald-100 text-emerald-800 dark:bg-emerald-950/50 dark:text-emerald-200': e.kind === 'success',
                'bg-red-100 text-red-800 dark:bg-red-950/50 dark:text-red-200': e.kind === 'error',
                'bg-amber-100 text-amber-900 dark:bg-amber-950/40 dark:text-amber-100': e.kind === 'warn',
                'bg-slate-100 text-slate-700 dark:bg-slate-800 dark:text-slate-200': e.kind === 'info',
              }"
            >{{ kindLabel(e.kind) }}</span>
            <time class="text-[10px] mts-muted" :datetime="new Date(e.at).toISOString()">{{ formatAt(e.at) }}</time>
          </div>
          <p class="break-words text-slate-700 dark:text-slate-200">
            {{ e.message }}<span v-if="e.count > 1" class="mts-muted"> (×{{ e.count }})</span>
          </p>
        </li>
      </ul>
    </div>
  </div>
</template>
