<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { apiGet, apiPost, getAdminToken, setAdminToken, getDataToken, setDataToken } from '@/api/client'
import { useAuth } from '@/composables/useAuth'
import { useNotify } from '@/composables/useNotify'
import { formatCaughtError } from '@/utils/apiError'
import { useI18n } from '@/composables/useI18n'
import { formatMessage } from '@/utils/formatMessage'
import PermissionDenied from '@/components/PermissionDenied.vue'
import ActionResultBanner from '@/components/ActionResultBanner.vue'
import { makeActionResult, type ActionResult } from '@/utils/actionResult'
import { RefreshCw, CheckCircle } from 'lucide-vue-next'

interface ConfigResponse { config: Record<string, unknown> }
interface ValidateResponse { ok: boolean; error?: string }
interface ReloadResponse { ok: boolean; fields: string[] }
interface ErrorCodeSpec { code: string; http_status: number; grpc_code: string; description: string }
interface ErrorCodesResponse { codes: ErrorCodeSpec[] }
interface SchemaField { name: string; description: string }
interface SchemaResponse { fields: SchemaField[] }

const { isAdmin } = useAuth()
const { t } = useI18n()
const { success, error: notifyError } = useNotify()
const config = ref<Record<string, unknown> | null>(null)
const validateResult = ref<ValidateResponse | null>(null)
const reloadResult = ref<ReloadResponse | null>(null)
const errorCodes = ref<ErrorCodeSpec[]>([])
const schemaFields = ref<SchemaField[]>([])
const schemaFilter = ref('')
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

onMounted(async () => {
  if (!isAdmin.value) return
  try {
    await loadConfig()
  } catch (e) {
    loadError.value = formatCaughtError(e)
  }
})

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
      <pre class="max-h-96 overflow-auto rounded-lg bg-slate-900 p-4 text-xs text-green-400">{{ JSON.stringify(config, null, 2) }}</pre>
    </div>

    <div class="mts-panel">
      <div class="mb-3 flex flex-wrap items-center justify-between gap-2">
        <h2 class="text-sm font-semibold text-slate-800 dark:text-slate-100">{{ t('configSchema') }}</h2>
        <input v-model="schemaFilter" class="mts-input max-w-xs text-xs"  :placeholder="t('configSchemaFilter')" />
      </div>
      <div class="max-h-80 overflow-auto" data-testid="config-schema-table">
        <table class="w-full text-sm">
          <thead>
            <tr class="border-b border-slate-200 text-left text-[11px] uppercase mts-muted dark:border-slate-700">
              <th class="px-2 py-2">{{ t('configColName') }}</th>
              <th class="px-2 py-2">{{ t('configColDescription') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="f in filteredSchema" :key="f.name" class="border-b border-slate-100 dark:border-slate-800">
              <td class="px-2 py-2 font-mono text-xs">{{ f.name }}</td>
              <td class="px-2 py-2 text-xs mts-muted">{{ f.description }}</td>
            </tr>
            <tr v-if="!filteredSchema.length">
              <td colspan="2" class="px-2 py-3 text-xs mts-muted">{{ t('configSchemaEmpty') }}</td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <div class="mts-panel">
      <h2 class="mb-4 text-sm font-semibold text-slate-800 dark:text-slate-100">{{ t('configErrorCodes') }}</h2>
      <table class="w-full text-sm" data-testid="config-error-codes-table">
        <thead>
          <tr class="border-b border-slate-200 text-left dark:border-slate-700">
            <th class="pb-2 text-xs font-medium mts-muted">{{ t('configColCode') }}</th>
            <th class="pb-2 text-xs font-medium mts-muted">{{ t('configColHTTP') }}</th>
            <th class="pb-2 text-xs font-medium mts-muted">{{ t('configColGRPC') }}</th>
            <th class="pb-2 text-xs font-medium mts-muted">{{ t('configColDescription') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="ec in errorCodes" :key="ec.code" class="border-b border-slate-100 last:border-b-0 dark:border-slate-800">
            <td class="py-2 font-mono text-xs">{{ ec.code }}</td>
            <td class="py-2"><span class="rounded bg-slate-100 px-1.5 py-0.5 text-xs dark:bg-slate-800">{{ ec.http_status }} {{ statusLabel(ec.http_status) }}</span></td>
            <td class="py-2 font-mono text-xs">{{ ec.grpc_code }}</td>
            <td class="py-2 text-xs mts-muted">{{ ec.description }}</td>
          </tr>
          <tr v-if="!errorCodes.length">
            <td colspan="4" class="py-3 text-xs mts-muted">{{ t('configErrorCodesEmpty') }}</td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>
