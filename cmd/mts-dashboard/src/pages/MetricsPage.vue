<script setup lang="ts">
import { computed, inject, onBeforeUnmount, onMounted, ref, watch, type ComputedRef } from 'vue'
import { useRoute } from 'vue-router'
import { useHashScroll } from '@/composables/useHashScroll'
import { parseMetricsPrefill, metricsFormToPrefill } from '@/utils/routePrefill'
import { copyText } from '@/utils/clipboard'
import { apiGet, apiGetText } from '@/api/client'
import {
  normalizeDownsampleStatusSummary,
  downsampleStatusSummaryTone,
  downsampleStatusHealthJump,
  type DownsampleStatusSummaryInput,
} from '@/utils/downsampleStatusSummary'
import { useAdminOpBusy } from '@/composables/useAdminOpBusy'
import { formatCaughtError } from '@/utils/apiError'
import { useI18n } from '@/composables/useI18n'
import { formatMessage } from '@/utils/formatMessage'
import { useAuth } from '@/composables/useAuth'
import PermissionDenied from '@/components/PermissionDenied.vue'
import AdminOpLastChip from '@/components/AdminOpLastChip.vue'
import ActionResultBanner from '@/components/ActionResultBanner.vue'
import PartialErrorBanner from '@/components/PartialErrorBanner.vue'
import EmptyState from '@/components/EmptyState.vue'
import VirtualTable from '@/components/VirtualTable.vue'
import {
  filterPrometheusFamilies,
  formatSampleLabels,
  parsePrometheusText,
  summarizeFamilies,
  type PrometheusFamily,
} from '@/utils/prometheus'
import { metricsFamiliesToJSON, metricsRefreshIntervalsMs } from '@/utils/metricsExport'
import { stampFilename } from '@/utils/download'
import { useExportJob } from '@/composables/useExportJob'
import ExportJobBanner from '@/components/ExportJobBanner.vue'
import { useNotify } from '@/composables/useNotify'
import { useNotifyAdminBusy } from '@/composables/useNotifyAdminBusy'

import { Activity, RefreshCw, Download } from 'lucide-vue-next'

const { isAdmin } = useAuth()
const adminOpBusySummary = inject<ComputedRef<{ lastSummary?: string; lastError?: string; lastOk?: boolean | null }> | undefined>('adminOpBusySummary', undefined)
const metricsAdminLastLabel = computed(() => (adminOpBusySummary?.value?.lastSummary || '').trim())
const metricsAdminLastErrorDetail = computed(() => {
  if (adminOpBusySummary?.value?.lastOk !== false) return ''
  return (adminOpBusySummary?.value?.lastError || '').trim()
})
const route = useRoute()
useHashScroll()
const { t } = useI18n()
const { refreshAdminOpBusy } = useAdminOpBusy()
const { success, info, error: notifyError } = useNotify()
const { notifyMaybeAdminBusy } = useNotifyAdminBusy()

const {
  exportJob,
  exportBusy,
  cancelExport,
  resetExport,
  retryLastExport,
  canRetryExport,
  runTextExport,
  runJSONExport,
} = useExportJob()
const loading = ref(false)
const loadError = ref('')
const refreshError = ref('')
const refreshFailStreak = ref(0)
const metricsSourcePath = ref('/metrics')
const opsStatusPath = ref('/api/v1/admin/ops-status')
const downsampleStatusesPath = ref('/api/v1/admin/downsample/statuses')
const raw = ref('')
const families = ref<PrometheusFamily[]>([])
const q = ref('')
const activeFamilyName = ref('')
const lastRefreshed = ref('')
const downsampleStatusSummary = ref<Required<DownsampleStatusSummaryInput> | null>(null)
const downsampleSummaryView = computed(() => downsampleStatusSummary.value || normalizeDownsampleStatusSummary(null))
const downsampleSummaryToneClass = computed(() => {
  const tone = downsampleStatusSummaryTone(downsampleSummaryView.value)
  if (tone === 'bad') return 'border-red-200 bg-red-50/70 dark:border-red-900/40 dark:bg-red-950/20'
  if (tone === 'warn') return 'border-amber-200 bg-amber-50/70 dark:border-amber-900/40 dark:bg-amber-950/20'
  return 'border-emerald-200 bg-emerald-50/50 dark:border-emerald-900/40 dark:bg-emerald-950/20'
})
const downsampleErrorJump = computed(() => downsampleStatusHealthJump('error'))
const downsampleLaggingJump = computed(() => downsampleStatusHealthJump('lagging'))
const autoRefreshMs = ref(0)
let timer: ReturnType<typeof setInterval> | null = null

