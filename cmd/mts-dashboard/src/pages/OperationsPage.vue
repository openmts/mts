<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { apiGet, apiPost } from '@/api/client'
import { useAuth } from '@/composables/useAuth'
import PermissionDenied from '@/components/PermissionDenied.vue'
import ConfirmDialog from '@/components/ConfirmDialog.vue'
import { useNotify } from '@/composables/useNotify'
import { RefreshCw, DatabaseBackup, Layers, Timer } from 'lucide-vue-next'

interface CompactionStats { active: number; backlog: number; total: number; success: number; failure: number; last_error: string }
interface CompactionStatsResponse { stats: CompactionStats }
interface MaintenanceStats {
  compaction_active: number
  compaction_backlog: number
  compaction_skipped: number
  compaction_failure: number
  downsample_active: number
  downsample_inflight: number
  downsample_failure: number
  maintenance_error_count: number
}
interface MaintenanceStatsResponse { stats: MaintenanceStats }

const { isAdmin } = useAuth()
const { success, error: notifyError } = useNotify()
const loadError = ref('')
const actionError = ref('')
const loading = ref(false)
const maintenanceStats = ref<MaintenanceStats | null>(null)
const compactionStats = ref<CompactionStats | null>(null)
const confirmKind = ref<'flush' | 'compact' | 'retention' | null>(null)
const confirmLoading = ref(false)

const confirmTitle = {
  flush: '执行 Flush',
  compact: '执行 Compact',
  retention: '应用保留策略',
} as const

const confirmMessage = {
  flush: '确定将内存数据刷盘？可能短时影响写入吞吐。',
  compact: '确定触发压缩？可能占用较多 CPU/IO。',
  retention: '确定按当前保留策略清理过期数据？此操作不可恢复。',
} as const

async function loadStats() {
  if (!isAdmin.value) return
  loading.value = true
  loadError.value = ''
  try {
    const results = await Promise.allSettled([
      apiGet<MaintenanceStatsResponse>('/api/v1/admin/stats/maintenance'),
      apiGet<CompactionStatsResponse>('/api/v1/admin/stats/compaction'),
    ])
    if (results[0].status === 'fulfilled') maintenanceStats.value = results[0].value.stats ?? null
    if (results[1].status === 'fulfilled') compactionStats.value = results[1].value.stats
    const errs = results.filter((r) => r.status === 'rejected') as PromiseRejectedResult[]
    if (errs.length && results.every((r) => r.status === 'rejected')) {
      loadError.value = errs[0].reason instanceof Error ? errs[0].reason.message : '加载失败'
    }
  } catch (e) {
    loadError.value = e instanceof Error ? e.message : '加载失败'
  } finally {
    loading.value = false
  }
}

function openConfirm(kind: 'flush' | 'compact' | 'retention') {
  confirmKind.value = kind
}

async function runConfirmed() {
  if (!confirmKind.value) return
  confirmLoading.value = true
  actionError.value = ''
  try {
    if (confirmKind.value === 'flush') {
      await apiPost('/api/v1/admin/flush', {})
      success('Flush 已完成')
    } else if (confirmKind.value === 'compact') {
      await apiPost('/api/v1/admin/compact', {})
      success('Compact 已触发')
    } else {
      // 不传 now：由服务端使用当前时间，避免前端不安全 ns 乘法
      await apiPost('/api/v1/admin/retention/apply', {})
      success('保留策略已应用')
    }
    confirmKind.value = null
    await loadStats()
  } catch (e) {
    actionError.value = e instanceof Error ? e.message : '操作失败'
    notifyError(actionError.value)
  } finally {
    confirmLoading.value = false
  }
}

onMounted(() => { void loadStats() })
</script>

