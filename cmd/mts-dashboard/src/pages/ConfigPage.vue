<script setup lang="ts">
import { ref, onMounted, computed, watch } from 'vue'
import { useRoute } from 'vue-router'
import { useHashScroll } from '@/composables/useHashScroll'
import { parseConfigPrefill, configFormToPrefill } from '@/utils/routePrefill'
import { apiGet, apiPost, getAdminToken, setAdminToken, getDataToken, setDataToken } from '@/api/client'
import { useAuth } from '@/composables/useAuth'
import { useNotify } from '@/composables/useNotify'
import { formatCaughtError } from '@/utils/apiError'
import { useI18n } from '@/composables/useI18n'
import { formatMessage } from '@/utils/formatMessage'
import PermissionDenied from '@/components/PermissionDenied.vue'
import ActionResultBanner from '@/components/ActionResultBanner.vue'
import VirtualTable from '@/components/VirtualTable.vue'
import { makeActionResult, type ActionResult } from '@/utils/actionResult'
import {
  buildConfigSchemaExport,
  buildEffectiveConfigExport,
  buildErrorCodesExport,
  formatConfigPretty,
} from '@/utils/configExport'
import { downloadJSON, stampFilename } from '@/utils/download'
import { copyText } from '@/utils/clipboard'
import { RefreshCw, CheckCircle, Download, Copy } from 'lucide-vue-next'

interface ConfigResponse { config: Record<string, unknown> }
interface ValidateResponse { ok: boolean; error?: string }
interface ReloadResponse { ok: boolean; fields: string[] }
interface ErrorCodeSpec { code: string; http_status: number; grpc_code: string; description: string }
interface ErrorCodesResponse { codes: ErrorCodeSpec[] }
interface SchemaField { name: string; description: string }
interface SchemaResponse { fields: SchemaField[] }

useHashScroll()
const route = useRoute()
const { isAdmin } = useAuth()
const { t } = useI18n()
const { success, error: notifyError } = useNotify()
const config = ref<Record<string, unknown> | null>(null)
const validateResult = ref<ValidateResponse | null>(null)
const reloadResult = ref<ReloadResponse | null>(null)
const errorCodes = ref<ErrorCodeSpec[]>([])
const schemaFields = ref<SchemaField[]>([])
const schemaFilter = ref('')
const SCHEMA_ROW_HEIGHT = 40
const ERROR_ROW_HEIGHT = 44
const CONFIG_LIST_HEIGHT = 320
const errorCodeFilter = ref('')
const loadError = ref('')
const actionResult = ref<ActionResult | null>(null)
const adminTokenInput = ref(getAdminToken())
const dataTokenInput = ref(getDataToken())

const filteredSchema = computed(() => {
  const q = schemaFilter.value.trim().toLowerCase()
  if (!q) return schemaFields.value
  return schemaFields.value.filter((f) =>
    f.name.toLowerCase().includes(q) || (f.description || '').toLowerCase().includes(q),
  )
})

const filteredErrorCodes = computed(() => {
  const q = errorCodeFilter.value.trim().toLowerCase()
  if (!q) return errorCodes.value
  return errorCodes.value.filter((ec) =>
    ec.code.toLowerCase().includes(q)
    || String(ec.http_status).includes(q)
    || (ec.grpc_code || '').toLowerCase().includes(q)
    || (ec.description || '').toLowerCase().includes(q),
  )
})

async function loadConfig() {
  const [cfgData, ecData, schemaData] = await Promise.all([
    apiGet<ConfigResponse>('/api/v1/admin/config/effective'),
    apiGet<ErrorCodesResponse>('/api/v1/admin/error-codes'),
    apiGet<SchemaResponse>('/api/v1/admin/config/schema').catch(() => ({ fields: [] as SchemaField[] })),
  ])
  config.value = cfgData.config
  errorCodes.value = ecData.codes ?? []
  schemaFields.value = schemaData.fields ?? []
}

function applyConfigPrefillFromRoute() {
  if (!isAdmin.value) return
  const pre = parseConfigPrefill(route.query as Record<string, unknown>, route.hash)
  let changed = false
  if (pre.schema_q != null && schemaFilter.value !== pre.schema_q) {
    schemaFilter.value = pre.schema_q
    changed = true
  }
  if (pre.error_q != null && errorCodeFilter.value !== pre.error_q) {
    errorCodeFilter.value = pre.error_q
    changed = true
  }
  if (changed) success(t.value('configPrefillApplied'))
}