const FAMILY_ROW_HEIGHT = 64
const METRICS_LIST_HEIGHT = 480

const filtered = computed(() => filterPrometheusFamilies(families.value, q.value))
const summary = computed(() => summarizeFamilies(filtered.value))
const refreshOptions = metricsRefreshIntervalsMs()
const activeFamily = computed(() => filtered.value.find((f) => f.name === activeFamilyName.value) ?? null)

async function load(opts?: { background?: boolean }) {
  if (!isAdmin.value) return
  const background = !!opts?.background
  if (!background) {
    loading.value = true
    loadError.value = ''
  }
  try {
    const text = await apiGetText('/metrics')
    raw.value = text
    families.value = parsePrometheusText(text)
    lastRefreshed.value = new Date().toLocaleTimeString()
    refreshError.value = ''
    refreshFailStreak.value = 0
    // /metrics 无 JSON busy/last；刷新后联动 ops-status 保持管理重操作可见
    void refreshAdminOpBusy()
    // 降采样健康摘要：失败不阻断 metrics；summary_only 减少载荷
    try {
      const st = await apiGet<{ summary?: DownsampleStatusSummaryInput; path?: string }>(
        '/api/v1/admin/downsample/statuses?summary_only=1',
      )
      if (st.path) downsampleStatusesPath.value = String(st.path)
      downsampleStatusSummary.value = normalizeDownsampleStatusSummary(st.summary)
    } catch {
      if (!downsampleStatusSummary.value) {
        downsampleStatusSummary.value = normalizeDownsampleStatusSummary(null)
      }
    }
  } catch (e) {
    const msg = formatCaughtError(e)
    const hasData = !!raw.value || families.value.length > 0
    if (background && hasData) {
      // 自动刷新失败：保留上次快照，避免闪空 + toast 风暴
      refreshError.value = msg
      refreshFailStreak.value += 1
      if (refreshFailStreak.value === 1) notifyMaybeAdminBusy(`${t.value('metricsRefreshFailed')}：${msg}`, e)
    } else {
      loadError.value = msg
      families.value = []
      raw.value = ''
      refreshError.value = ''
      notifyMaybeAdminBusy(msg, e)
    }
  } finally {
    if (!background) loading.value = false
  }
}

function toggle(name: string) {
  activeFamilyName.value = activeFamilyName.value === name ? '' : name
}

function expandAll() {
  // 单展开面板：全选展开时定位到筛选列表首项
  activeFamilyName.value = filtered.value[0]?.name ?? ''
}

function collapseAll() {
  activeFamilyName.value = ''
}

async function exportRaw() {
  if (!raw.value) {
    notifyError(t.value('metricsEmpty'))
    return
  }
  if (exportBusy.value) return
  const text = raw.value
  const outcome = await runTextExport({
    label: 'TXT',
    filename: stampFilename('mts-metrics', 'txt'),
    mime: 'text/plain;charset=utf-8',
    total: 1,
    build: async ({ isCancelled, progress }) => {
      progress(0, 1)
      if (isCancelled()) return null
      progress(1, 1)
      return text
    },
  })
  if (outcome === 'done') success(t.value('metricsExported'))
  else if (outcome === 'cancelled') info(t.value('exportCancelledToast'))
  else if (outcome === 'error') notifyError(exportJob.value.error || t.value('failed'))
}

async function exportJSON() {
  if (!filtered.value.length) {
    notifyError(t.value('metricsEmpty'))
    return
  }
  if (exportBusy.value) return
  const list = filtered.value.slice()
  const outcome = await runJSONExport({
    label: 'JSON',
    filename: stampFilename('mts-metrics', 'json'),
    total: list.length,
    build: async ({ isCancelled, progress }) => {
      progress(0, list.length)
      const chunk = 50
      for (let i = 0; i < list.length; i++) {
        if (isCancelled()) return null
        const done = i + 1
        if (done === list.length || done % chunk === 0) {
          progress(done, list.length)
          if (done < list.length) await new Promise((r) => setTimeout(r, 0))
        }
      }
      return metricsFamiliesToJSON(list, {
        downsample_status_summary: downsampleStatusSummary.value,
      })
    },
  })
  if (outcome === 'done') success(t.value('metricsExported'))
  else if (outcome === 'cancelled') info(t.value('exportCancelledToast'))
  else if (outcome === 'error') notifyError(exportJob.value.error || t.value('failed'))
}

