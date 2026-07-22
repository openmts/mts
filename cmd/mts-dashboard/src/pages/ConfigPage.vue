<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount, computed, inject, watch, type ComputedRef } from 'vue'
import { useRoute } from 'vue-router'
import { useHashScroll } from '@/composables/useHashScroll'
import { parseConfigPrefill, configFormToPrefill } from '@/utils/routePrefill'
import { apiGet, apiPost, getAdminToken, setAdminToken, getDataToken, setDataToken } from '@/api/client'
import { useMutationGuard } from '@/composables/useMutationGuard'
import { registerDirtyChecker } from '@/utils/routeDirty'
import { useAuth } from '@/composables/useAuth'
import { useNotify } from '@/composables/useNotify'
import { useNotifyAdminBusy } from '@/composables/useNotifyAdminBusy'
import { useAdminOpBusy } from '@/composables/useAdminOpBusy'
import { actionResultAdminBusyAction, parseAdminOpStatusPayload } from '@/utils/adminOpBusy'
import { formatCaughtError, isCanceledError, isTimeoutError } from '@/utils/apiError'
import { createActionAbort } from '@/utils/actionAbort'
import { useI18n } from '@/composables/useI18n'
import { formatMessage } from '@/utils/formatMessage'
import PermissionDenied from '@/components/PermissionDenied.vue'
import AdminOpLastChip from '@/components/AdminOpLastChip.vue'
import ActionResultBanner from '@/components/ActionResultBanner.vue'
import InFlightBanner from '@/components/InFlightBanner.vue'
import PartialErrorBanner from '@/components/PartialErrorBanner.vue'
import VirtualTable from '@/components/VirtualTable.vue'
import { useActionRetry } from '@/composables/useActionRetry'
import { makeActionResult } from '@/utils/actionResult'
import {
  buildConfigSchemaExport,
  buildEffectiveConfigExport,
  buildErrorCodesExport,
  formatConfigPretty,
} from '@/utils/configExport'
import { stampFilename } from '@/utils/download'
import { useExportJob } from '@/composables/useExportJob'
import ExportJobBanner from '@/components/ExportJobBanner.vue'
import { copyText } from '@/utils/clipboard'
import { RefreshCw, CheckCircle, Download, Copy } from 'lucide-vue-next'
import PasswordInputWithToggle from '@/components/PasswordInputWithToggle.vue'
import EmptyState from '@/components/EmptyState.vue'

interface ConfigResponse {
  config: Record<string, unknown>
  admin_op_busy?: boolean
  op?: string
  started_at_unix?: number
  last?: unknown
}
interface ValidateResponse {
  ok: boolean
  path?: string
  error?: string
  admin_op_busy?: boolean
  op?: string
  started_at_unix?: number
  last?: unknown
}
interface ReloadResponse {
  ok: boolean
  path?: string
  fields: string[]
  admin_op_busy?: boolean
  op?: string
  started_at_unix?: number
  last?: unknown
}
interface ErrorCodeSpec {
  code: string
  http_status: number
  grpc_code: string
  description: string
  retryable?: boolean
  category?: string
  remediation?: string
  dashboard_path?: string
}
interface ErrorCodesResponse {
  codes: ErrorCodeSpec[]
  admin_op_busy?: boolean
  op?: string
  started_at_unix?: number
  last?: unknown
}

function normalizeErrorCodeSpec(raw: Partial<ErrorCodeSpec> | null | undefined): ErrorCodeSpec | null {
  const code = String(raw?.code || '').trim()
  if (!code) return null
  return {
    code,
    http_status: Number(raw?.http_status) || 0,
    grpc_code: String(raw?.grpc_code || ''),
    description: String(raw?.description || ''),
    retryable: Boolean(raw?.retryable),
    category: String(raw?.category || '') || undefined,
    remediation: String(raw?.remediation || '') || undefined,
    dashboard_path: String(raw?.dashboard_path || '') || undefined,
  }
}

