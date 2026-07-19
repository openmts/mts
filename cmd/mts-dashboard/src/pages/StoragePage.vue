<script setup lang="ts">
import { computed, ref, onMounted } from 'vue'
import { apiPost, apiGet, apiDelete } from '@/api/client'
import { useAuth } from '@/composables/useAuth'
import PermissionDenied from '@/components/PermissionDenied.vue'
import ConfirmDialog from '@/components/ConfirmDialog.vue'
import ActionResultBanner from '@/components/ActionResultBanner.vue'
import EmptyState from '@/components/EmptyState.vue'
import { useNotify } from '@/composables/useNotify'
import { useI18n } from '@/composables/useI18n'
import { makeActionResult, type ActionResult } from '@/utils/actionResult'
import { formatBytes } from '@/utils/formatBytes'
import { BACKUP_DRILL_STEPS, drillProgress } from '@/utils/backupDrill'
import { EDGE_HTTPS_ACCEPTANCE_STEPS, edgeHttpsProgress } from '@/utils/edgeHttpsAcceptance'
import { CheckCircle, Camera, Download, Trash2, RefreshCw, ClipboardList } from 'lucide-vue-next'

interface ValidateResponse { ok: boolean; data_dir: string; health: Record<string, unknown> }
interface SnapshotResponse { ok: boolean; path: string }
interface DataSnapshotResponse { ok: boolean; path: string; source?: string; files?: number; bytes?: number }
interface RestoreDrillResponse {
  ok: boolean
  source: string
  target: string
  files?: number
  bytes?: number
  check_issues?: number
  check_fatals?: number
  check_root?: string
}
interface DataSnapshotInfo { name: string; kind: string; path: string; size_bytes: number; mod_time: string }
interface DataSnapshotsResponse { snapshots: DataSnapshotInfo[] }
interface SnapshotInfo { name: string; path: string; size_bytes: number; mod_time: string }
interface SnapshotsResponse { snapshots: SnapshotInfo[] }
interface ExportData { generated_at: string; config: Record<string, unknown>; health: Record<string, unknown> }
interface ExportResponse { export: ExportData }

const { isAdmin } = useAuth()
const { success, error: notifyError } = useNotify()
const { t } = useI18n()
const validateResult = ref<ValidateResponse | null>(null)
const snapshotResult = ref<SnapshotResponse | null>(null)
const dataSnapshotResult = ref<DataSnapshotResponse | null>(null)
const restoreDrillResult = ref<RestoreDrillResponse | null>(null)
const dataSnapshots = ref<DataSnapshotInfo[]>([])
const snapshots = ref<SnapshotInfo[]>([])
const exportData = ref<ExportData | null>(null)
const actionResult = ref<ActionResult | null>(null)
const loading = ref('')
const listLoading = ref(false)
const deleteOpen = ref(false)
const deleteName = ref('')
const deleteLoading = ref(false)
const edgeDone = ref<Record<string, boolean>>({})
const edgeSteps = EDGE_HTTPS_ACCEPTANCE_STEPS
const edgeStats = computed(() => edgeHttpsProgress(Object.keys(edgeDone.value).filter((k) => edgeDone.value[k])))
const drillDone = ref<Record<string, boolean>>({
  validate: false,
  snapshot: false,
  'data-snapshot': false,
  'restore-side': false,
  'export-config': false,
})
const drillSteps = BACKUP_DRILL_STEPS
const drillStats = computed(() => drillProgress(Object.entries(drillDone.value).filter(([, v]) => v).map(([k]) => k)))
function toggleEdge(id: string, checked: boolean) {
  edgeDone.value = { ...edgeDone.value, [id]: checked }
}

function toggleHostDrill(id: string, checked: boolean) {
  drillDone.value = { ...drillDone.value, [id]: checked }
}

async function loadSnapshots() {
  listLoading.value = true
  try {
    const data = await apiGet<SnapshotsResponse>('/api/v1/admin/storage/snapshots')
    snapshots.value = data.snapshots ?? []
  } catch {
    snapshots.value = []
  } finally {
    listLoading.value = false
  }
}