async function copyConfigShareLink() {
  const path = configFormToPrefill({
    schema_q: schemaFilter.value,
    error_q: errorCodeFilter.value,
    section: schemaFilter.value ? 'config-schema' : errorCodeFilter.value ? 'config-error-codes' : 'config-effective',
  })
  const url = `${window.location.origin}${path}`
  const res = await copyText(url)
  if (res.ok) success(t.value('configShareCopied'))
  else notifyError(res.error || t.value('failed'))
}

onMounted(async () => {
  if (!isAdmin.value) return
  try {
    await loadConfig()
  } catch (e) {
    loadError.value = formatCaughtError(e)
  }
  applyConfigPrefillFromRoute()
})

watch(
  () => route.fullPath,
  (path, prev) => {
    if (!isAdmin.value) return
    if (prev != null && path !== prev) applyConfigPrefillFromRoute()
  },
)

async function handleValidate() {
  actionResult.value = null
  validateResult.value = null
  try {
    validateResult.value = await apiPost<ValidateResponse>('/api/v1/admin/config/validate', { config: config.value })
    if (validateResult.value.ok) {
      actionResult.value = makeActionResult('ok', t.value('configValidateOk'))
      success(t.value('configValidateOk'))
    } else {
      const msg = formatMessage(t.value('configValidateFail'), { error: validateResult.value.error || '' })
      actionResult.value = makeActionResult('error', msg)
      notifyError(msg)
    }
  } catch (e) {
    const msg = formatCaughtError(e)
    actionResult.value = makeActionResult('error', msg)
    notifyError(msg)
  }
}

async function handleReload() {
  actionResult.value = null
  reloadResult.value = null
  try {
    reloadResult.value = await apiPost<ReloadResponse>('/api/v1/admin/config/reload')
    actionResult.value = makeActionResult('ok', reloadResult.value.fields?.length ? formatMessage(t.value('configReloadOkFields'), { fields: reloadResult.value.fields.join(', ') }) : t.value('configReloadOk'))
    await loadConfig()
    success(t.value('configReloadToast'))
  } catch (e) {
    const msg = formatCaughtError(e)
    actionResult.value = makeActionResult('error', msg)
    notifyError(msg)
  }
}

function saveServiceTokens() {
  setAdminToken(adminTokenInput.value)
  setDataToken(dataTokenInput.value)
  success(t.value('configTokenSaved'))
}

function clearServiceTokens() {
  adminTokenInput.value = ''
  dataTokenInput.value = ''
  setAdminToken('')
  setDataToken('')
  actionResult.value = makeActionResult('ok', t.value('configTokenCleared'))
  success(t.value('configTokenCleared'))
}

function exportEffective() {
  if (!config.value) {
    notifyError(t.value('configExportEmpty'))
    return
  }
  downloadJSON(stampFilename('mts-config-effective', 'json'), buildEffectiveConfigExport(config.value))
  success(t.value('configExported'))
}

async function copyEffective() {
  if (!config.value) {
    notifyError(t.value('configCopyEmpty'))
    return
  }
  const res = await copyText(formatConfigPretty(config.value))
  if (res.ok) success(t.value('configCopied'))
  else notifyError(res.error || t.value('failed'))
}

function exportSchema() {
  if (!filteredSchema.value.length) {
    notifyError(t.value('configExportEmpty'))
    return
  }
  downloadJSON(stampFilename('mts-config-schema', 'json'), buildConfigSchemaExport(filteredSchema.value))
  success(t.value('configExported'))
}

function exportErrorCodes() {
  if (!filteredErrorCodes.value.length) {
    notifyError(t.value('configExportEmpty'))
    return
  }
  downloadJSON(stampFilename('mts-error-codes', 'json'), buildErrorCodesExport(filteredErrorCodes.value))
  success(t.value('configExported'))
}

function statusLabel(httpStatus: number): string {
  if (httpStatus >= 200 && httpStatus < 300) return t.value('configHttpOk')
  if (httpStatus >= 400 && httpStatus < 500) return t.value('configHttpClientErr')
  if (httpStatus >= 500) return t.value('configHttpServerErr')
  return ''
}
</script>

