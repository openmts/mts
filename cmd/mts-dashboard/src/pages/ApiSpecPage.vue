<script setup lang="ts">
import { ref, computed, inject, onMounted, watch, type ComputedRef } from 'vue'
import { useRoute } from 'vue-router'
import { useHashScroll } from '@/composables/useHashScroll'
import { parseApiSpecPrefill, apiSpecFormToPrefill } from '@/utils/routePrefill'
import { apiGet } from '@/api/client'
import { useAuth } from '@/composables/useAuth'
import PermissionDenied from '@/components/PermissionDenied.vue'
import AdminOpLastChip from '@/components/AdminOpLastChip.vue'
import { useNotify } from '@/composables/useNotify'
import { useNotifyAdminBusy } from '@/composables/useNotifyAdminBusy'
import { formatCaughtError } from '@/utils/apiError'
import { useI18n } from '@/composables/useI18n'
import { formatMessage } from '@/utils/formatMessage'
import { Download, RefreshCw, Search } from 'lucide-vue-next'
import VirtualTable from '@/components/VirtualTable.vue'
import EmptyState from '@/components/EmptyState.vue'
import ActionResultBanner from '@/components/ActionResultBanner.vue'
import PartialErrorBanner from '@/components/PartialErrorBanner.vue'
import { apiSpecToMarkdown, buildApiSpecExport } from '@/utils/apiSpecExport'
import { stampFilename } from '@/utils/download'
import { useExportJob } from '@/composables/useExportJob'
import ExportJobBanner from '@/components/ExportJobBanner.vue'
import { copyText } from '@/utils/clipboard'


interface APIEndpoint {
  method: string
  path: string
  auth?: string
  description?: string
}
interface APINamespace {
  name: string
  base_path?: string
  endpoints: APIEndpoint[]
}
interface APISpecResponse {
  version?: string
  namespaces: APINamespace[]
}

const { isAdmin } = useAuth()
const adminOpBusySummary = inject<ComputedRef<{ lastSummary?: string; lastError?: string; lastOk?: boolean | null }> | undefined>('adminOpBusySummary', undefined)
const apiSpecAdminLastLabel = computed(() => (adminOpBusySummary?.value?.lastSummary || '').trim())
const apiSpecAdminLastErrorDetail = computed(() => {
  if (adminOpBusySummary?.value?.lastOk !== false) return ''
  return (adminOpBusySummary?.value?.lastError || '').trim()
})
const route = useRoute()
useHashScroll()
const { success, info, error: notifyError, warn } = useNotify()
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
const { t, locale } = useI18n()
const loading = ref(false)
const loadError = ref('')
const version = ref('')
const namespaces = ref<APINamespace[]>([])
const q = ref('')
const nsFilter = ref('')
const EP_ROW_HEIGHT = 40
const EP_LIST_HEIGHT = 360

async function load() {
  if (!isAdmin.value) return
  loading.value = true
  try {
    const data = await apiGet<APISpecResponse>('/api/v1/admin/api-spec')
    version.value = data.version || 'v1'
    namespaces.value = data.namespaces || []
    if (!nsFilter.value && namespaces.value.length) {
      nsFilter.value = namespaces.value[0].name
    }
    loadError.value = ''
  } catch (e) {
    const msg = formatCaughtError(e)
    if (namespaces.value.length) {
      loadError.value = msg
      notifyMaybeAdminBusy(msg, e, { treatLocalBusy: true })
    } else {
      loadError.value = msg
      notifyError(msg)
      notifyMaybeAdminBusy(msg, e)
    }
  } finally {
    loading.value = false
  }
}


function applyApiSpecPrefillFromRoute() {
  if (!isAdmin.value) return
  const pre = parseApiSpecPrefill(route.query as Record<string, unknown>)
  let changed = false
  if (pre.ns != null && nsFilter.value !== pre.ns) {
    nsFilter.value = pre.ns
    changed = true
  }
  if (pre.q != null && q.value !== pre.q) {
    q.value = pre.q
    changed = true
  }
  if (changed) success(t.value('apiSpecPrefillApplied'))
}

async function copyApiSpecShareLink() {
  const path = apiSpecFormToPrefill({
    ns: nsFilter.value || undefined,
    q: q.value,
  })
  const url = `${window.location.origin}${path}`
  const res = await copyText(url)
  if (res.ok) success(t.value('apiSpecShareCopied'))
  else notifyError(res.error || t.value('failed'))
}

onMounted(() => {
  applyApiSpecPrefillFromRoute()
  void load()
})

watch(
  () => route.fullPath,
  (path, prev) => {
    if (!isAdmin.value) return
    if (prev != null && path !== prev) applyApiSpecPrefillFromRoute()
  },
)