function clearTimer() {
  if (timer) {
    clearInterval(timer)
    timer = null
  }
}

function setupAutoRefresh() {
  clearTimer()
  if (!autoRefreshMs.value || autoRefreshMs.value <= 0) return
  timer = setInterval(() => {
    void load({ background: true })
  }, autoRefreshMs.value)
}

watch(autoRefreshMs, () => {
  setupAutoRefresh()
})

watch(filtered, (list) => {
  if (activeFamilyName.value && !list.some((f) => f.name === activeFamilyName.value)) {
    activeFamilyName.value = ''
  }
})

function applyMetricsPrefillFromRoute() {
  if (!isAdmin.value) return
  const pre = parseMetricsPrefill(route.query as Record<string, unknown>)
  let changed = false
  if (pre.q != null && q.value !== pre.q) {
    q.value = pre.q
    changed = true
  }
  if (pre.family) {
    if (activeFamilyName.value !== pre.family) {
      activeFamilyName.value = pre.family
      changed = true
    }
  }
  if (changed) success(t.value('metricsPrefillApplied'))
}

async function copyMetricsShareLink() {
  const path = metricsFormToPrefill({
    q: q.value,
    family: activeFamilyName.value || undefined,
  }, { hash: activeFamilyName.value ? '#metrics-detail' : '#metrics-list' })
  const url = `${window.location.origin}${path}`
  const res = await copyText(url)
  if (res.ok) success(t.value('metricsShareCopied'))
  else notifyError(res.error || t.value('failed'))
}

watch(
  () => route.fullPath,
  (path, prev) => {
    if (!isAdmin.value) return
    if (prev != null && path !== prev) applyMetricsPrefillFromRoute()
  },
)

onMounted(() => {
  applyMetricsPrefillFromRoute()
  void load()
  setupAutoRefresh()
})

onBeforeUnmount(() => {
  clearTimer()
})
</script>

