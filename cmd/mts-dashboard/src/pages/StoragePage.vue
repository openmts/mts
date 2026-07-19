<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { apiPost, apiGet, apiDelete } from '@/api/client'
import { useAuth } from '@/composables/useAuth'
import PermissionDenied from '@/components/PermissionDenied.vue'
import ConfirmDialog from '@/components/ConfirmDialog.vue'
import { useNotify } from '@/composables/useNotify'
import { useI18n } from '@/composables/useI18n'
import { CheckCircle, Camera, Download, Trash2, RefreshCw } from 'lucide-vue-next'

interface ValidateResponse { ok: boolean; data_dir: string; health: Record<string, unknown> }
interface SnapshotResponse { ok: boolean; path: string }
interface SnapshotInfo { name: string; path: string; size_bytes: number; mod_time: string }
interface SnapshotsResponse { snapshots: SnapshotInfo[] }
interface ExportData { generated_at: string; config: Record<string, unknown>; health: Record<string, unknown> }
interface ExportResponse { export: ExportData }

const { isAdmin } = useAuth()
const { success, error: notifyError } = useNotify()
const { t } = useI18n()
const validateResult = ref<ValidateResponse | null>(null)
const snapshotResult = ref<SnapshotResponse | null>(null)
const snapshots = ref<SnapshotInfo[]>([])
const exportData = ref<ExportData | null>(null)
const actionError = ref('')
const loading = ref('')
const deleteOpen = ref(false)
const deleteName = ref('')
const deleteLoading = ref(false)

async function loadSnapshots() {
  try {
    const data = await apiGet<SnapshotsResponse>('/api/v1/admin/storage/snapshots')
    snapshots.value = data.snapshots ?? []
  } catch (e) {
    // 列表接口失败不阻断主流程
    snapshots.value = []
  }
}

onMounted(() => { if (isAdmin.value) void loadSnapshots() })

async function doValidate() {
  loading.value = 'validate'; actionError.value = ''
  try {
    validateResult.value = await apiPost<ValidateResponse>('/api/v1/admin/storage/validate')
    success(validateResult.value.ok ? '验证通过' : '验证完成（存在问题）')
  } catch (e) {
    actionError.value = e instanceof Error ? e.message : '验证失败'
    notifyError(actionError.value)
  } finally { loading.value = '' }
}

async function doSnapshot() {
  loading.value = 'snapshot'; actionError.value = ''
  try {
    snapshotResult.value = await apiPost<SnapshotResponse>('/api/v1/admin/storage/snapshot')
    success(t.value('createSnapshot'))
    await loadSnapshots()
  } catch (e) {
    actionError.value = e instanceof Error ? e.message : '快照失败'
    notifyError(actionError.value)
  } finally { loading.value = '' }
}