<template>
  <PermissionDenied v-if="!isAdmin" />
  <div v-else class="space-y-6" data-testid="config-page">
    <div class="flex flex-wrap items-center justify-between gap-3">
      <div>
        <h1 class="mts-title">{{ t('configTitle') }}</h1>
        <p class="text-xs mts-muted">{{ t('configDesc') }}</p>
      </div>
      <div class="flex flex-wrap gap-2">
        <button type="button" class="mts-btn" data-testid="config-share-link" @click="copyConfigShareLink">
          {{ t('configShareLink') }}
        </button>
        <button type="button" class="mts-btn" data-testid="config-export-effective" :disabled="!config" @click="exportEffective">
          <Download class="h-3.5 w-3.5" /> {{ t('configExportEffective') }}
        </button>
        <button type="button" class="mts-btn" data-testid="config-copy-effective" :disabled="!config" @click="copyEffective">
          <Copy class="h-3.5 w-3.5" /> {{ t('configCopyEffective') }}
        </button>
      </div>
    </div>
    <ActionResultBanner
      v-if="loadError"
      kind="error"
      :message="loadError"
      @dismiss="loadError = ''"
    />
    <ActionResultBanner :result="actionResult" @dismiss="actionResult = null" />

    <div class="mts-panel">
      <h2 class="mb-2 text-sm font-semibold text-slate-800 dark:text-slate-100">{{ t('configTokenTitle') }}</h2>
      <p class="mb-4 text-xs mts-muted">{{ t('configTokenHint') }}</p>
      <div class="grid gap-3 sm:grid-cols-2">
        <div>
          <label class="mb-1 block text-xs mts-muted">{{ t('configAdminTokenLabel') }}</label>
          <input v-model="adminTokenInput" type="password" class="mts-input"  :placeholder="t('optional')" />
        </div>
        <div>
          <label class="mb-1 block text-xs mts-muted">{{ t('configDataTokenLabel') }}</label>
          <input v-model="dataTokenInput" type="password" class="mts-input"  :placeholder="t('optional')" />
        </div>
      </div>
      <div class="mt-3 flex gap-2">
        <button class="mts-btn-primary" @click="saveServiceTokens">{{ t('configTokenSave') }}</button>
        <button class="mts-btn" @click="clearServiceTokens">{{ t('configTokenClear') }}</button>
      </div>
    </div>

    <div class="flex flex-wrap gap-3">
      <button class="mts-btn-primary" @click="handleValidate"><CheckCircle class="h-4 w-4" />{{ t('configValidate') }}</button>
      <button class="mts-btn-primary" @click="handleReload"><RefreshCw class="h-4 w-4" />{{ t('configReload') }}</button>
    </div>
    <div v-if="validateResult" :class="validateResult.ok ? 'mts-alert-ok' : 'mts-alert-error'">
      <p v-if="validateResult.ok">{{ t('configValidateOk') }}</p>
      <p v-else>{{ formatMessage(t('configValidateFail'), { error: validateResult.error || '' }) }}</p>
    </div>
    <div v-if="reloadResult" class="mts-alert-ok">
      <p>{{ reloadResult.fields?.length ? formatMessage(t('configReloadOkFields'), { fields: reloadResult.fields.join(', ') }) : t('configReloadOk') }}</p>
    </div>

    <div v-if="config" class="mts-panel">
      <h2 class="mb-4 text-sm font-semibold text-slate-800 dark:text-slate-100">{{ t('configEffective') }}</h2>
      <pre class="max-h-96 overflow-auto rounded-lg bg-slate-900 p-4 text-xs text-green-400" id="config-effective" data-testid="config-effective-json">{{ JSON.stringify(config, null, 2) }}</pre>
    </div>

    <div class="mts-panel">
      <div class="mb-3 flex flex-wrap items-center justify-between gap-2">
        <h2 class="text-sm font-semibold text-slate-800 dark:text-slate-100">{{ t('configSchema') }}</h2>
        <div class="flex flex-wrap items-center gap-2">
          <input v-model="schemaFilter" class="mts-input max-w-xs text-xs" data-testid="config-schema-filter" :placeholder="t('configSchemaFilter')" />
          <button type="button" class="mts-btn" data-testid="config-export-schema" :disabled="!filteredSchema.length" @click="exportSchema">
            <Download class="h-3.5 w-3.5" /> {{ t('configExportSchema') }}
          </button>
        </div>
      </div>
      <div class="overflow-hidden rounded-lg border border-slate-100 dark:border-slate-800 scroll-mt-20" id="config-schema" data-testid="config-schema-table">
        <div
          class="grid grid-cols-[minmax(10rem,0.9fr)_minmax(14rem,1.4fr)] border-b border-slate-200 text-left text-[11px] uppercase mts-muted dark:border-slate-700"
          data-testid="config-schema-header"
        >
          <div class="px-2 py-2">{{ t('configColName') }}</div>
          <div class="px-2 py-2">{{ t('configColDescription') }}</div>
        </div>
        <div v-if="!filteredSchema.length" class="px-2 py-3 text-xs mts-muted" data-testid="config-schema-empty">
          {{ t('configSchemaEmpty') }}
        </div>
        <VirtualTable
          v-else
          :items="filteredSchema"
          :row-height="SCHEMA_ROW_HEIGHT"
          :height="Math.min(CONFIG_LIST_HEIGHT, Math.max(160, filteredSchema.length * SCHEMA_ROW_HEIGHT))"
          data-testid="config-schema-virtual-list"
        >
          <template #default="{ item: f }">
            <div
              class="grid h-full grid-cols-[minmax(10rem,0.9fr)_minmax(14rem,1.4fr)] items-center border-b border-slate-100 dark:border-slate-800"
              :data-testid="`config-schema-row-${f.name}`"
            >
              <div class="truncate px-2 font-mono text-xs" :title="f.name">{{ f.name }}</div>
              <div class="truncate px-2 text-xs mts-muted" :title="f.description">{{ f.description }}</div>
            </div>
          </template>
        </VirtualTable>
        <p
          v-if="filteredSchema.length"
          class="border-t border-slate-100 px-3 py-1.5 text-[11px] mts-muted dark:border-slate-800"
          data-testid="config-schema-virtual-hint"
        >
          {{ t('configSchemaVirtualHint') }}
        </p>
      </div>
    </div>

    <div class="mts-panel">
      <div class="mb-2 flex flex-wrap items-center justify-between gap-2">
        <h2 class="text-sm font-semibold text-slate-800 dark:text-slate-100">{{ t('configErrorCodes') }}</h2>
        <div class="flex flex-wrap items-center gap-2">
          <input
            v-model="errorCodeFilter"
            class="mts-input max-w-xs text-xs"
            data-testid="config-error-codes-filter"
            :placeholder="t('configErrorCodesFilter')"
          />
          <button type="button" class="mts-btn" data-testid="config-export-error-codes" :disabled="!filteredErrorCodes.length" @click="exportErrorCodes">
            <Download class="h-3.5 w-3.5" /> {{ t('configExportErrorCodes') }}
          </button>
        </div>
      </div>
      <div class="overflow-hidden rounded-lg border border-slate-100 dark:border-slate-800 scroll-mt-20" id="config-error-codes" data-testid="config-error-codes-table">
        <div
          class="grid grid-cols-[minmax(8rem,0.9fr)_minmax(7rem,0.7fr)_minmax(7rem,0.7fr)_minmax(12rem,1.4fr)] border-b border-slate-200 text-left dark:border-slate-700"
          data-testid="config-error-codes-header"
        >
          <div class="px-2 py-2 text-xs font-medium mts-muted">{{ t('configColCode') }}</div>
          <div class="px-2 py-2 text-xs font-medium mts-muted">{{ t('configColHTTP') }}</div>
          <div class="px-2 py-2 text-xs font-medium mts-muted">{{ t('configColGRPC') }}</div>
          <div class="px-2 py-2 text-xs font-medium mts-muted">{{ t('configColDescription') }}</div>
        </div>
        <div v-if="!filteredErrorCodes.length" class="px-2 py-3 text-xs mts-muted" data-testid="config-error-codes-empty">
          {{ t('configErrorCodesEmpty') }}
        </div>
        <VirtualTable
          v-else
          :items="filteredErrorCodes"
          :row-height="ERROR_ROW_HEIGHT"
          :height="Math.min(CONFIG_LIST_HEIGHT, Math.max(176, filteredErrorCodes.length * ERROR_ROW_HEIGHT))"
          data-testid="config-error-codes-virtual-list"
        >
          <template #default="{ item: ec }">
            <div
              class="grid h-full grid-cols-[minmax(8rem,0.9fr)_minmax(7rem,0.7fr)_minmax(7rem,0.7fr)_minmax(12rem,1.4fr)] items-center border-b border-slate-100 dark:border-slate-800"
              :data-testid="`config-error-code-row-${ec.code}`"
            >
              <div class="truncate px-2 font-mono text-xs" :title="ec.code">{{ ec.code }}</div>
              <div class="px-2">
                <span class="rounded bg-slate-100 px-1.5 py-0.5 text-xs dark:bg-slate-800">{{ ec.http_status }} {{ statusLabel(ec.http_status) }}</span>
              </div>
              <div class="truncate px-2 font-mono text-xs" :title="ec.grpc_code">{{ ec.grpc_code }}</div>
              <div class="truncate px-2 text-xs mts-muted" :title="ec.description">{{ ec.description }}</div>
            </div>
          </template>
        </VirtualTable>
        <p
          v-if="filteredErrorCodes.length"
          class="border-t border-slate-100 px-3 py-1.5 text-[11px] mts-muted dark:border-slate-800"
          data-testid="config-error-codes-virtual-hint"
        >
          {{ t('configErrorCodesVirtualHint') }}
        </p>
      </div>
    </div>
  </div>
</template>