<template>
  <PermissionDenied v-if="!isAdmin" />
  <div v-else class="space-y-4" data-testid="metrics-page">
    <div class="flex flex-wrap items-start justify-between gap-3">
      <div>
        <h1 class="mts-title flex items-center gap-2">
          <Activity class="h-5 w-5" />
          {{ t('metricsTitle') }}
        </h1>
        <div class="mt-1 flex flex-wrap items-center gap-2">
          <p class="text-xs mts-muted">
            {{ t('metricsDesc') }}
            <span v-if="lastRefreshed" data-testid="metrics-refreshed">{{ formatMessage(t('metricsRefreshedAt'), { time: lastRefreshed }) }}</span>
            <p
              class="max-w-full truncate font-mono text-[10px] text-slate-500 dark:text-slate-400"
              data-testid="metrics-source-paths"
              :title="[metricsSourcePath, opsStatusPath, downsampleStatusesPath].join(' · ')"
            >{{ [metricsSourcePath, opsStatusPath, downsampleStatusesPath].join(' · ') }}</p>
          </p>
          <AdminOpLastChip
            v-if="metricsAdminLastLabel"
            :label="metricsAdminLastLabel"
            :last-ok="adminOpBusySummary?.lastOk"
            :last-error="metricsAdminLastErrorDetail"
            test-id="metrics-admin-last"
            show-copy
            copy-test-id="metrics-admin-last-copy"
            error-test-id="metrics-admin-last-error"
          />

        </div>
      </div>
      <div class="flex flex-wrap items-center gap-2">
        <label class="flex items-center gap-1 text-xs mts-muted">
          {{ t('metricsAutoRefresh') }}
          <select v-model.number="autoRefreshMs" class="mts-input py-1 text-xs" data-testid="metrics-auto-refresh">
            <option v-for="ms in refreshOptions" :key="ms" :value="ms">
              {{ ms === 0 ? t('metricsAutoOff') : `${ms / 1000}s` }}
            </option>
          </select>
        </label>
        <button type="button" class="mts-btn" data-testid="metrics-share-link" @click="copyMetricsShareLink">
          {{ t('metricsShareLink') }}
        </button>
        <ExportJobBanner class="w-full basis-full" :job="exportJob" :retryable="canRetryExport" @cancel="cancelExport" @retry="retryLastExport" @dismiss="resetExport" />
        <button type="button" class="mts-btn" data-testid="metrics-export-raw" :disabled="exportBusy || (!raw)" @click="exportRaw">
          <Download class="h-3.5 w-3.5" /> {{ t('metricsExportRaw') }}
        </button>
        <button type="button" class="mts-btn" data-testid="metrics-export-json" :disabled="exportBusy || (!filtered.length)" @click="exportJSON">
          <Download class="h-3.5 w-3.5" /> {{ t('metricsExportJSON') }}
        </button>
        <button type="button" class="mts-btn" data-testid="metrics-refresh" :disabled="loading" :aria-busy="loading ? 'true' : undefined" @click="() => load()">
          <RefreshCw class="h-3.5 w-3.5" :class="loading ? 'animate-spin' : ''" /> {{ t('refresh') }}
        </button>
      </div>
    </div>

    <div v-if="loadError" data-testid="metrics-error">
      <div
      v-if="downsampleStatusSummary"
      class="rounded-lg border p-3"
      :class="downsampleSummaryToneClass"
      data-testid="metrics-downsample-summary"
    >
      <div class="flex flex-wrap items-start justify-between gap-2">
        <div>
          <p class="text-sm font-semibold text-slate-800 dark:text-slate-100">{{ t('metricsDownsampleSummaryTitle') }}</p>
          <p class="text-[11px] mts-muted">{{ t('metricsDownsampleSummaryDesc') }}</p>
        </div>
        <div class="flex flex-wrap gap-1.5">
          <router-link class="mts-btn text-xs" :to="downsampleErrorJump" data-testid="metrics-downsample-jump-error">{{ t('overviewDownsampleJumpError') }}</router-link>
          <router-link class="mts-btn text-xs" :to="downsampleLaggingJump" data-testid="metrics-downsample-jump-lagging">{{ t('overviewDownsampleJumpLagging') }}</router-link>
        </div>
      </div>
      <div class="mt-2 grid grid-cols-2 gap-2 text-xs sm:grid-cols-4">
        <div data-testid="metrics-downsample-total">{{ t('overviewDownsampleTotal') }}: <span class="font-semibold">{{ downsampleSummaryView.total }}</span></div>
        <div data-testid="metrics-downsample-errors">{{ t('overviewDownsampleErrors') }}: <span class="font-semibold">{{ downsampleSummaryView.error }}</span></div>
        <div data-testid="metrics-downsample-lagging">{{ t('overviewDownsampleLagging') }}: <span class="font-semibold">{{ downsampleSummaryView.lagging }}</span></div>
        <div data-testid="metrics-downsample-max-lag">{{ t('overviewDownsampleMaxLag') }}: <span class="font-semibold">{{ downsampleSummaryView.max_lag_seconds }}s</span></div>
      </div>
    </div>

    <ActionResultBanner kind="error" :message="loadError" retryable data-testid="metrics-load-error" @retry="() => load()" @dismiss="loadError = ''" />
    </div>
    <PartialErrorBanner
      v-else-if="refreshError"
      :message="`${t('metricsRefreshFailed')}：${refreshError}`"
      test-id="metrics-refresh-error"
      @retry="() => load({ background: true })"
      @dismiss="refreshError = ''"
    />

    <div class="grid gap-3 sm:grid-cols-2">
      <div class="mts-card p-3" id="metrics-summary" data-testid="metrics-summary-families">
        <p class="text-xs mts-muted">{{ t('metricsFamilies') }}</p>
        <p class="text-xl font-semibold text-slate-800 dark:text-slate-100">{{ summary.families }}</p>
      </div>
      <div class="mts-card p-3" data-testid="metrics-summary-samples">
        <p class="text-xs mts-muted">{{ t('metricsSamples') }}</p>
        <p class="text-xl font-semibold text-slate-800 dark:text-slate-100">{{ summary.samples }}</p>
      </div>
    </div>

    <div class="flex flex-wrap items-center gap-2">
      <input
        v-model="q"
        class="mts-input max-w-xl text-sm"
        id="metrics-filter" data-testid="metrics-filter"
        :placeholder="t('metricsFilterPlaceholder')"
        :aria-label="t('metricsFilterPlaceholder')"
      />
      <button type="button" class="mts-btn" data-testid="metrics-expand-all" @click="expandAll">{{ t('metricsExpandAll') }}</button>
      <button type="button" class="mts-btn" data-testid="metrics-collapse-all" @click="collapseAll">{{ t('metricsCollapseAll') }}</button>
    </div>

    <div v-if="!loading && !filtered.length" class="mts-card" data-testid="metrics-empty">
      <EmptyState :title="t('metricsEmpty')" :description="t('metricsEmptyDesc')">
        <template v-if="q.trim()" #action>
          <button type="button" class="mts-btn-primary" data-testid="metrics-clear-filters" @click="q = ''">{{ t('clearFilters') }}</button>
        </template>
      </EmptyState>
    </div>
    <div class="space-y-3 scroll-mt-20" v-else id="metrics-list" data-testid="metrics-list">
      <div class="overflow-hidden rounded-xl border border-slate-200 bg-white dark:border-slate-700 dark:bg-slate-900">
        <VirtualTable
          :items="filtered"
          :row-height="FAMILY_ROW_HEIGHT"
          :height="METRICS_LIST_HEIGHT"
          data-testid="metrics-virtual-list"
        >
          <template #default="{ item: fam }">
            <button
              type="button"
              class="flex h-full w-full items-start justify-between gap-3 border-b border-slate-100 px-3 py-2 text-left hover:bg-slate-50 dark:border-slate-800 dark:hover:bg-slate-800/50"
              :class="activeFamilyName === fam.name ? 'bg-slate-50 dark:bg-slate-800/40' : ''"
              :data-testid="`metrics-family-${fam.name}`"
              :aria-expanded="activeFamilyName === fam.name ? 'true' : 'false'"
              @click="toggle(fam.name)"
            >
              <div class="min-w-0">
                <p class="truncate font-mono text-sm font-medium text-slate-800 dark:text-slate-100" :title="fam.name">{{ fam.name }}</p>
                <p class="truncate text-xs mts-muted" :title="fam.help || ''">
                  <span v-if="fam.type" class="mr-2 uppercase">{{ fam.type }}</span>
                  {{ fam.help || t('emptyValue') }}
                </p>
              </div>
              <span class="shrink-0 text-xs mts-muted whitespace-nowrap">{{ formatMessage(t('metricsSampleCount'), { count: fam.samples.length }) }}</span>
            </button>
          </template>
        </VirtualTable>
        <p class="border-t border-slate-100 px-3 py-1.5 text-[11px] mts-muted dark:border-slate-800" data-testid="metrics-virtual-hint">
          {{ t('metricsVirtualHint') }}
        </p>
      </div>

      <div
        v-if="activeFamily"
        id="metrics-detail"
        class="mts-card scroll-mt-20 overflow-hidden"
        data-testid="metrics-detail-panel"
      >
        <div class="flex flex-wrap items-center justify-between gap-2 border-b border-slate-100 px-3 py-2 dark:border-slate-800">
          <div class="min-w-0">
            <p class="truncate font-mono text-sm font-medium text-slate-800 dark:text-slate-100" data-testid="metrics-detail-name">{{ activeFamily.name }}</p>
            <p class="text-xs mts-muted">
              <span v-if="activeFamily.type" class="mr-2 uppercase">{{ activeFamily.type }}</span>
              {{ activeFamily.help || t('emptyValue') }}
            </p>
          </div>
          <button type="button" class="mts-btn" data-testid="metrics-detail-collapse" @click="collapseAll">
            {{ t('metricsDetailCollapse') }}
          </button>
        </div>
        <div class="mts-table-wrap">
          <table class="min-w-full text-left text-xs">
            <thead class="bg-slate-50 text-slate-500 dark:bg-slate-900/50 dark:text-slate-400">
              <tr>
                <th class="px-3 py-1.5 font-medium">{{ t('metricsColLabels') }}</th>
                <th class="px-3 py-1.5 font-medium">{{ t('metricsColValue') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr
                v-for="(s, i) in activeFamily.samples"
                :key="i"
                class="border-t border-slate-100 dark:border-slate-800"
              >
                <td class="px-3 py-1.5 font-mono text-slate-600 dark:text-slate-300">
                  {{ formatSampleLabels(s.labels) || t('emptyValue') }}
                </td>
                <td class="px-3 py-1.5 font-mono text-slate-800 dark:text-slate-100">{{ s.value }}</td>
              </tr>
              <tr v-if="!activeFamily.samples.length">
                <td colspan="2" class="px-3 py-2 text-xs mts-muted">{{ t('emptyValue') }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
    </div>

    <details class="mts-card p-3 text-xs" data-testid="metrics-raw">
      <summary class="cursor-pointer text-slate-600 dark:text-slate-300">{{ t('metricsRawPreview') }}</summary>
      <pre class="mt-2 max-h-64 overflow-auto whitespace-pre-wrap break-all font-mono text-[11px] text-slate-500 dark:text-slate-400">{{ raw.slice(0, 8192) }}{{ raw.length > 8192 ? '\n…' : '' }}</pre>
    </details>
  </div>
</template>