const filtered = computed(() => {
  const text = q.value.trim().toLowerCase()
  return namespaces.value
    .filter((ns) => !nsFilter.value || ns.name === nsFilter.value)
    .map((ns) => ({
      ...ns,
      endpoints: (ns.endpoints || []).filter((ep) => {
        if (!text) return true
        return (
          ep.method.toLowerCase().includes(text) ||
          ep.path.toLowerCase().includes(text) ||
          (ep.description || '').toLowerCase().includes(text) ||
          (ep.auth || '').toLowerCase().includes(text)
        )
      }),
    }))
    .filter((ns) => ns.endpoints.length || !text)
})

const totalEndpoints = computed(() =>
  namespaces.value.reduce((n, ns) => n + (ns.endpoints?.length || 0), 0),
)

const exportLocale = computed(() => (locale.value === 'en' ? 'en' : 'zh') as 'zh' | 'en')

async function exportJSON() {
  if (!namespaces.value.length) {
    warn(t.value('apiSpecExportEmpty'))
    return
  }
  if (exportBusy.value) return
  const ns = namespaces.value.slice()
  const outcome = await runJSONExport({
    label: 'JSON',
    filename: stampFilename('mts-api-spec', 'json'),
    total: ns.length,
    build: async ({ isCancelled, progress }) => {
      progress(0, ns.length)
      for (let i = 0; i < ns.length; i++) {
        if (isCancelled()) return null
        const done = i + 1
        if (done === ns.length || done % 20 === 0) {
          progress(done, ns.length)
          if (done < ns.length) await new Promise((r) => setTimeout(r, 0))
        }
      }
      return buildApiSpecExport({ version: version.value, namespaces: ns })
    },
  })
  if (outcome === 'done') success(t.value('apiSpecExported'))
  else if (outcome === 'cancelled') info(t.value('exportCancelledToast'))
  else if (outcome === 'error') notifyError(exportJob.value.error || t.value('failed'))
}

async function exportMarkdown() {
  if (!namespaces.value.length) {
    warn(t.value('apiSpecExportEmpty'))
    return
  }
  if (exportBusy.value) return
  const ns = namespaces.value.slice()
  const outcome = await runTextExport({
    label: 'Markdown',
    filename: stampFilename('mts-api-spec', 'md'),
    mime: 'text/markdown;charset=utf-8',
    total: ns.length,
    build: async ({ isCancelled, progress }) => {
      progress(0, ns.length)
      for (let i = 0; i < ns.length; i++) {
        if (isCancelled()) return null
        const done = i + 1
        if (done === ns.length || done % 20 === 0) {
          progress(done, ns.length)
          if (done < ns.length) await new Promise((r) => setTimeout(r, 0))
        }
      }
      return apiSpecToMarkdown({ version: version.value, namespaces: ns }, exportLocale.value)
    },
  })
  if (outcome === 'done') success(t.value('apiSpecExported'))
  else if (outcome === 'cancelled') info(t.value('exportCancelledToast'))
  else if (outcome === 'error') notifyError(exportJob.value.error || t.value('failed'))
}
</script>

<template>
  <PermissionDenied v-if="!isAdmin" />
  <div v-else class="space-y-4" data-testid="api-spec-page">
    <div class="flex flex-wrap items-center justify-between gap-2">
      <div>
        <h1 class="mts-title" data-testid="api-spec-title">{{ t('apiSpec') }}</h1>
        <div class="mt-1 flex flex-wrap items-center gap-2">
          <p class="text-xs mts-muted">{{ formatMessage(t('apiSpecDesc'), { version: version || t('emptyValue'), count: totalEndpoints }) }}</p>
          <AdminOpLastChip
            v-if="apiSpecAdminLastLabel"
            :label="apiSpecAdminLastLabel"
            :last-ok="adminOpBusySummary?.lastOk"
            :last-error="apiSpecAdminLastErrorDetail"
            test-id="api-spec-admin-last"
            show-copy
            copy-test-id="api-spec-admin-last-copy"
            error-test-id="api-spec-admin-last-error"
          />

        </div>
      </div>
      <div class="flex flex-wrap gap-2">
        <ExportJobBanner class="w-full basis-full" :job="exportJob" :retryable="canRetryExport" @cancel="cancelExport" @retry="retryLastExport" @dismiss="resetExport" />
        <button
          type="button"
          class="mts-btn"
          data-testid="api-spec-export-json"
          :disabled="exportBusy || (!namespaces.length)"
          @click="exportJSON"
        >
          <Download class="h-3.5 w-3.5" /> {{ t('apiSpecExportJSON') }}
        </button>
        <button
          type="button"
          class="mts-btn"
          data-testid="api-spec-export-md"
          :disabled="exportBusy || (!namespaces.length)"
          @click="exportMarkdown"
        >
          <Download class="h-3.5 w-3.5" /> {{ t('apiSpecExportMarkdown') }}
        </button>
        <button type="button" class="mts-btn" data-testid="api-spec-share-link" @click="copyApiSpecShareLink">
          {{ t('apiSpecShareLink') }}
        </button>
        <button class="mts-btn" data-testid="api-spec-refresh" :disabled="loading" :aria-busy="loading ? 'true' : undefined" @click="load">
          <RefreshCw class="h-3.5 w-3.5" :class="loading ? 'animate-spin' : ''" /> {{ t('refresh') }}
        </button>
      </div>
    </div>

        <div v-if="loadError && !namespaces.length" data-testid="api-spec-error">
      <ActionResultBanner
        kind="error"
        :message="loadError"
        retryable
        data-testid="api-spec-load-error"
        @retry="load"
        @dismiss="loadError = ''"
      />
    </div>
    <PartialErrorBanner
      v-else-if="loadError && namespaces.length"
      :message="`${t('apiSpecRefreshFailed')}：${loadError}`"
      test-id="api-spec-refresh-error"
      @retry="load"
      @dismiss="loadError = ''"
    />
