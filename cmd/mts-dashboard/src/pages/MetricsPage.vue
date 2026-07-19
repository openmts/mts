<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
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
import { Activity, RefreshCw } from 'lucide-vue-next'

const { isAdmin } = useAuth()
const { t } = useI18n()
const loading = ref(false)
const loadError = ref('')
const raw = ref('')
const families = ref<PrometheusFamily[]>([])
const q = ref('')
const expanded = ref<Record<string, boolean>>({})
const lastRefreshed = ref('')

const filtered = computed(() => filterPrometheusFamilies(families.value, q.value))
const summary = computed(() => summarizeFamilies(filtered.value))

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
  } finally {
    loading.value = false
  }
}

function toggle(name: string) {
  expanded.value = { ...expanded.value, [name]: !expanded.value[name] }
}

onMounted(() => { void load() })
</script>

<template>
  <PermissionDenied v-if="!isAdmin" />
  <div v-else class="space-y-4">
    <div class="flex flex-wrap items-start justify-between gap-3">
      <div>
        <h1 class="mts-title flex items-center gap-2">
          <Activity class="h-5 w-5" />
          {{ t('metricsTitle') }}
        </h1>
        <p class="text-xs mts-muted">
          {{ t('metricsDesc') }}
          <span v-if="lastRefreshed">{{ formatMessage(t('metricsRefreshedAt'), { time: lastRefreshed }) }}</span>
        </p>
      </div>
      <button class="mts-btn" :disabled="loading" @click="load">
        <RefreshCw class="h-3.5 w-3.5" :class="loading ? 'animate-spin' : ''" /> {{ t('refresh') }}
      </button>
    </div>

    <ActionResultBanner v-if="loadError" kind="error" :message="loadError" @dismiss="loadError = ''" />

    <div class="grid gap-3 sm:grid-cols-2">
      <div class="mts-card p-3">
        <p class="text-xs mts-muted">{{ t('metricsFamilies') }}</p>
        <p class="text-xl font-semibold text-slate-800 dark:text-slate-100">{{ summary.families }}</p>
      </div>
      <div class="mts-card p-3">
        <p class="text-xs mts-muted">{{ t('metricsSamples') }}</p>
        <p class="text-xl font-semibold text-slate-800 dark:text-slate-100">{{ summary.samples }}</p>
      </div>
    </div>

    <input v-model="q" class="mts-input max-w-xl text-sm" :placeholder="t('metricsFilterPlaceholder')" />

    <div v-if="!loading && !filtered.length" class="mts-card">
      <EmptyState :title="t('metricsEmpty')" :description="t('metricsEmptyDesc')" />
    </div>
    <div v-else class="space-y-2">
      <div
        v-for="fam in filtered"
        :key="fam.name"
        class="mts-card overflow-hidden"
      >
        <button
          type="button"
          class="flex w-full items-start justify-between gap-3 px-3 py-2 text-left hover:bg-slate-50 dark:hover:bg-slate-800/50"
          @click="toggle(fam.name)"
        >
          <div>
            <p class="font-mono text-sm font-medium text-slate-800 dark:text-slate-100">{{ fam.name }}</p>
            <p class="text-xs mts-muted">
              <span v-if="fam.type" class="mr-2 uppercase">{{ fam.type }}</span>
              {{ fam.help || '—' }}
            </p>
          </div>
          <span class="text-xs mts-muted whitespace-nowrap">{{ fam.samples.length }} samples</span>
        </button>
        <div v-if="expanded[fam.name]" class="border-t border-slate-100 dark:border-slate-800">
          <table class="min-w-full text-left text-xs">
            <thead class="bg-slate-50 text-slate-500 dark:bg-slate-900/50 dark:text-slate-400">
              <tr>
                <th class="px-3 py-1.5 font-medium">labels</th>
                <th class="px-3 py-1.5 font-medium">value</th>
              </tr>
            </thead>
            <tbody>
              <tr
                v-for="(s, i) in fam.samples"
                :key="i"
                class="border-t border-slate-100 dark:border-slate-800"
              >
                <td class="px-3 py-1.5 font-mono text-slate-600 dark:text-slate-300">
                  {{ formatSampleLabels(s.labels) || '—' }}
                </td>
                <td class="px-3 py-1.5 font-mono text-slate-800 dark:text-slate-100">{{ s.value }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
    </div>

    <details class="mts-card p-3 text-xs">
      <summary class="cursor-pointer text-slate-600 dark:text-slate-300">{{ t('metricsRawPreview') }}</summary>
      <pre class="mt-2 max-h-64 overflow-auto whitespace-pre-wrap break-all font-mono text-[11px] text-slate-500 dark:text-slate-400">{{ raw.slice(0, 8192) }}{{ raw.length > 8192 ? '\n…' : '' }}</pre>
    </details>
  </div>
</template>
