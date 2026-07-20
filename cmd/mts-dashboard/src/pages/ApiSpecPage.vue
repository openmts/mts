<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { apiGet } from '@/api/client'
import { useAuth } from '@/composables/useAuth'
import PermissionDenied from '@/components/PermissionDenied.vue'
import { useNotify } from '@/composables/useNotify'
import { formatCaughtError } from '@/utils/apiError'
import { useI18n } from '@/composables/useI18n'
import { formatMessage } from '@/utils/formatMessage'
import { Download, RefreshCw, Search } from 'lucide-vue-next'
import { apiSpecToMarkdown, buildApiSpecExport } from '@/utils/apiSpecExport'
import { downloadJSON, downloadText, stampFilename } from '@/utils/download'

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
const { success, error: notifyError, warn } = useNotify()
const { t, locale } = useI18n()
const loading = ref(false)
const loadError = ref('')
const version = ref('')
const namespaces = ref<APINamespace[]>([])
const q = ref('')
const nsFilter = ref('')

async function load() {
  if (!isAdmin.value) return
  loading.value = true
  loadError.value = ''
  try {
    const data = await apiGet<APISpecResponse>('/api/v1/admin/api-spec')
    version.value = data.version || 'v1'
    namespaces.value = data.namespaces || []
    if (!nsFilter.value && namespaces.value.length) nsFilter.value = namespaces.value[0].name
  } catch (e) {
    loadError.value = formatCaughtError(e)
    notifyError(loadError.value)
  } finally {
    loading.value = false
  }
}

onMounted(() => { void load() })

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

function exportJSON() {
  if (!namespaces.value.length) {
    warn(t.value('apiSpecExportEmpty'))
    return
  }
  downloadJSON(
    stampFilename('mts-api-spec', 'json'),
    buildApiSpecExport({ version: version.value, namespaces: namespaces.value }),
  )
  success(t.value('apiSpecExported'))
}

function exportMarkdown() {
  if (!namespaces.value.length) {
    warn(t.value('apiSpecExportEmpty'))
    return
  }
  downloadText(
    stampFilename('mts-api-spec', 'md'),
    apiSpecToMarkdown({ version: version.value, namespaces: namespaces.value }, exportLocale.value),
    'text/markdown;charset=utf-8',
  )
  success(t.value('apiSpecExported'))
}
</script>

<template>
  <PermissionDenied v-if="!isAdmin" />
  <div v-else class="space-y-4" data-testid="api-spec-page">
    <div class="flex flex-wrap items-center justify-between gap-2">
      <div>
        <h1 class="mts-title" data-testid="api-spec-title">{{ t('apiSpec') }}</h1>
        <p class="text-xs mts-muted">{{ formatMessage(t('apiSpecDesc'), { version: version || t('emptyValue'), count: totalEndpoints }) }}</p>
      </div>
      <div class="flex flex-wrap gap-2">
        <button
          type="button"
          class="mts-btn"
          data-testid="api-spec-export-json"
          :disabled="!namespaces.length"
          @click="exportJSON"
        >
          <Download class="h-3.5 w-3.5" /> {{ t('apiSpecExportJSON') }}
        </button>
        <button
          type="button"
          class="mts-btn"
          data-testid="api-spec-export-md"
          :disabled="!namespaces.length"
          @click="exportMarkdown"
        >
          <Download class="h-3.5 w-3.5" /> {{ t('apiSpecExportMarkdown') }}
        </button>
        <button class="mts-btn" data-testid="api-spec-refresh" :disabled="loading" :aria-busy="loading ? 'true' : undefined" @click="load">
          <RefreshCw class="h-3.5 w-3.5" :class="loading ? 'animate-spin' : ''" /> {{ t('refresh') }}
        </button>
      </div>
    </div>

    <p v-if="loadError" class="mts-alert-error" data-testid="api-spec-error">{{ loadError }}</p>

    <div class="grid gap-3 md:grid-cols-3">
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

    <div v-for="ns in filtered" :key="ns.name" class="mts-panel" :data-testid="`api-spec-ns-${ns.name}`">
      <div class="mb-3 flex flex-wrap items-center justify-between gap-2">
        <h2 class="text-sm font-semibold text-slate-800 dark:text-slate-100">{{ ns.name }}</h2>
        <span class="text-xs mts-muted">{{ ns.base_path || t('emptyValue') }} · {{ formatMessage(t('apiSpecEndpoints'), { count: ns.endpoints.length }) }}</span>
      </div>
      <div class="overflow-x-auto">
        <table class="w-full text-sm">
          <thead>
            <tr class="border-b border-slate-200 text-left text-[11px] uppercase mts-muted dark:border-slate-700">
              <th class="px-2 py-2">{{ t('apiSpecColMethod') }}</th>
              <th class="px-2 py-2">{{ t('apiSpecColPath') }}</th>
              <th class="px-2 py-2">{{ t('apiSpecColAuth') }}</th>
              <th class="px-2 py-2">{{ t('apiSpecColDescription') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="(ep, i) in ns.endpoints" :key="i" class="border-b border-slate-100 dark:border-slate-800">
              <td class="px-2 py-2 font-mono text-xs font-semibold">{{ ep.method }}</td>
              <td class="px-2 py-2 font-mono text-xs">{{ ep.path }}</td>
              <td class="px-2 py-2 text-xs mts-muted">{{ ep.auth || t('emptyValue') }}</td>
              <td class="px-2 py-2 text-xs">{{ ep.description || t('emptyValue') }}</td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>
  </div>
</template>