async function loadDataSnapshots() {
  try {
    const data = await apiGet<DataSnapshotsResponse>('/api/v1/admin/storage/data-snapshots')
    dataSnapshots.value = data.snapshots ?? []
  } catch {
    dataSnapshots.value = []
  }
}

onMounted(() => {
  if (isAdmin.value) {
    void loadSnapshots()
    void loadDataSnapshots()
  }
})

async function doValidate() {
  loading.value = 'validate'
  actionResult.value = null
  try {
    validateResult.value = await apiPost<ValidateResponse>('/api/v1/admin/storage/validate')
    drillDone.value = { ...drillDone.value, validate: true }
    const msg = validateResult.value.ok ? '验证通过' : '验证完成（存在问题）'
    actionResult.value = makeActionResult(validateResult.value.ok ? 'ok' : 'warn', msg)
    success(msg)
  } catch (e) {
    const msg = e instanceof Error ? e.message : '验证失败'
    actionResult.value = makeActionResult('error', msg)
    notifyError(msg)
  } finally { loading.value = '' }
}

async function doSnapshot() {
  loading.value = 'snapshot'
  actionResult.value = null
  try {
    snapshotResult.value = await apiPost<SnapshotResponse>('/api/v1/admin/storage/snapshot')
    drillDone.value = { ...drillDone.value, snapshot: true }
    const msg = `${t.value('createSnapshot')}：${snapshotResult.value.path || 'ok'}`
    actionResult.value = makeActionResult('ok', msg)
    success(t.value('createSnapshot'))
    await loadSnapshots()
  } catch (e) {
    const msg = e instanceof Error ? e.message : '快照失败'
    actionResult.value = makeActionResult('error', msg)
    notifyError(msg)
  } finally { loading.value = '' }
}

async function doDataSnapshot() {
  loading.value = 'data-snapshot'
  actionResult.value = null
  try {
    dataSnapshotResult.value = await apiPost<DataSnapshotResponse>('/api/v1/admin/storage/data-snapshot', { flush: true })
    drillDone.value = { ...drillDone.value, 'data-snapshot': true }
    const msg = `data_dir 快照：${dataSnapshotResult.value.path || 'ok'}（files=${dataSnapshotResult.value.files ?? 0}）`
    actionResult.value = makeActionResult('ok', msg)
    success('data_dir 快照完成')
    await loadDataSnapshots()
  } catch (e) {
    const msg = e instanceof Error ? e.message : 'data_dir 快照失败'
    actionResult.value = makeActionResult('error', msg)
    notifyError(msg)
  } finally { loading.value = '' }
}

async function doRestoreDrill() {
  loading.value = 'restore-drill'
  actionResult.value = null
  try {
    const body = dataSnapshotResult.value?.path
      ? { source_path: dataSnapshotResult.value.path }
      : {}
    restoreDrillResult.value = await apiPost<RestoreDrillResponse>('/api/v1/admin/storage/restore-drill', body)
    const ok = !!restoreDrillResult.value.ok && (restoreDrillResult.value.check_fatals ?? 0) === 0
    drillDone.value = { ...drillDone.value, 'restore-side': ok }
    const msg = ok
      ? `旁路恢复完成：${restoreDrillResult.value.target}`
      : `旁路恢复存在致命问题：fatals=${restoreDrillResult.value.check_fatals ?? '?'}`
    actionResult.value = makeActionResult(ok ? 'ok' : 'warn', msg)
    if (ok) success('旁路恢复演练完成')
    else notifyError(msg)
    await loadDataSnapshots()
  } catch (e) {
    const msg = e instanceof Error ? e.message : '旁路恢复失败'
    actionResult.value = makeActionResult('error', msg)
    notifyError(msg)
  } finally { loading.value = '' }
}

