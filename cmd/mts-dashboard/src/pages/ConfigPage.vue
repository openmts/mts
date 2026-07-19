<script setup lang="ts">
import { ref, onMounted } from 'vue'
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

const { isAdmin } = useAuth()
const { success, error: notifyError } = useNotify()
const config = ref<Record<string, unknown> | null>(null)
const validateResult = ref<ValidateResponse | null>(null)
const reloadResult = ref<ReloadResponse | null>(null)
const errorCodes = ref<ErrorCodeSpec[]>([])
const loadError = ref('')
const actionError = ref('')
const adminTokenInput = ref(getAdminToken())
const dataTokenInput = ref(getDataToken())

async function loadConfig() {
  const [cfgData, ecData] = await Promise.all([
    apiGet<ConfigResponse>('/api/v1/admin/config/effective'),
    apiGet<ErrorCodesResponse>('/api/v1/admin/error-codes'),
  ])
  config.value = cfgData.config
  errorCodes.value = ecData.codes ?? []
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
    <p v-if="loadError" class="rounded-lg border border-red-200 bg-red-50 p-3 text-sm text-red-700">{{ loadError }}</p>
    <p v-if="actionError" class="rounded-lg border border-red-200 bg-red-50 p-3 text-sm text-red-700">{{ actionError }}</p>

    <div class="rounded-xl border border-slate-200 bg-white p-6">
      <h2 class="mb-2 text-sm font-semibold text-slate-800">服务级 Token（可选）</h2>
      <p class="mb-4 text-xs text-slate-500">当服务启用 admin_token / data_tokens 时，可在此配置。保存在 sessionStorage，关闭浏览器后失效。</p>
      <div class="grid gap-3 sm:grid-cols-2">
        <div>
          <label class="mb-1 block text-xs text-slate-500">X-MTS-Admin-Token</label>
          <input v-model="adminTokenInput" type="password" class="w-full rounded-lg border border-slate-200 px-3 py-2 text-sm" placeholder="可选" />
        </div>
        <div>
          <label class="mb-1 block text-xs text-slate-500">X-MTS-Data-Token</label>
          <input v-model="dataTokenInput" type="password" class="w-full rounded-lg border border-slate-200 px-3 py-2 text-sm" placeholder="可选" />
        </div>
      </div>
      <div class="mt-3 flex gap-2">
        <button class="rounded-lg bg-slate-800 px-3 py-1.5 text-xs text-white" @click="saveServiceTokens">保存</button>
        <button class="rounded-lg border border-slate-200 px-3 py-1.5 text-xs" @click="clearServiceTokens">清除</button>
      </div>
    </div>

    <div class="flex flex-wrap gap-3">
      <button class="flex items-center gap-2 whitespace-nowrap rounded-lg bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700" @click="handleValidate">
        <CheckCircle class="h-4 w-4" />验证配置
      </button>
      <button class="flex items-center gap-2 whitespace-nowrap rounded-lg bg-slate-800 px-4 py-2 text-sm font-medium text-white hover:bg-slate-700" @click="handleReload">
        <RefreshCw class="h-4 w-4" />热重载
      </button>
    </div>
    <div v-if="validateResult" :class="validateResult.ok ? 'border-green-200 bg-green-50' : 'border-red-200 bg-red-50'" class="rounded-lg border p-3">
      <p v-if="validateResult.ok" class="text-sm text-green-700">配置验证通过</p>
      <p v-else class="text-sm text-red-700">配置验证失败: {{ validateResult.error }}</p>
    </div>
    <div v-if="reloadResult" class="rounded-lg border border-green-200 bg-green-50 p-3">
      <p class="text-sm text-green-700">配置已重载<span v-if="reloadResult.fields?.length">，变更字段: {{ reloadResult.fields.join(', ') }}</span></p>
    </div>
    <div v-if="config" class="rounded-xl border border-slate-200 bg-white p-6">
      <h2 class="mb-4 text-sm font-semibold text-slate-800">有效配置</h2>
      <pre class="max-h-96 overflow-auto rounded-lg bg-slate-900 p-4 text-xs text-green-400">{{ JSON.stringify(config, null, 2) }}</pre>
    </div>
    <div class="rounded-xl border border-slate-200 bg-white p-4 sm:p-6">
      <h2 class="mb-4 text-sm font-semibold text-slate-800">错误码契约</h2>
      <div v-if="!errorCodes.length" class="text-sm text-slate-400">暂无数据</div>
      <table v-else class="w-full text-sm">
        <thead>
          <tr class="border-b border-slate-200 text-left">
            <th class="pb-2 text-xs font-medium text-slate-500">Code</th>
            <th class="pb-2 text-xs font-medium text-slate-500">HTTP</th>
            <th class="pb-2 text-xs font-medium text-slate-500">gRPC</th>
            <th class="pb-2 text-xs font-medium text-slate-500">说明</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="ec in errorCodes" :key="ec.code" class="border-b border-slate-100 last:border-b-0">
            <td class="py-2 font-mono text-xs text-slate-700">{{ ec.code }}</td>
            <td class="py-2"><span class="rounded bg-slate-100 px-1.5 py-0.5 text-xs text-slate-600">{{ ec.http_status }} {{ statusLabel(ec.http_status) }}</span></td>
            <td class="py-2 font-mono text-xs text-slate-600">{{ ec.grpc_code }}</td>
            <td class="py-2 text-xs text-slate-600">{{ ec.description }}</td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>