async function doExport() {
  loading.value = 'export'; actionError.value = ''
  try {
    const data = await apiGet<ExportResponse>('/api/v1/admin/storage/export')
    exportData.value = data.export
    success('配置已导出')
  } catch (e) {
    actionError.value = e instanceof Error ? e.message : '导出失败'
    notifyError(actionError.value)
  } finally { loading.value = '' }
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

function requestDelete(name: string) {
  deleteName.value = name
  deleteOpen.value = true
}

async function confirmDelete() {
  deleteLoading.value = true
  try {
    await apiDelete(`/api/v1/admin/storage/snapshots?name=${encodeURIComponent(deleteName.value)}`)
    deleteOpen.value = false
    success('快照已删除')
    await loadSnapshots()
  } catch (e) {
    actionError.value = e instanceof Error ? e.message : '删除失败'
    notifyError(actionError.value)
  } finally {
    deleteLoading.value = false
  }
}

function formatBytes(n: number): string {
  if (n >= 1 << 20) return (n / (1 << 20)).toFixed(1) + ' MB'
  if (n >= 1 << 10) return (n / (1 << 10)).toFixed(1) + ' KB'
  return n + ' B'
}
</script>

<template>
  <PermissionDenied v-if="!isAdmin" />
  <div v-else class="space-y-6">
    <p v-if="actionError" class="rounded-lg border border-red-200 bg-red-50 dark:border-red-900 dark:bg-red-950/40 p-3 text-sm text-red-700 dark:text-red-200">{{ actionError }}</p>
    <div class="grid grid-cols-1 gap-4 sm:grid-cols-3">
      <div class="rounded-xl border border-slate-200 bg-white p-6 dark:border-slate-700 dark:bg-slate-900">
        <div class="mb-3 flex items-center gap-2"><CheckCircle class="h-5 w-5 text-slate-500 dark:text-slate-400 dark:text-slate-500" /><h3 class="text-sm font-semibold">存储验证</h3></div>
        <button :disabled="loading === 'validate'" class="w-full rounded-lg bg-slate-800 px-4 py-2 text-sm font-medium text-white disabled:opacity-50 dark:bg-slate-100 dark:bg-slate-800 dark:text-slate-900" @click="doValidate">{{ loading === 'validate' ? t('loading') : '执行验证' }}</button>
      </div>
      <div class="rounded-xl border border-slate-200 bg-white p-6 dark:border-slate-700 dark:bg-slate-900">
        <div class="mb-3 flex items-center gap-2"><Camera class="h-5 w-5 text-slate-500 dark:text-slate-400 dark:text-slate-500" /><h3 class="text-sm font-semibold">{{ t('createSnapshot') }}</h3></div>
        <button :disabled="loading === 'snapshot'" class="w-full rounded-lg bg-slate-800 px-4 py-2 text-sm font-medium text-white disabled:opacity-50 dark:bg-slate-100 dark:bg-slate-800 dark:text-slate-900" @click="doSnapshot">{{ loading === 'snapshot' ? t('loading') : t('createSnapshot') }}</button>
      </div>
      <div class="rounded-xl border border-slate-200 bg-white p-6 dark:border-slate-700 dark:bg-slate-900">
        <div class="mb-3 flex items-center gap-2"><Download class="h-5 w-5 text-slate-500 dark:text-slate-400 dark:text-slate-500" /><h3 class="text-sm font-semibold">配置导出</h3></div>
        <button :disabled="loading === 'export'" class="w-full rounded-lg bg-slate-800 px-4 py-2 text-sm font-medium text-white disabled:opacity-50 dark:bg-slate-100 dark:bg-slate-800 dark:text-slate-900" @click="doExport">{{ loading === 'export' ? t('loading') : t('export') }}</button>
      </div>
    </div>

    <div class="rounded-xl border border-slate-200 bg-white dark:border-slate-700 dark:bg-slate-900">
      <div class="flex items-center justify-between border-b border-slate-100 px-4 py-2 dark:border-slate-800">
        <h3 class="text-sm font-semibold">{{ t('snapshots') }}</h3>
        <button class="inline-flex items-center gap-1 text-xs text-slate-500 dark:text-slate-400 dark:text-slate-500" @click="loadSnapshots"><RefreshCw class="h-3.5 w-3.5" />{{ t('refresh') }}</button>
      </div>
      <div v-if="!snapshots.length" class="p-6 text-center text-sm text-slate-400 dark:text-slate-500">暂无快照</div>
      <table v-else class="w-full text-sm">
        <thead>
          <tr class="border-b border-slate-100 text-left text-xs text-slate-500 dark:text-slate-400 dark:text-slate-500 dark:border-slate-800">
            <th class="px-4 py-2">名称</th>
            <th class="px-4 py-2">大小</th>
            <th class="px-4 py-2">时间</th>
            <th class="px-4 py-2">操作</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="s in snapshots" :key="s.name" class="border-b border-slate-50 dark:border-slate-800">
            <td class="px-4 py-2 font-mono text-xs">{{ s.name }}</td>
            <td class="px-4 py-2 text-xs">{{ formatBytes(s.size_bytes) }}</td>
            <td class="px-4 py-2 text-xs text-slate-500 dark:text-slate-400 dark:text-slate-500">{{ s.mod_time }}</td>
            <td class="px-4 py-2">
              <button class="rounded p-1 text-slate-400 dark:text-slate-500 hover:text-red-600 dark:text-red-300" :title="t('delete')" @click="requestDelete(s.name)"><Trash2 class="h-4 w-4" /></button>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <div v-if="validateResult" class="rounded-xl border border-slate-200 bg-white p-6 dark:border-slate-700 dark:bg-slate-900">
      <h3 class="mb-3 text-sm font-semibold">验证结果</h3>
      <p class="text-xs text-slate-500 dark:text-slate-400 dark:text-slate-500">数据目录: {{ validateResult.data_dir }}</p>
      <pre class="mt-2 max-h-48 overflow-auto rounded-lg bg-slate-900 p-3 text-xs text-green-400">{{ JSON.stringify(validateResult.health, null, 2) }}</pre>
    </div>
    <div v-if="snapshotResult" class="rounded-xl border border-slate-200 bg-white p-6 dark:border-slate-700 dark:bg-slate-900">
      <h3 class="mb-2 text-sm font-semibold">最近创建</h3>
      <p class="text-sm" :class="snapshotResult.ok ? 'text-green-700 dark:text-green-200' : 'text-red-700 dark:text-red-200'">{{ snapshotResult.ok ? '成功' : '失败' }}</p>
      <p v-if="snapshotResult.path" class="mt-1 text-xs text-slate-500 dark:text-slate-400 dark:text-slate-500">{{ snapshotResult.path }}</p>
    </div>
    <div v-if="exportData" class="rounded-xl border border-slate-200 bg-white p-6 dark:border-slate-700 dark:bg-slate-900">
      <div class="mb-2 flex items-center justify-between gap-2">
        <h3 class="text-sm font-semibold">配置导出</h3>
        <button class="rounded-lg border border-slate-200 px-3 py-1 text-xs dark:border-slate-700" @click="downloadExport">下载 JSON</button>
      </div>
      <pre class="max-h-96 overflow-auto rounded-lg bg-slate-900 p-4 text-xs text-green-400">{{ JSON.stringify(exportData, null, 2) }}</pre>
    </div>

    <ConfirmDialog
      v-model:open="deleteOpen"
      title="删除快照"
      :message="`确定删除快照 ${deleteName}？`"
      confirm-label="删除"
      danger
      :loading="deleteLoading"
      @confirm="confirmDelete"
    />
  </div>
</template>