function normalizeErrorCodeList(list: unknown): ErrorCodeSpec[] {
  if (!Array.isArray(list)) return []
  const out: ErrorCodeSpec[] = []
  for (const item of list) {
    const n = normalizeErrorCodeSpec(item as Partial<ErrorCodeSpec>)
    if (n) out.push(n)
  }
  return out
}

interface SchemaField { name: string; description: string }
interface SchemaResponse {
  fields: SchemaField[]
  admin_op_busy?: boolean
  op?: string
  started_at_unix?: number
  last?: unknown
}

useHashScroll()
const route = useRoute()
const { isAdmin } = useAuth()
const { offline, writeBlocked, blockReason, blockedMessageKey } = useMutationGuard()
const { t } = useI18n()
const { applyAdminOpStatus } = useAdminOpBusy()
const { success, info, error: notifyError } = useNotify()
const { notifyMaybeAdminBusy } = useNotifyAdminBusy()
const adminOpBusySummary = inject<ComputedRef<{ busy?: boolean; lastSummary?: string; lastError?: string; lastOk?: boolean | null }> | undefined>('adminOpBusySummary', undefined)
const configAdminLastLabel = computed(() => (adminOpBusySummary?.value?.lastSummary || '').trim())
const configAdminLastErrorDetail = computed(() => {
  if (adminOpBusySummary?.value?.lastOk !== false) return ''
  return (adminOpBusySummary?.value?.lastError || '').trim()
})

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
const schemaError = ref('')
const errorCodesError = ref('')
type ConfigActionKey = 'validate' | 'reload'
const {
  lastFailedAction,
  actionResult,
  canRetryAction,
  clearActionResult,
  setActionOk,
  setActionError,
  setActionResult,
  reportActionError,
} = useActionRetry<ConfigActionKey>()
const configActionLoading = ref('')
const configActionStartedAt = ref<number | null>(null)
const configActionAbort = createActionAbort()

function cancelConfigAction() {
  configActionAbort.cancel()
}

function reportConfigCatch(key: ConfigActionKey, e: unknown) {
  if (isCanceledError(e)) {
    const msg = t.value('adminActionCancelled')
    setActionResult(makeActionResult('info', msg))
    info(msg)
    return
  }
  if (isTimeoutError(e)) {
    const msg = t.value('adminActionTimedOut')
    setActionResult(makeActionResult('error', msg))
    notifyError(msg)
    return
  }
  reportActionError(key, e)
  if (actionResult.value?.message) notifyMaybeAdminBusy(actionResult.value.message, e)
}

function reportAndNotify(key: ConfigActionKey, e: unknown) {
  reportActionError(key, e)
  if (actionResult.value?.message) notifyMaybeAdminBusy(actionResult.value.message, e)
}

const configAdminBusyAction = computed(() =>
  actionResultAdminBusyAction({
    message: actionResult.value?.message || '',
    openLabel: t.value('adminOpBusyOpenOps'),
  }),
)
async function retryLastConfigAction() {
  const key = lastFailedAction.value
  if (key === 'validate') return handleValidate()
  if (key === 'reload') return handleReload()
}
const adminTokenInput = ref(getAdminToken())
const dataTokenInput = ref(getDataToken())
const tokenBaseline = ref({ admin: getAdminToken(), data: getDataToken() })
const tokenFormDirty = computed(
  () =>
    adminTokenInput.value !== tokenBaseline.value.admin
    || dataTokenInput.value !== tokenBaseline.value.data,
)

function onConfigBeforeUnload(e: BeforeUnloadEvent) {
  if (!tokenFormDirty.value) return
  e.preventDefault()
  e.returnValue = ''
}

let unregisterConfigDirty: (() => void) | null = null

function clearSchemaFilter() {
  schemaFilter.value = ''
}

function clearErrorCodeFilter() {
  errorCodeFilter.value = ''
}

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
    || (ec.description || '').toLowerCase().includes(q)
    || (ec.category || '').toLowerCase().includes(q)
    || (ec.remediation || '').toLowerCase().includes(q)
    || (ec.retryable ? 'retryable' : 'non-retryable').includes(q),
  )
})

