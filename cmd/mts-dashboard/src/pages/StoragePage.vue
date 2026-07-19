<script setup lang="ts">
import { ref } from 'vue'
import { apiPost, apiGet } from '@/api/client'
import { useAuth } from '@/composables/useAuth'
import PermissionDenied from '@/components/PermissionDenied.vue'
import { useNotify } from '@/composables/useNotify'
import { CheckCircle, Camera, Download } from 'lucide-vue-next'

interface ValidateResponse { ok: boolean; data_dir: string; health: Record<string, unknown> }
interface SnapshotResponse { ok: boolean; path: string }
interface ExportData { generated_at: string; config: Record<string, unknown>; health: Record<string, unknown> }
interface ExportResponse { export: ExportData }

const { isAdmin } = useAuth()
const { success, error: notifyError } = useNotify()
const validateResult = ref<ValidateResponse | null>(null)
const snapshotResult = ref<SnapshotResponse | null>(null)
const exportData = ref<ExportData | null>(null)
const actionError = ref('')
const loading = ref('')

async function doValidate() {
  loading.value = 'validate'; actionError.value = ''
  try { validateResult.value = await apiPost<ValidateResponse>('/api/v1/admin/storage/validate') }
  catch (e) { actionError.value = e instanceof Error ? e.message : '验证失败' }
  finally { loading.value = '' }
}

async function doSnapshot() {
  loading.value = 'snapshot'; actionError.value = ''
  try { snapshotResult.value = await apiPost<SnapshotResponse>('/api/v1/admin/storage/snapshot') }
  catch (e) { actionError.value = e instanceof Error ? e.message : '快照失败' }
  finally { loading.value = '' }
}

async function doExport() {
  loading.value = 'export'; actionError.value = ''
  try { const data = await apiGet<ExportResponse>('/api/v1/admin/storage/export'); exportData.value = data.export
    success('配置已导出')
  }
  catch (e) { actionError.value = e instanceof Error ? e.message : '导出失败'; notifyError(actionError.value) }
  finally { loading.value = '' }
}

function downloadExport() {
  if (!exportData.value) return
  const blob = new Blob([JSON.stringify(exportData.value, null, 2)], { type: 'application/json' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = `mts-export-${Date.now()}.json`
  a.rel = 'noopener'
  document.body.appendChild(a)
  a.click()
  document.body.removeChild(a)
  URL.revokeObjectURL(url)
  success('已开始下载')
}
</script>

<template>
  <PermissionDenied v-if="!isAdmin" />
  <div v-else class="space-y-6">
    <p v-if="actionError" class="rounded-lg border border-red-200 bg-red-50 p-3 text-sm text-red-700">{{ actionError }}</p>
    <div class="grid grid-cols-1 gap-4 sm:grid-cols-3">
      <div class="rounded-xl border border-slate-200 bg-white p-6">
        <div class="mb-3 flex items-center gap-2"><CheckCircle class="h-5 w-5 text-slate-500" /><h3 class="text-sm font-semibold text-slate-800">存储验证</h3></div>
        <p class="mb-4 text-xs text-slate-500">验证文件结构完整性</p>
        <button :disabled="loading === 'validate'" class="w-full rounded-lg bg-slate-800 px-4 py-2 text-sm font-medium text-white hover:bg-slate-700 disabled:opacity-50" @click="doValidate">{{ loading === 'validate' ? '验证中...' : '执行验证' }}</button>
      </div>
      <div class="rounded-xl border border-slate-200 bg-white p-6">
        <div class="mb-3 flex items-center gap-2"><Camera class="h-5 w-5 text-slate-500" /><h3 class="text-sm font-semibold text-slate-800">存储快照</h3></div>
        <p class="mb-4 text-xs text-slate-500">创建当前存储一致性快照</p>
        <button :disabled="loading === 'snapshot'" class="w-full rounded-lg bg-slate-800 px-4 py-2 text-sm font-medium text-white hover:bg-slate-700 disabled:opacity-50" @click="doSnapshot">{{ loading === 'snapshot' ? '快照中...' : '创建快照' }}</button>
      </div>
      <div class="rounded-xl border border-slate-200 bg-white p-6">
        <div class="mb-3 flex items-center gap-2"><Download class="h-5 w-5 text-slate-500" /><h3 class="text-sm font-semibold text-slate-800">配置导出</h3></div>
        <p class="mb-4 text-xs text-slate-500">导出当前配置和健康快照</p>
        <button :disabled="loading === 'export'" class="w-full rounded-lg bg-slate-800 px-4 py-2 text-sm font-medium text-white hover:bg-slate-700 disabled:opacity-50" @click="doExport">{{ loading === 'export' ? '导出中...' : '导出配置' }}</button>
      </div>
    </div>
    <div v-if="validateResult" class="rounded-xl border border-slate-200 bg-white p-6">
      <h3 class="mb-3 text-sm font-semibold text-slate-800">验证结果</h3>
      <div class="mb-2 flex items-center gap-2">
        <CheckCircle :class="validateResult.ok ? 'text-green-600' : 'text-red-600'" class="h-5 w-5" />
        <span :class="validateResult.ok ? 'text-green-700' : 'text-red-700'" class="text-sm font-medium">{{ validateResult.ok ? '验证通过' : '验证失败' }}</span>
      </div>
      <p class="text-xs text-slate-500">数据目录: {{ validateResult.data_dir }}</p>
      <pre class="mt-2 overflow-auto rounded-lg bg-slate-900 p-3 text-xs text-green-400 max-h-48">{{ JSON.stringify(validateResult.health, null, 2) }}</pre>
    </div>
    <div v-if="snapshotResult" class="rounded-xl border border-slate-200 bg-white p-6">
      <h3 class="mb-2 text-sm font-semibold text-slate-800">快照结果</h3>
      <p :class="snapshotResult.ok ? 'text-green-700' : 'text-red-700'" class="text-sm">{{ snapshotResult.ok ? '快照创建成功' : '快照创建失败' }}</p>
      <p v-if="snapshotResult.path" class="mt-1 text-xs text-slate-500">路径: {{ snapshotResult.path }}</p>
    </div>
    <div v-if="exportData" class="rounded-xl border border-slate-200 bg-white p-6">
      <div class="mb-2 flex items-center justify-between gap-2">
        <h3 class="text-sm font-semibold text-slate-800">配置导出</h3>
        <button class="rounded-lg border border-slate-200 px-3 py-1 text-xs" @click="downloadExport">下载 JSON</button>
      </div>
      <p class="mb-2 text-xs text-slate-500">生成时间: {{ exportData.generated_at }}</p>
      <pre class="max-h-96 overflow-auto rounded-lg bg-slate-900 p-4 text-xs text-green-400">{{ JSON.stringify(exportData, null, 2) }}</pre>
    </div>
  </div>
</template>
