<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { apiGetText } from '@/api/client'
import { formatCaughtError } from '@/utils/apiError'
import { useI18n } from '@/composables/useI18n'
import { formatMessage } from '@/utils/formatMessage'
import { useAuth } from '@/composables/useAuth'
import PermissionDenied from '@/components/PermissionDenied.vue'
import ActionResultBanner from '@/components/ActionResultBanner.vue'
import EmptyState from '@/components/EmptyState.vue'
import {
  filterPrometheusFamilies,
  formatSampleLabels,
  parsePrometheusText,
  summarizeFamilies,
  type PrometheusFamily,
} from '@/utils/prometheus'
import { metricsFamiliesToJSON, metricsRefreshIntervalsMs } from '@/utils/metricsExport'
import { downloadJSON, downloadText, stampFilename } from '@/utils/download'
import { useNotify } from '@/composables/useNotify'
import { Activity, RefreshCw, Download } from 'lucide-vue-next'

const { isAdmin } = useAuth()
const { t } = useI18n()
const { success, error: notifyError } = useNotify()
const loading = ref(false)
const loadError = ref('')
const raw = ref('')
const families = ref<PrometheusFamily[]>([])
const q = ref('')
const expanded = ref<Record<string, boolean>>({})
const lastRefreshed = ref('')
const autoRefreshMs = ref(0)
let timer: ReturnType<typeof setInterval> | null = null

const filtered = computed(() => filterPrometheusFamilies(families.value, q.value))
const summary = computed(() => summarizeFamilies(filtered.value))
const refreshOptions = metricsRefreshIntervalsMs()

async function load() {
  if (!isAdmin.value) return
  loading.value = true
  loadError.value = ''
  try {
    const text = await apiGetText('/metrics')
    raw.value = text
    families.value = parsePrometheusText(text)
    lastRefreshed.value = new Date().toLocaleTimeString()
  } catch (e) {
    loadError.value = formatCaughtError(e)
    families.value = []
    raw.value = ''
    notifyError(loadError.value)
  } finally {
    loading.value = false
  }
}

function toggle(name: string) {
  expanded.value = { ...expanded.value, [name]: !expanded.value[name] }
}

function expandAll() {
  const next: Record<string, boolean> = {}
  for (const f of filtered.value) next[f.name] = true
  expanded.value = next
}

function collapseAll() {
  expanded.value = {}
}

function exportRaw() {
  if (!raw.value) {
    notifyError(t.value('metricsEmpty'))
    return
  }
  downloadText(stampFilename('mts-metrics', 'txt'), raw.value, 'text/plain;charset=utf-8')
  success(t.value('metricsExported'))
}

function exportJSON() {
  if (!filtered.value.length) {
    notifyError(t.value('metricsEmpty'))
    return
  }
  downloadJSON(stampFilename('mts-metrics', 'json'), metricsFamiliesToJSON(filtered.value))
  success(t.value('metricsExported'))
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
    void load()
  }, autoRefreshMs.value)
}

watch(autoRefreshMs, () => {
  setupAutoRefresh()
})

onMounted(() => {
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
        <p class="text-xs mts-muted">
          {{ t('metricsDesc') }}
          <span v-if="lastRefreshed" data-testid="metrics-refreshed">{{ formatMessage(t('metricsRefreshedAt'), { time: lastRefreshed }) }}</span>
        </p>
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
        <button type="button" class="mts-btn" data-testid="metrics-export-raw" :disabled="!raw" @click="exportRaw">
          <Download class="h-3.5 w-3.5" /> {{ t('metricsExportRaw') }}
        </button>
        <button type="button" class="mts-btn" data-testid="metrics-export-json" :disabled="!filtered.length" @click="exportJSON">
          <Download class="h-3.5 w-3.5" /> {{ t('metricsExportJSON') }}
        </button>
        <button type="button" class="mts-btn" data-testid="metrics-refresh" :disabled="loading" :aria-busy="loading ? 'true' : undefined" @click="load">
          <RefreshCw class="h-3.5 w-3.5" :class="loading ? 'animate-spin' : ''" /> {{ t('refresh') }}
        </button>
      </div>
    </div>

    <div v-if="loadError" data-testid="metrics-error">
      <ActionResultBanner kind="error" :message="loadError" @dismiss="loadError = ''" />
    </div>

    <div class="grid gap-3 sm:grid-cols-2">
      <div class="mts-card p-3" data-testid="metrics-summary-families">
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
        data-testid="metrics-filter"
        :placeholder="t('metricsFilterPlaceholder')"
        :aria-label="t('metricsFilterPlaceholder')"
      />
      <button type="button" class="mts-btn" data-testid="metrics-expand-all" @click="expandAll">{{ t('metricsExpandAll') }}</button>
      <button type="button" class="mts-btn" data-testid="metrics-collapse-all" @click="collapseAll">{{ t('metricsCollapseAll') }}</button>
    </div>

    <div v-if="!loading && !filtered.length" class="mts-card" data-testid="metrics-empty">
      <EmptyState :title="t('metricsEmpty')" :description="t('metricsEmptyDesc')" />
    </div>
    <div v-else class="space-y-2" data-testid="metrics-list">
      <div
        v-for="fam in filtered"
        :key="fam.name"
        class="mts-card overflow-hidden"
        :data-testid="`metrics-family-${fam.name}`"
      >
        <button
          type="button"
          class="flex w-full items-start justify-between gap-3 px-3 py-2 text-left hover:bg-slate-50 dark:hover:bg-slate-800/50"
          :aria-expanded="expanded[fam.name] ? 'true' : 'false'"
          @click="toggle(fam.name)"
        >
          <div>
            <p class="font-mono text-sm font-medium text-slate-800 dark:text-slate-100">{{ fam.name }}</p>
            <p class="text-xs mts-muted">
              <span v-if="fam.type" class="mr-2 uppercase">{{ fam.type }}</span>
              {{ fam.help || t('emptyValue') }}
            </p>
          </div>
          <span class="text-xs mts-muted whitespace-nowrap">{{ formatMessage(t('metricsSampleCount'), { count: fam.samples.length }) }}</span>
        </button>
        <div v-if="expanded[fam.name]" class="mts-table-wrap border-t border-slate-100 dark:border-slate-800">
          <table class="min-w-full text-left text-xs">
            <thead class="bg-slate-50 text-slate-500 dark:bg-slate-900/50 dark:text-slate-400">
              <tr>
                <th class="px-3 py-1.5 font-medium">{{ t('metricsColLabels') }}</th>
                <th class="px-3 py-1.5 font-medium">{{ t('metricsColValue') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr
                v-for="(s, i) in fam.samples"
                :key="i"
                class="border-t border-slate-100 dark:border-slate-800"
              >
                <td class="px-3 py-1.5 font-mono text-slate-600 dark:text-slate-300">
                  {{ formatSampleLabels(s.labels) || t('emptyValue') }}
                </td>
                <td class="px-3 py-1.5 font-mono text-slate-800 dark:text-slate-100">{{ s.value }}</td>
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