async function reloadAllConfig() {
  loadError.value = ''
  try {
    await loadConfig()
  } catch (e) {
    loadError.value = formatCaughtError(e)
  }
}

async function reloadConfigSchema() {
  try {
    const schemaData = await apiGet<SchemaResponse>('/api/v1/admin/config/schema')
    applyAdminOpStatus(parseAdminOpStatusPayload(schemaData))
    schemaFields.value = schemaData.fields ?? []
    schemaError.value = ''
  } catch (e) {
    const msg = formatCaughtError(e)
    if (schemaFields.value.length) schemaError.value = msg
    else {
      schemaFields.value = []
      schemaError.value = msg
    }
  }
}

async function reloadErrorCodes() {
  try {
    const data = await apiGet<ErrorCodesResponse>('/api/v1/admin/error-codes')
    applyAdminOpStatus(parseAdminOpStatusPayload(data))
    errorCodes.value = normalizeErrorCodeList(data.codes)
    errorCodesError.value = ''
  } catch (e) {
    const msg = formatCaughtError(e)
    if (errorCodes.value.length) errorCodesError.value = msg
    else {
      errorCodes.value = []
      errorCodesError.value = msg
    }
  }
}

async function loadConfig() {
  schemaError.value = ''
  errorCodesError.value = ''
  const results = await Promise.allSettled([
    apiGet<ConfigResponse>('/api/v1/admin/config/effective'),
    apiGet<ErrorCodesResponse>('/api/v1/admin/error-codes'),
    apiGet<SchemaResponse>('/api/v1/admin/config/schema'),
  ])
  if (results[0].status !== 'fulfilled') throw results[0].reason
  applyAdminOpStatus(parseAdminOpStatusPayload(results[0].value))
  config.value = results[0].value.config
  if (results[1].status === 'fulfilled') {
    applyAdminOpStatus(parseAdminOpStatusPayload(results[1].value))
    errorCodes.value = normalizeErrorCodeList(results[1].value.codes)
    errorCodesError.value = ''
  } else {
    // 保留上次 error-codes，分项提示
    if (!errorCodes.value.length) errorCodes.value = []
    errorCodesError.value = formatCaughtError(results[1].reason)
  }
  if (results[2].status === 'fulfilled') {
    applyAdminOpStatus(parseAdminOpStatusPayload(results[2].value))
    schemaFields.value = results[2].value.fields ?? []
    schemaError.value = ''
  } else {
    if (!schemaFields.value.length) schemaFields.value = []
    schemaError.value = formatCaughtError(results[2].reason)
  }
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
  unregisterConfigDirty = registerDirtyChecker('config', () => tokenFormDirty.value)
  window.addEventListener('beforeunload', onConfigBeforeUnload)
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
  if (configActionLoading.value) return
  if (writeBlocked.value) {
    const msg = t.value(blockedMessageKey('offlineAdminBlocked'))
    setActionError(msg)
    notifyError(msg)
    return
  }
  clearActionResult()
  validateResult.value = null
  configActionLoading.value = 'validate'
  configActionStartedAt.value = Date.now()
  const signal = configActionAbort.begin()
  try {
    validateResult.value = await apiPost<ValidateResponse>('/api/v1/admin/config/validate', { config: config.value }, { signal })
    applyAdminOpStatus(parseAdminOpStatusPayload(validateResult.value))
    if (validateResult.value.ok) {
      const msg = formatMessage(t.value('configValidateOk'), {
        path: String(validateResult.value.path || '/api/v1/admin/config/validate'),
      })
      setActionOk(msg)
      success(msg)
    } else {
      const msg = formatMessage(t.value('configValidateFail'), {
        error: validateResult.value.error || '',
        path: String(validateResult.value.path || '/api/v1/admin/config/validate'),
      })
      setActionError(msg)
      notifyError(msg)
    }
  } catch (e) {
    reportConfigCatch('validate', e)
  } finally {
    configActionAbort.end()
    configActionLoading.value = ''
    configActionStartedAt.value = null
  }
}

