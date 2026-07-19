<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { apiGet, apiPost, getAdminToken, setAdminToken, getDataToken, setDataToken } from '@/api/client'
import { useAuth } from '@/composables/useAuth'
import { useNotify } from '@/composables/useNotify'
import PermissionDenied from '@/components/PermissionDenied.vue'
import { RefreshCw, CheckCircle } from 'lucide-vue-next'

interface ConfigResponse { config: Record<string, unknown> }
interface ValidateResponse { ok: boolean; error?: string }
interface ReloadResponse { ok: boolean; fields: string[] }
interface ErrorCodeSpec { code: string; http_status: number; grpc_code: string; description: string }
interface ErrorCodesResponse { codes: ErrorCodeSpec[] }
interface SchemaField { name: string; description: string }
interface SchemaResponse { fields: SchemaField[] }

const { isAdmin } = useAuth()
const { success, error: notifyError } = useNotify()
const config = ref<Record<string, unknown> | null>(null)
const validateResult = ref<ValidateResponse | null>(null)
const reloadResult = ref<ReloadResponse | null>(null)
const errorCodes = ref<ErrorCodeSpec[]>([])
const schemaFields = ref<SchemaField[]>([])
const schemaFilter = ref('')
const loadError = ref('')
const actionError = ref('')
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
    loadError.value = e instanceof Error ? e.message : '加载失败'
  }
})

async function handleValidate() {
  actionError.value = ''
  validateResult.value = null
  try {
    validateResult.value = await apiPost<ValidateResponse>('/api/v1/admin/config/validate', { config: config.value })
    if (validateResult.value.ok) success('配置验证通过')
  } catch (e) {
    actionError.value = e instanceof Error ? e.message : '验证失败'
    notifyError(actionError.value)
  }
}

async function handleReload() {
  actionError.value = ''
  reloadResult.value = null
  try {
    reloadResult.value = await apiPost<ReloadResponse>('/api/v1/admin/config/reload')
    await loadConfig()
    success('配置已热重载')
  } catch (e) {
    actionError.value = e instanceof Error ? e.message : '重载失败'
    notifyError(actionError.value)
  }
}

function saveServiceTokens() {
  setAdminToken(adminTokenInput.value)
  setDataToken(dataTokenInput.value)
  success('服务级 Token 已保存到 sessionStorage')
}

function clearServiceTokens() {
  adminTokenInput.value = ''
  dataTokenInput.value = ''
  setAdminToken('')
  setDataToken('')
  success('服务级 Token 已清除')
}

function statusLabel(httpStatus: number): string {
  if (httpStatus >= 200 && httpStatus < 300) return '成功'
  if (httpStatus >= 400 && httpStatus < 500) return '客户端错误'
  if (httpStatus >= 500) return '服务端错误'
  return ''
}
</script>

<template>
  <PermissionDenied v-if="!isAdmin" />
  <div v-else class="space-y-6">
    <p v-if="loadError" class="mts-alert-error">{{ loadError }}</p>
    <p v-if="actionError" class="mts-alert-error">{{ actionError }}</p>

    <div class="mts-panel">
      <h2 class="mb-2 text-sm font-semibold text-slate-800 dark:text-slate-100">服务级 Token（可选）</h2>
      <p class="mb-4 text-xs mts-muted">当服务启用 admin_token / data_tokens 时，可在此配置。保存在 sessionStorage（会话级）。</p>
      <div class="grid gap-3 sm:grid-cols-2">
        <div>
          <label class="mb-1 block text-xs mts-muted">X-MTS-Admin-Token</label>
          <input v-model="adminTokenInput" type="password" class="mts-input" placeholder="可选" />
        </div>
        <div>
          <label class="mb-1 block text-xs mts-muted">X-MTS-Data-Token</label>
          <input v-model="dataTokenInput" type="password" class="mts-input" placeholder="可选" />
        </div>
      </div>
      <div class="mt-3 flex gap-2">
        <button class="mts-btn-primary" @click="saveServiceTokens">保存</button>
        <button class="mts-btn" @click="clearServiceTokens">清除</button>
      </div>
    </div>

    <div class="flex flex-wrap gap-3">
      <button class="mts-btn-primary" @click="handleValidate"><CheckCircle class="h-4 w-4" />验证配置</button>
      <button class="mts-btn-primary" @click="handleReload"><RefreshCw class="h-4 w-4" />热重载</button>
    </div>
    <div v-if="validateResult" :class="validateResult.ok ? 'mts-alert-ok' : 'mts-alert-error'">
      <p v-if="validateResult.ok">配置验证通过</p>
      <p v-else>配置验证失败: {{ validateResult.error }}</p>
    </div>
    <div v-if="reloadResult" class="mts-alert-ok">
      <p>配置已重载<span v-if="reloadResult.fields?.length">，变更字段: {{ reloadResult.fields.join(', ') }}</span></p>
    </div>

    <div v-if="config" class="mts-panel">
      <h2 class="mb-4 text-sm font-semibold text-slate-800 dark:text-slate-100">有效配置</h2>
      <pre class="max-h-96 overflow-auto rounded-lg bg-slate-900 p-4 text-xs text-green-400">{{ JSON.stringify(config, null, 2) }}</pre>
    </div>

    <div class="mts-panel">
      <div class="mb-3 flex flex-wrap items-center justify-between gap-2">
        <h2 class="text-sm font-semibold text-slate-800 dark:text-slate-100">配置 Schema</h2>
        <input v-model="schemaFilter" class="mts-input max-w-xs text-xs" placeholder="过滤 name/description" />
      </div>
      <div v-if="!filteredSchema.length" class="text-sm mts-muted">暂无 schema</div>
      <div v-else class="max-h-80 overflow-auto">
        <table class="w-full text-sm">
          <thead>
            <tr class="border-b border-slate-200 text-left text-[11px] uppercase mts-muted dark:border-slate-700">
              <th class="px-2 py-2">Name</th>
              <th class="px-2 py-2">Description</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="f in filteredSchema" :key="f.name" class="border-b border-slate-100 dark:border-slate-800">
              <td class="px-2 py-2 font-mono text-xs">{{ f.name }}</td>
              <td class="px-2 py-2 text-xs mts-muted">{{ f.description }}</td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <div class="mts-panel">
      <h2 class="mb-4 text-sm font-semibold text-slate-800 dark:text-slate-100">错误码契约</h2>
      <div v-if="!errorCodes.length" class="text-sm mts-muted">暂无数据</div>
      <table v-else class="w-full text-sm">
        <thead>
          <tr class="border-b border-slate-200 text-left dark:border-slate-700">
            <th class="pb-2 text-xs font-medium mts-muted">Code</th>
            <th class="pb-2 text-xs font-medium mts-muted">HTTP</th>
            <th class="pb-2 text-xs font-medium mts-muted">gRPC</th>
            <th class="pb-2 text-xs font-medium mts-muted">说明</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="ec in errorCodes" :key="ec.code" class="border-b border-slate-100 last:border-b-0 dark:border-slate-800">
            <td class="py-2 font-mono text-xs">{{ ec.code }}</td>
            <td class="py-2"><span class="rounded bg-slate-100 px-1.5 py-0.5 text-xs dark:bg-slate-800">{{ ec.http_status }} {{ statusLabel(ec.http_status) }}</span></td>
            <td class="py-2 font-mono text-xs">{{ ec.grpc_code }}</td>
            <td class="py-2 text-xs mts-muted">{{ ec.description }}</td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>