<template>
  <PermissionDenied v-if="!isAdmin" />
  <div v-else class="space-y-6">
    <div class="flex flex-wrap items-center justify-between gap-3">
      <div>
        <h1 class="text-lg font-semibold text-slate-800">运维</h1>
        <p class="text-xs text-slate-500">Flush / Compact / 保留策略</p>
      </div>
      <button class="inline-flex items-center gap-1 rounded-lg border border-slate-200 bg-white px-3 py-1.5 text-xs" :disabled="loading" @click="loadStats">
        <RefreshCw class="h-3.5 w-3.5" /> 刷新
      </button>
    </div>

    <p v-if="loadError" class="rounded-xl border border-red-200 bg-red-50 p-4 text-sm text-red-700">{{ loadError }}</p>
    <p v-if="actionError" class="rounded-xl border border-red-200 bg-red-50 p-4 text-sm text-red-700">{{ actionError }}</p>

    <div class="grid gap-4 sm:grid-cols-3">
      <button class="rounded-xl border border-slate-200 bg-white p-5 text-left hover:border-slate-300" @click="openConfirm('flush')">
        <DatabaseBackup class="mb-2 h-5 w-5 text-slate-500" />
        <p class="text-sm font-semibold text-slate-800">Flush</p>
        <p class="mt-1 text-xs text-slate-500">将 memtable 刷到持久层</p>
      </button>
      <button class="rounded-xl border border-slate-200 bg-white p-5 text-left hover:border-slate-300" @click="openConfirm('compact')">
        <Layers class="mb-2 h-5 w-5 text-slate-500" />
        <p class="text-sm font-semibold text-slate-800">Compact</p>
        <p class="mt-1 text-xs text-slate-500">触发后台压缩合并</p>
      </button>
      <button class="rounded-xl border border-slate-200 bg-white p-5 text-left hover:border-slate-300" @click="openConfirm('retention')">
        <Timer class="mb-2 h-5 w-5 text-slate-500" />
        <p class="text-sm font-semibold text-slate-800">Apply Retention</p>
        <p class="mt-1 text-xs text-slate-500">按策略清理过期数据（服务端 now）</p>
      </button>
    </div>

    <div class="grid gap-4 lg:grid-cols-2">
      <div class="rounded-xl border border-slate-200 bg-white p-5">
        <h2 class="mb-3 text-sm font-semibold text-slate-800">维护统计</h2>
        <div v-if="!maintenanceStats" class="text-sm text-slate-400">暂无数据</div>
        <dl v-else class="grid grid-cols-2 gap-2 text-xs text-slate-600">
          <div>compact active: <b>{{ maintenanceStats.compaction_active }}</b></div>
          <div>compact backlog: <b>{{ maintenanceStats.compaction_backlog }}</b></div>
          <div>compact failure: <b>{{ maintenanceStats.compaction_failure }}</b></div>
          <div>downsample inflight: <b>{{ maintenanceStats.downsample_inflight }}</b></div>
          <div>downsample failure: <b>{{ maintenanceStats.downsample_failure }}</b></div>
          <div>errors: <b>{{ maintenanceStats.maintenance_error_count }}</b></div>
        </dl>
      </div>
      <div class="rounded-xl border border-slate-200 bg-white p-5">
        <h2 class="mb-3 text-sm font-semibold text-slate-800">压缩统计</h2>
        <div v-if="!compactionStats" class="text-sm text-slate-400">暂无数据</div>
        <dl v-else class="grid grid-cols-2 gap-2 text-xs text-slate-600">
          <div>total: <b>{{ compactionStats.total }}</b></div>
          <div>success: <b>{{ compactionStats.success }}</b></div>
          <div>failure: <b>{{ compactionStats.failure }}</b></div>
          <div>backlog: <b>{{ compactionStats.backlog }}</b></div>
          <div>active: <b>{{ compactionStats.active }}</b></div>
        </dl>
        <p v-if="compactionStats?.last_error" class="mt-2 text-xs text-red-600">{{ compactionStats.last_error }}</p>
      </div>
    </div>

    <ConfirmDialog
      :open="!!confirmKind"
      :title="confirmKind ? confirmTitle[confirmKind] : ''"
      :message="confirmKind ? confirmMessage[confirmKind] : ''"
      confirm-label="执行"
      danger
      :loading="confirmLoading"
      @update:open="(v) => { if (!v) confirmKind = null }"
      @confirm="runConfirmed"
    />
  </div>
</template>