async function handleReload() {
  if (configActionLoading.value) return
  if (writeBlocked.value) {
    const msg = t.value(blockedMessageKey('offlineAdminBlocked'))
    setActionError(msg)
    notifyError(msg)
    return
  }
  clearActionResult()
  reloadResult.value = null
  configActionLoading.value = 'reload'
  configActionStartedAt.value = Date.now()
  const signal = configActionAbort.begin()
  try {
    reloadResult.value = await apiPost<ReloadResponse>('/api/v1/admin/config/reload', undefined, { signal })
    applyAdminOpStatus(parseAdminOpStatusPayload(reloadResult.value))
    const path = String(reloadResult.value.path || '/api/v1/admin/config/reload')
    const okMsg = reloadResult.value.fields?.length
      ? formatMessage(t.value('configReloadOkFields'), { fields: reloadResult.value.fields.join(', '), path })
      : formatMessage(t.value('configReloadOk'), { path })
    setActionOk(okMsg)
    await loadConfig()
    success(okMsg)
  } catch (e) {
    reportConfigCatch('reload', e)
  } finally {
    configActionAbort.end()
    configActionLoading.value = ''
    configActionStartedAt.value = null
  }
}

function saveServiceTokens() {
  setAdminToken(adminTokenInput.value)
  setDataToken(dataTokenInput.value)
  tokenBaseline.value = { admin: adminTokenInput.value, data: dataTokenInput.value }
  success(t.value('configTokenSaved'))
}

function clearServiceTokens() {
  adminTokenInput.value = ''
  dataTokenInput.value = ''
  setAdminToken('')
  setDataToken('')
  tokenBaseline.value = { admin: '', data: '' }
  setActionOk(t.value('configTokenCleared'))
  success(t.value('configTokenCleared'))
}