<div id="api-spec-filters" class="grid gap-3 scroll-mt-20 md:grid-cols-3" data-testid="api-spec-filters">
      <label class="text-xs mts-muted md:col-span-1">{{ t('apiSpecNamespace') }}
        <select v-model="nsFilter" class="mts-input mt-1" data-testid="api-spec-ns-filter">
          <option value="">{{ t('apiSpecAll') }}</option>
          <option v-for="ns in namespaces" :key="ns.name" :value="ns.name">{{ ns.name }}</option>
        </select>
      </label>
      <label class="text-xs mts-muted md:col-span-2">{{ t('apiSpecSearch') }}
        <div class="relative mt-1">
          <Search class="pointer-events-none absolute left-2 top-2.5 h-3.5 w-3.5 text-slate-400" />
          <input v-model="q" class="mts-input pl-8" data-testid="api-spec-search" :placeholder="t('apiSpecSearchPlaceholder')" />
        </div>
      </label>
    </div>

    <EmptyState
      v-if="!loading && !filtered.length && !(loadError && !namespaces.length)"
      data-testid="api-spec-empty"
      :title="t('apiSpecFilterEmpty')"
      :description="t('apiSpecFilterEmptyDesc')"
    >
      <template #action>
        <button type="button" class="mts-btn-primary" data-testid="api-spec-clear-filters" @click="q = ''; nsFilter = ''">{{ t('clearFilters') }}</button>
      </template>
    </EmptyState>
    <div v-for="ns in filtered" :key="ns.name" class="mts-panel" :data-testid="`api-spec-ns-${ns.name}`">
      <div class="mb-3 flex flex-wrap items-center justify-between gap-2">
        <h2 class="text-sm font-semibold text-slate-800 dark:text-slate-100">{{ ns.name }}</h2>
        <span class="text-xs mts-muted">{{ ns.base_path || t('emptyValue') }} · {{ formatMessage(t('apiSpecEndpoints'), { count: ns.endpoints.length }) }}</span>
      </div>
      <div v-if="ns.endpoints.length" class="overflow-hidden rounded-lg border border-slate-100 dark:border-slate-800" :data-testid="`api-spec-ep-table-${ns.name}`">
        <div class="grid grid-cols-[5rem_minmax(8rem,1.2fr)_minmax(5rem,0.7fr)_minmax(8rem,1.2fr)] border-b border-slate-200 px-2 py-2 text-left text-[11px] uppercase mts-muted dark:border-slate-700">
          <span>{{ t('apiSpecColMethod') }}</span>
          <span>{{ t('apiSpecColPath') }}</span>
          <span>{{ t('apiSpecColAuth') }}</span>
          <span>{{ t('apiSpecColDescription') }}</span>
        </div>
        <VirtualTable
          :items="ns.endpoints"
          :row-height="EP_ROW_HEIGHT"
          :height="Math.min(EP_LIST_HEIGHT, Math.max(160, ns.endpoints.length * EP_ROW_HEIGHT))"
          :data-testid="`api-spec-ep-virtual-list-${ns.name}`"
        >
          <template #default="{ item: ep, index }">
            <div
              class="grid h-full grid-cols-[5rem_minmax(8rem,1.2fr)_minmax(5rem,0.7fr)_minmax(8rem,1.2fr)] items-center border-b border-slate-100 px-2 text-xs dark:border-slate-800"
              :data-testid="`api-spec-ep-row-${ns.name}-${index}`"
            >
              <span class="font-mono font-semibold">{{ ep.method }}</span>
              <span class="truncate font-mono" :title="ep.path">{{ ep.path }}</span>
              <span class="mts-muted truncate">{{ ep.auth || t('emptyValue') }}</span>
              <span class="truncate" :title="ep.description || ''">{{ ep.description || t('emptyValue') }}</span>
            </div>
          </template>
        </VirtualTable>
        <p class="border-t border-slate-100 px-3 py-1.5 text-[11px] mts-muted dark:border-slate-800" :data-testid="`api-spec-ep-virtual-hint-${ns.name}`">
          {{ t('apiSpecEpVirtualHint') }}
        </p>
      </div>
      <EmptyState v-else compact :title="t('apiSpecEpEmpty')" :description="t('apiSpecEpEmptyDesc')" />
    </div>
  </div>
</template>