async function doExport() {
  loading.value = 'export'
  actionResult.value = null
  try {
    const data = await apiGet<ExportResponse>('/api/v1/admin/storage/export')
    exportData.value = data.export
    drillDone.value = { ...drillDone.value, 'export-config': true }
    actionResult.value = makeActionResult('ok', '配置已导出，可下载 JSON')
    success('配置已导出')
  } catch (e) {
    const msg = e instanceof Error ? e.message : '导出失败'
    actionResult.value = makeActionResult('error', msg)
    notifyError(msg)
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
  actionResult.value = makeActionResult('ok', '已开始下载导出文件')
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
    actionResult.value = makeActionResult('ok', `快照已删除：${deleteName.value}`)
    success('快照已删除')
    await loadSnapshots()
  } catch (e) {
    const msg = e instanceof Error ? e.message : '删除失败'
    actionResult.value = makeActionResult('error', msg)
    notifyError(msg)
  } finally {
    deleteLoading.value = false
  }
}

</script>

<template>
  <PermissionDenied v-if="!isAdmin" />
  <div v-else class="space-y-6">
    <div class="flex flex-wrap items-center justify-between gap-3">
      <div>
        <h1 class="text-lg font-semibold text-slate-800 dark:text-slate-100">{{ t('storage') }}</h1>
        <p class="text-xs mts-muted">验证 · 快照 · 配置导出</p>
      </div>
      <button class="mts-btn" :disabled="listLoading" @click="() => { void loadSnapshots(); void loadDataSnapshots() }">
        <RefreshCw class="h-3.5 w-3.5" /> 刷新快照
      </button>
    </div>

    <ActionResultBanner :result="actionResult" @dismiss="actionResult = null" />

    <div class="mts-card p-4">
      <div class="mb-2 flex flex-wrap items-center justify-between gap-2">
        <h2 class="flex items-center gap-2 text-sm font-semibold text-slate-800 dark:text-slate-100">
          <ClipboardList class="h-4 w-4" />
          备份演练清单
        </h2>
        <span class="text-xs mts-muted">
          进度 {{ drillStats.completed }}/{{ drillStats.total }}
          · 必做 {{ drillStats.requiredCompleted }}/{{ drillStats.requiredTotal }}
        </span>
      </div>
      <p class="mb-3 text-xs mts-muted">
        控制台可完成校验/配置快照/data_dir 快照/旁路恢复/导出；异地拷贝仍需按
        <code class="font-mono">docs/ops/dashboard-production-runbook.md</code> 在主机侧执行。
      </p>
      <ol class="space-y-2">
        <li
          v-for="step in drillSteps"
          :key="step.id"
          class="flex items-start gap-2 rounded-lg border border-slate-100 px-3 py-2 text-sm dark:border-slate-800"
        >
          <input
            v-if="step.inDashboard"
            type="checkbox"
            class="mt-1"
            :checked="!!drillDone[step.id]"
            disabled
          />
          <input
            v-else
            type="checkbox"
            class="mt-1"
            :checked="!!drillDone[step.id]"
            @change="toggleHostDrill(step.id, ($event.target as HTMLInputElement).checked)"
          />
          <div class="min-w-0">
            <p class="font-medium text-slate-800 dark:text-slate-100">
              {{ step.title }}
              <span class="ml-1 text-[11px] font-normal mts-muted">{{ step.severity === 'required' ? '必做' : '推荐' }}</span>
              <span v-if="step.inDashboard" class="ml-1 text-[11px] font-normal text-emerald-700 dark:text-emerald-300">Dashboard</span>
            </p>
            <p class="text-xs mts-muted">{{ step.detail }}</p>
          </div>
        </li>
      </ol>
    </div>


    <div class="mts-card p-4">
      <div class="mb-2 flex flex-wrap items-center justify-between gap-2">
        <h2 class="flex items-center gap-2 text-sm font-semibold text-slate-800 dark:text-slate-100">
          边缘 HTTPS / HSTS 验收
        </h2>
        <span class="text-xs mts-muted">
          进度 {{ edgeStats.done }}/{{ edgeStats.total }}
          · 必做 {{ edgeStats.requiredDone }}/{{ edgeStats.requiredTotal }}
        </span>
      </div>
      <p class="mb-3 text-xs mts-muted">
        证书与跳转由边缘层人工验收；本机 TLS 时 HSTS/doctor 已自动化。详见 runbook 第 2 节。
      </p>
      <ol class="space-y-2">
        <li
          v-for="step in edgeSteps"
          :key="step.id"
          class="flex items-start gap-2 rounded-lg border border-slate-100 px-3 py-2 text-sm dark:border-slate-800"
        >
          <input
            type="checkbox"
            class="mt-1"
            :checked="!!edgeDone[step.id]"
            @change="toggleEdge(step.id, ($event.target as HTMLInputElement).checked)"
          />
          <div class="min-w-0">
            <p class="font-medium text-slate-800 dark:text-slate-100">
              {{ step.title }}
              <span class="ml-1 text-[11px] font-normal mts-muted">{{ step.severity === 'required' ? '必做' : '推荐' }}</span>
              <span v-if="step.partialAutomated" class="ml-1 text-[11px] font-normal text-emerald-700 dark:text-emerald-300">部分自动</span>
            </p>
            <p class="text-xs mts-muted">{{ step.detail }}</p>
          </div>
        </li>
      </ol>
    </div>

    <div class="grid grid-cols-1 gap-4 sm:grid-cols-3">
      <div class="mts-panel">
        <div class="mb-3 flex items-center gap-2"><CheckCircle class="h-5 w-5 text-slate-500" /><h3 class="text-sm font-semibold">存储验证</h3></div>
        <button :disabled="loading === 'validate'" class="mts-btn-primary w-full justify-center py-2" @click="doValidate">{{ loading === 'validate' ? t('loading') : '执行验证' }}</button>
        <pre v-if="validateResult" class="mt-3 max-h-40 overflow-auto rounded bg-slate-950 p-2 text-[11px] text-emerald-400">{{ JSON.stringify(validateResult, null, 2) }}</pre>
      </div>
      <div class="mts-panel">
        <div class="mb-3 flex items-center gap-2"><Camera class="h-5 w-5 text-slate-500" /><h3 class="text-sm font-semibold">{{ t('createSnapshot') }}</h3></div>
        <button :disabled="loading === 'snapshot'" class="mts-btn-primary w-full justify-center py-2" @click="doSnapshot">{{ loading === 'snapshot' ? t('loading') : t('createSnapshot') }}</button>
        <p v-if="snapshotResult?.path" class="mt-2 break-all font-mono text-[11px] mts-muted">{{ snapshotResult.path }}</p>
      </div>
      <div class="mts-panel">
        <div class="mb-3 flex items-center gap-2"><Download class="h-5 w-5 text-slate-500" /><h3 class="text-sm font-semibold">{{ t('export') }}</h3></div>
        <button :disabled="loading === 'export'" class="mts-btn-primary w-full justify-center py-2" @click="doExport">{{ loading === 'export' ? t('loading') : t('export') }}</button>
        <button v-if="exportData" class="mts-btn mt-2 w-full justify-center" @click="downloadExport">下载 JSON</button>
      </div>
    </div>

    <div class="mts-card overflow-hidden">
      <div class="flex items-center justify-between border-b border-slate-100 px-4 py-2 text-xs mts-muted dark:border-slate-800">
        <span>{{ t('snapshots') }}</span>
        <span>{{ snapshots.length }} 个</span>
      </div>
      <EmptyState
        v-if="listLoading"
        compact
        :title="t('loading')"
        description="正在加载快照列表…"
      />
      <EmptyState
        v-else-if="!snapshots.length"
        title="暂无快照"
        description="创建快照后可在此管理；删除操作不可恢复。"
      >
        <template #action>
          <button type="button" class="mts-btn-primary" :disabled="loading === 'snapshot'" @click="doSnapshot">{{ t('createSnapshot') }}</button>
        </template>
      </EmptyState>
      <table v-else class="w-full text-sm">
        <thead>
          <tr class="border-b border-slate-200 text-left text-[11px] uppercase mts-muted dark:border-slate-700">
            <th class="px-4 py-2">名称</th>
            <th class="px-4 py-2">大小</th>
            <th class="px-4 py-2">时间</th>
            <th class="px-4 py-2"></th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="s in snapshots" :key="s.name" class="border-b border-slate-100 dark:border-slate-800">
            <td class="px-4 py-2 font-mono text-xs">{{ s.name }}</td>
            <td class="px-4 py-2 text-xs">{{ formatBytes(s.size_bytes) }}</td>
            <td class="px-4 py-2 text-xs mts-muted">{{ s.mod_time }}</td>
            <td class="px-4 py-2 text-right">
              <button class="rounded p-1 text-slate-400 hover:text-red-600" :title="t('delete')" @click="requestDelete(s.name)">
                <Trash2 class="h-4 w-4" />
              </button>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <div v-if="exportData" class="mts-panel">
      <h3 class="mb-2 text-sm font-semibold">导出预览</h3>
      <pre class="max-h-96 overflow-auto rounded-lg bg-slate-900 p-4 text-xs text-green-400">{{ JSON.stringify(exportData, null, 2) }}</pre>
    </div>

    
    <div class="mts-panel">
      <div class="mb-3 flex flex-wrap items-center justify-between gap-2">
        <h3 class="text-sm font-semibold text-slate-800 dark:text-slate-100">data_dir 旁路恢复编排</h3>
        <span class="text-xs mts-muted">真实存储拷贝（storagecheck）</span>
      </div>
      <p class="mb-3 text-xs mts-muted">
        先创建 data_dir 快照，再恢复到 backups/restore-drill-*（不会覆盖 live data_dir）。
      </p>
      <div class="grid grid-cols-1 gap-3 sm:grid-cols-2">
        <button
          type="button"
          class="mts-btn-primary justify-center py-2"
          :disabled="loading === 'data-snapshot'"
          @click="doDataSnapshot"
        >
          {{ loading === 'data-snapshot' ? t('loading') : '创建 data_dir 快照' }}
        </button>
        <button
          type="button"
          class="mts-btn justify-center py-2"
          :disabled="loading === 'restore-drill'"
          @click="doRestoreDrill"
        >
          {{ loading === 'restore-drill' ? t('loading') : '执行旁路恢复演练' }}
        </button>
      </div>
      <pre v-if="dataSnapshotResult" class="mt-3 max-h-32 overflow-auto rounded bg-slate-950 p-2 text-[11px] text-emerald-400">{{ JSON.stringify(dataSnapshotResult, null, 2) }}</pre>
      <pre v-if="restoreDrillResult" class="mt-2 max-h-32 overflow-auto rounded bg-slate-950 p-2 text-[11px] text-sky-300">{{ JSON.stringify(restoreDrillResult, null, 2) }}</pre>
      <div v-if="dataSnapshots.length" class="mt-4 overflow-x-auto">
        <table class="w-full text-sm">
          <thead>
            <tr class="border-b border-slate-200 text-left text-[11px] uppercase mts-muted dark:border-slate-700">
              <th class="px-2 py-2">Kind</th>
              <th class="px-2 py-2">Name</th>
              <th class="px-2 py-2">Size</th>
              <th class="px-2 py-2">Time</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="s in dataSnapshots" :key="s.path" class="border-b border-slate-100 dark:border-slate-800">
              <td class="px-2 py-2 text-xs">{{ s.kind }}</td>
              <td class="px-2 py-2 font-mono text-xs">{{ s.name }}</td>
              <td class="px-2 py-2 text-xs">{{ formatBytes(s.size_bytes || 0) }}</td>
              <td class="px-2 py-2 text-xs mts-muted">{{ s.mod_time }}</td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

<ConfirmDialog
      v-model:open="deleteOpen"
      title="删除快照"
      :message="`确定删除快照 ${deleteName}？此操作不可恢复。`"
      confirm-label="删除"
      danger
      :loading="deleteLoading"
      @confirm="confirmDelete"
    />
  </div>
</template>