async function exportEffective() {
  if (!config.value) {
    notifyError(t.value('configExportEmpty'))
    return
  }
  if (exportBusy.value) return
  const cfg = config.value
  const outcome = await runJSONExport({
    label: 'JSON',
    filename: stampFilename('mts-config-effective', 'json'),
    total: 1,
    build: async ({ isCancelled, progress }) => {
      progress(0, 1)
      if (isCancelled()) return null
      progress(1, 1)
      return buildEffectiveConfigExport(cfg)
    },
  })
  if (outcome === 'done') success(t.value('configExported'))
  else if (outcome === 'cancelled') info(t.value('exportCancelledToast'))
  else if (outcome === 'error') notifyError(exportJob.value.error || t.value('failed'))
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

async function exportSchema() {
  if (!filteredSchema.value.length) {
    notifyError(t.value('configExportEmpty'))
    return
  }
  if (exportBusy.value) return
  const list = filteredSchema.value.slice()
  const outcome = await runJSONExport({
    label: 'JSON',
    filename: stampFilename('mts-config-schema', 'json'),
    total: list.length,
    build: async ({ isCancelled, progress }) => {
      progress(0, list.length)
      const chunk = 100
      for (let i = 0; i < list.length; i++) {
        if (isCancelled()) return null
        const done = i + 1
        if (done === list.length || done % chunk === 0) {
          progress(done, list.length)
          if (done < list.length) await new Promise((r) => setTimeout(r, 0))
        }
      }
      return buildConfigSchemaExport(list)
    },
  })
  if (outcome === 'done') success(t.value('configExported'))
  else if (outcome === 'cancelled') info(t.value('exportCancelledToast'))
  else if (outcome === 'error') notifyError(exportJob.value.error || t.value('failed'))
}

async function exportErrorCodes() {
  if (!filteredErrorCodes.value.length) {
    notifyError(t.value('configExportEmpty'))
    return
  }
  if (exportBusy.value) return
  const list = filteredErrorCodes.value.slice()
  const outcome = await runJSONExport({
    label: 'JSON',
    filename: stampFilename('mts-error-codes', 'json'),
    total: list.length,
    build: async ({ isCancelled, progress }) => {
      progress(0, list.length)
      const chunk = 100
      for (let i = 0; i < list.length; i++) {
        if (isCancelled()) return null
        const done = i + 1
        if (done === list.length || done % chunk === 0) {
          progress(done, list.length)
          if (done < list.length) await new Promise((r) => setTimeout(r, 0))
        }
      }
      return buildErrorCodesExport(list)
    },
  })
  if (outcome === 'done') success(t.value('configExported'))
  else if (outcome === 'cancelled') info(t.value('exportCancelledToast'))
  else if (outcome === 'error') notifyError(exportJob.value.error || t.value('failed'))
}

function statusLabel(httpStatus: number): string {
  if (httpStatus >= 200 && httpStatus < 300) return t.value('configHttpOk')
  if (httpStatus >= 400 && httpStatus < 500) return t.value('configHttpClientErr')
  if (httpStatus >= 500) return t.value('configHttpServerErr')
  return ''
}

onBeforeUnmount(() => {
  cancelConfigAction()
  unregisterConfigDirty?.()
  unregisterConfigDirty = null
  window.removeEventListener('beforeunload', onConfigBeforeUnload)
})
</script>

<template>
  <PermissionDenied v-if="!isAdmin" />
  <div v-else class="space-y-6" data-testid="config-page">
    <div class="flex flex-wrap items-center justify-between gap-3">
      <div>
        <h1 class="mts-title flex flex-wrap items-center gap-2">
          {{ t('configTitle') }}
          <span
            v-if="tokenFormDirty"
            data-testid="config-token-dirty-badge"
            class="rounded-full bg-amber-100 px-2 py-0.5 text-[11px] font-medium text-amber-800 dark:bg-amber-950/50 dark:text-amber-200"
          >{{ t('configTokenDirtyBadge') }}</span>
        </h1>
        <div class="mt-1 flex flex-wrap items-center gap-2">
          <p class="text-xs mts-muted">{{ t('configDesc') }}</p>
          <AdminOpLastChip
            v-if="configAdminLastLabel"
            :label="configAdminLastLabel"
            :last-ok="adminOpBusySummary?.lastOk"
            :last-error="configAdminLastErrorDetail"
            test-id="config-admin-last"
            show-copy
            copy-test-id="config-admin-last-copy"
            error-test-id="config-admin-last-error"
          />

        </div>
      </div>
      <div class="flex flex-wrap gap-2">
        <button type="button" class="mts-btn" data-testid="config-share-link" @click="copyConfigShareLink">
          {{ t('configShareLink') }}
        </button>
        <ExportJobBanner class="w-full basis-full" :job="exportJob" :retryable="canRetryExport" @cancel="cancelExport" @retry="retryLastExport" @dismiss="resetExport" />
        <button type="button" class="mts-btn" data-testid="config-export-effective" :disabled="exportBusy || (!config)" @click="exportEffective">
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
      retryable
      data-testid="config-load-error"
      @retry="() => { void reloadAllConfig() }"
      @dismiss="loadError = ''"
    />
    <PartialErrorBanner
      v-if="schemaError"
      :message="`${t('configSchemaLoadFailed')}：${schemaError}`"
      test-id="config-schema-error"
      @retry="() => { void reloadConfigSchema() }"
      @dismiss="schemaError = ''"
    />
    <PartialErrorBanner
      v-if="errorCodesError"
      :message="`${t('configErrorCodesLoadFailed')}：${errorCodesError}`"
      test-id="config-error-codes-error"
      @retry="() => { void reloadErrorCodes() }"
      @dismiss="errorCodesError = ''"
    />
    <ActionResultBanner
      :result="actionResult"
      :retryable="canRetryAction"
      :action-label="configAdminBusyAction?.label || ''"
      :action-path="configAdminBusyAction?.path || ''"
      data-testid="config-action-result"
      @retry="retryLastConfigAction"
      @dismiss="clearActionResult"
    />
    <InFlightBanner
      :active="!!configActionLoading"
      :started-at-ms="configActionStartedAt"
      kind="admin"
      @cancel="cancelConfigAction"
    />

    <div class="mts-panel" data-testid="config-token-panel">
      <h2 class="mb-2 text-sm font-semibold text-slate-800 dark:text-slate-100">{{ t('configTokenTitle') }}</h2>
      <p class="mb-4 text-xs mts-muted">{{ t('configTokenHint') }}</p>
      <div class="grid gap-3 sm:grid-cols-2">
        <div>
          <label class="mb-1 block text-xs mts-muted" for="config-admin-token">{{ t('configAdminTokenLabel') }}</label>
          <PasswordInputWithToggle
            id="config-admin-token"
            v-model="adminTokenInput"
            input-class="mts-input pr-10"
            test-id="config-token-admin"
            toggle-test-id="config-token-admin-toggle"
            autocomplete="off"
            :placeholder="t('optional')"
          />
        </div>
        <div>
          <label class="mb-1 block text-xs mts-muted" for="config-data-token">{{ t('configDataTokenLabel') }}</label>
          <PasswordInputWithToggle
            id="config-data-token"
            v-model="dataTokenInput"
            input-class="mts-input pr-10"
            test-id="config-token-data"
            toggle-test-id="config-token-data-toggle"
            autocomplete="off"
            :placeholder="t('optional')"
          />
        </div>
      </div>
      <div class="mt-3 flex gap-2">
        <button type="button" class="mts-btn-primary" data-testid="config-token-save" @click="saveServiceTokens">{{ t('configTokenSave') }}</button>
        <button type="button" class="mts-btn" data-testid="config-token-clear" @click="clearServiceTokens">{{ t('configTokenClear') }}</button>
      </div>
    </div>

    <div class="flex flex-wrap gap-3">
      <button class="mts-btn-primary" data-testid="config-validate" @click="handleValidate" :disabled="writeBlocked || !!configActionLoading" :title="writeBlocked ? t(blockedMessageKey('offlineAdminBlocked')) : undefined"><CheckCircle class="h-4 w-4" />{{ t('configValidate') }}</button>
      <button class="mts-btn-primary" data-testid="config-reload" :disabled="writeBlocked || !!configActionLoading" :title="writeBlocked ? t(blockedMessageKey('offlineAdminBlocked')) : undefined" @click="handleReload"><RefreshCw class="h-4 w-4" />{{ t('configReload') }}</button>
    </div>
    <div v-if="validateResult" :class="validateResult.ok ? 'mts-alert-ok' : 'mts-alert-error'" :role="validateResult.ok ? 'status' : 'alert'" :aria-live="validateResult.ok ? 'polite' : 'assertive'" data-testid="config-validate-result">
      <p v-if="validateResult.ok">{{ t('configValidateOk') }}</p>
      <p v-else>{{ formatMessage(t('configValidateFail'), { error: validateResult.error || '' }) }}</p>
    </div>
    <div v-if="reloadResult" class="mts-alert-ok" role="status" aria-live="polite" data-testid="config-reload-result">
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
          <button type="button" class="mts-btn" data-testid="config-export-schema" :disabled="exportBusy || (!filteredSchema.length)" @click="exportSchema">
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
        <EmptyState
          v-if="!filteredSchema.length"
          compact
          data-testid="config-schema-empty"
          :title="t('configSchemaEmpty')"
          :description="schemaFilter.trim() ? t('configSchemaEmptyDesc') : t('configSchemaEmptyNoData')"
        >
          <template v-if="schemaFilter.trim()" #action>
            <button type="button" class="mts-btn-primary text-xs" data-testid="config-schema-clear-filter" @click="clearSchemaFilter">
              {{ t('clearFilters') }}
            </button>
          </template>
        </EmptyState>
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
          <button type="button" class="mts-btn" data-testid="config-export-error-codes" :disabled="exportBusy || (!filteredErrorCodes.length)" @click="exportErrorCodes">
            <Download class="h-3.5 w-3.5" /> {{ t('configExportErrorCodes') }}
          </button>
        </div>
      </div>
      <div class="overflow-hidden rounded-lg border border-slate-100 dark:border-slate-800 scroll-mt-20" id="config-error-codes" data-testid="config-error-codes-table">
        <div
          class="grid grid-cols-[minmax(7rem,0.8fr)_minmax(5.5rem,0.55fr)_minmax(5.5rem,0.55fr)_minmax(4.5rem,0.45fr)_minmax(10rem,1fr)_minmax(11rem,1.2fr)] border-b border-slate-200 text-left dark:border-slate-700"
          data-testid="config-error-codes-header"
        >
          <div class="px-2 py-2 text-xs font-medium mts-muted">{{ t('configColCode') }}</div>
          <div class="px-2 py-2 text-xs font-medium mts-muted">{{ t('configColHTTP') }}</div>
          <div class="px-2 py-2 text-xs font-medium mts-muted">{{ t('configColGRPC') }}</div>
          <div class="px-2 py-2 text-xs font-medium mts-muted">{{ t('configColRetryable') }}</div>
          <div class="px-2 py-2 text-xs font-medium mts-muted">{{ t('configColDescription') }}</div>
          <div class="px-2 py-2 text-xs font-medium mts-muted">{{ t('configColRemediation') }}</div>
        </div>
        <EmptyState
          v-if="!filteredErrorCodes.length"
          compact
          data-testid="config-error-codes-empty"
          :title="t('configErrorCodesEmpty')"
          :description="errorCodeFilter.trim() ? t('configErrorCodesEmptyDesc') : t('configErrorCodesEmptyNoData')"
        >
          <template v-if="errorCodeFilter.trim()" #action>
            <button type="button" class="mts-btn-primary text-xs" data-testid="config-error-codes-clear-filter" @click="clearErrorCodeFilter">
              {{ t('clearFilters') }}
            </button>
          </template>
        </EmptyState>
        <VirtualTable
          v-else
          :items="filteredErrorCodes"
          :row-height="ERROR_ROW_HEIGHT"
          :height="Math.min(CONFIG_LIST_HEIGHT, Math.max(176, filteredErrorCodes.length * ERROR_ROW_HEIGHT))"
          data-testid="config-error-codes-virtual-list"
        >
          <template #default="{ item: ec }">
            <div
              class="grid h-full grid-cols-[minmax(7rem,0.8fr)_minmax(5.5rem,0.55fr)_minmax(5.5rem,0.55fr)_minmax(4.5rem,0.45fr)_minmax(10rem,1fr)_minmax(11rem,1.2fr)] items-center border-b border-slate-100 dark:border-slate-800"
              :data-testid="`config-error-code-row-${ec.code}`"
            >
              <div class="truncate px-2 font-mono text-xs" :title="ec.code">
                {{ ec.code }}
                <span v-if="ec.category" class="ml-1 rounded bg-slate-100 px-1 text-[10px] mts-muted dark:bg-slate-800" :data-testid="`config-error-code-cat-${ec.code}`">{{ ec.category }}</span>
              </div>
              <div class="px-2">
                <span class="rounded bg-slate-100 px-1.5 py-0.5 text-xs dark:bg-slate-800">{{ ec.http_status }} {{ statusLabel(ec.http_status) }}</span>
              </div>
              <div class="truncate px-2 font-mono text-xs" :title="ec.grpc_code">{{ ec.grpc_code }}</div>
              <div class="px-2 text-xs" :data-testid="`config-error-code-retry-${ec.code}`">
                <span :class="ec.retryable ? 'text-emerald-700 dark:text-emerald-300' : 'mts-muted'">{{ ec.retryable ? t('yes') : t('no') }}</span>
              </div>
              <div class="truncate px-2 text-xs mts-muted" :title="ec.description">{{ ec.description }}</div>
              <div class="truncate px-2 text-xs mts-muted" :title="ec.remediation || ''" :data-testid="`config-error-code-remediation-${ec.code}`">{{ ec.remediation || '—' }}</div>
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
