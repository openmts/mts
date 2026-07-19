<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { apiPost, apiGet } from '@/api/client'
import { HardDrive, ArrowDownWideNarrow, Archive, Activity } from 'lucide-vue-next'

interface OpResult { ok: boolean; result?: Record<string, unknown> }
interface MaintenanceErrorsResponse { errors: string[] }
interface MaintenanceStatsResponse {
  stats: {
    compaction_active: number
    compaction_backlog: number
    compaction_skipped: number
    compaction_failure: number
    compaction_last_skip?: string
    downsample_active: number
    downsample_inflight: number
    downsample_skipped: number
    downsample_failure: number
    downsample_max_concurrent: number
    maintenance_error_count: number
  }
}
interface CompactionStats { active: number; backlog: number; total: number; success: number; failure: number; last_error: string }
interface CompactionStatsResponse { stats: CompactionStats }

const flushResult = ref('')
const compactResult = ref('')
const retentionResult = ref('')
const maintenanceErrors = ref<string[]>([])
const maintenanceStats = ref<MaintenanceStatsResponse['stats'] | null>(null)
const compactionStats = ref<CompactionStats | null>(null)
const actionLoading = ref('')
const pageError = ref('')
const lastLoaded = ref('')

async function doFlush() {
  if (!confirm('确定执行此操作？')) return
  actionLoading.value = 'flush'
  try {
    const res = await apiPost<OpResult>('/api/v1/admin/flush')
    flushResult.value = res.ok ? 'Flush 执行成功' : 'Flush 执行失败'
    await refreshObservability()
  } catch (e) {
    flushResult.value = `失败: ${e instanceof Error ? e.message : '未知错误'}`
  } finally {
    actionLoading.value = ''
  }
}

async function doCompact() {
  if (!confirm('确定执行此操作？')) return
  actionLoading.value = 'compact'
  try {
    const res = await apiPost<OpResult>('/api/v1/admin/compact')
    compactResult.value = res.ok ? 'Compact 执行成功' : 'Compact 执行失败'
    if (res.result) {
      compactResult.value += ` · ${JSON.stringify(res.result)}`
    }
    await refreshObservability()
  } catch (e) {
    compactResult.value = `失败: ${e instanceof Error ? e.message : '未知错误'}`
  } finally {
    actionLoading.value = ''
  }
}

async function doApplyRetention() {
  if (!confirm('确定执行此操作？')) return
  actionLoading.value = 'retention'
  try {
    const res = await apiPost<OpResult>('/api/v1/admin/retention/apply', { now_unix_nanos: Date.now() * 1e6 })
    retentionResult.value = res.ok ? '保留策略应用成功' : '保留策略应用失败'
    if (res.result) {
      retentionResult.value += ` · ${JSON.stringify(res.result)}`
    }
    await refreshObservability()
  } catch (e) {
    retentionResult.value = `失败: ${e instanceof Error ? e.message : '未知错误'}`
  } finally {
    actionLoading.value = ''
  }
}

async function loadMaintenanceErrors() {
  actionLoading.value = 'errors'
  pageError.value = ''
  try {
    const res = await apiGet<MaintenanceErrorsResponse>('/api/v1/admin/maintenance/errors')
    maintenanceErrors.value = res.errors ?? []
    lastLoaded.value = new Date().toLocaleString()
  } catch (e) {
    maintenanceErrors.value = [`加载失败: ${e instanceof Error ? e.message : '未知错误'}`]
  } finally {
    actionLoading.value = ''
  }
}

async function loadMaintenanceStats() {
  actionLoading.value = 'stats'
  pageError.value = ''
  try {
    const res = await apiGet<MaintenanceStatsResponse>('/api/v1/admin/stats/maintenance')
    maintenanceStats.value = res.stats ?? null
    lastLoaded.value = new Date().toLocaleString()
  } catch (e) {
    maintenanceStats.value = null
    pageError.value = `维护统计加载失败: ${e instanceof Error ? e.message : '未知错误'}`
  } finally {
    actionLoading.value = ''
  }
}

async function loadCompactionStats() {
  try {
    const res = await apiGet<CompactionStatsResponse>('/api/v1/admin/stats/compaction')
    compactionStats.value = res.stats ?? null
  } catch (e) {
    compactionStats.value = null
    pageError.value = `Compaction 统计加载失败: ${e instanceof Error ? e.message : '未知错误'}`
  }
}

async function refreshObservability() {
  pageError.value = ''
  actionLoading.value = 'refresh'
  try {
    await Promise.all([
      loadMaintenanceStatsQuiet(),
      loadMaintenanceErrorsQuiet(),
      loadCompactionStats(),
    ])
    lastLoaded.value = new Date().toLocaleString()
  } finally {
    actionLoading.value = ''
  }
}

async function loadMaintenanceStatsQuiet() {
  const res = await apiGet<MaintenanceStatsResponse>('/api/v1/admin/stats/maintenance')
  maintenanceStats.value = res.stats ?? null
}

async function loadMaintenanceErrorsQuiet() {
  const res = await apiGet<MaintenanceErrorsResponse>('/api/v1/admin/maintenance/errors')
  maintenanceErrors.value = res.errors ?? []
}

onMounted(() => {
  void refreshObservability()
})
</script>

<template>
  <div class="space-y-6">
    <div class="flex flex-wrap items-center justify-between gap-3">
      <div>
        <h2 class="text-sm font-semibold text-slate-800">运维操作</h2>
        <p class="mt-1 text-xs text-slate-500">最近观测刷新：{{ lastLoaded || '尚未加载' }}</p>
      </div>
      <button
        :disabled="actionLoading === 'refresh'"
        class="rounded-lg bg-slate-100 px-3 py-1.5 text-xs text-slate-600 hover:bg-slate-200 disabled:opacity-50"
        @click="refreshObservability"
      >
        {{ actionLoading === 'refresh' ? '刷新中...' : '刷新观测' }}
      </button>
    </div>

    <p v-if="pageError" class="rounded-lg border border-red-200 bg-red-50 p-3 text-sm text-red-700">{{ pageError }}</p>

    <div class="grid grid-cols-1 gap-4 sm:grid-cols-3">
      <div class="rounded-xl border border-slate-200 bg-white p-6">
        <div class="mb-3 flex items-center gap-2">
          <HardDrive class="h-5 w-5 text-slate-500" /><h3 class="text-sm font-semibold text-slate-800">Flush</h3>
        </div>
        <p class="mb-4 text-xs text-slate-500">将 MemTable 数据刷写到 SSTable</p>
        <button :disabled="actionLoading === 'flush'" class="w-full rounded-lg bg-slate-800 px-4 py-2 text-sm font-medium text-white hover:bg-slate-700 disabled:opacity-50" @click="doFlush">{{ actionLoading === 'flush' ? '执行中...' : '执行 Flush' }}</button>
        <p v-if="flushResult" class="mt-2 text-xs" :class="!flushResult.includes('成功') ? 'text-red-600' : 'text-green-600'">{{ flushResult }}</p>
      </div>
      <div class="rounded-xl border border-slate-200 bg-white p-6">
        <div class="mb-3 flex items-center gap-2">
          <ArrowDownWideNarrow class="h-5 w-5 text-slate-500" /><h3 class="text-sm font-semibold text-slate-800">Compact</h3>
        </div>
        <p class="mb-4 text-xs text-slate-500">触发 Compaction 合并 SSTable 文件</p>
        <button :disabled="actionLoading === 'compact'" class="w-full rounded-lg bg-slate-800 px-4 py-2 text-sm font-medium text-white hover:bg-slate-700 disabled:opacity-50" @click="doCompact">{{ actionLoading === 'compact' ? '执行中...' : '执行 Compact' }}</button>
        <p v-if="compactResult" class="mt-2 break-all text-xs" :class="!compactResult.includes('成功') ? 'text-red-600' : 'text-green-600'">{{ compactResult }}</p>
      </div>
      <div class="rounded-xl border border-slate-200 bg-white p-6">
        <div class="mb-3 flex items-center gap-2">
          <Archive class="h-5 w-5 text-slate-500" /><h3 class="text-sm font-semibold text-slate-800">保留策略</h3>
        </div>
        <p class="mb-4 text-xs text-slate-500">应用保留策略删除过期数据</p>
        <button :disabled="actionLoading === 'retention'" class="w-full rounded-lg bg-slate-800 px-4 py-2 text-sm font-medium text-white hover:bg-slate-700 disabled:opacity-50" @click="doApplyRetention">{{ actionLoading === 'retention' ? '执行中...' : '应用保留策略' }}</button>
        <p v-if="retentionResult" class="mt-2 break-all text-xs" :class="!retentionResult.includes('成功') ? 'text-red-600' : 'text-green-600'">{{ retentionResult }}</p>
      </div>
    </div>

    <div class="rounded-xl border border-slate-200 bg-white p-6">
      <div class="mb-4 flex items-center justify-between">
        <div class="flex items-center gap-2">
          <Activity class="h-4 w-4 text-slate-500" />
          <h3 class="text-sm font-semibold text-slate-800">Compaction 详情</h3>
        </div>
        <button :disabled="actionLoading === 'refresh'" class="rounded-lg bg-slate-100 px-3 py-1.5 text-xs text-slate-600 hover:bg-slate-200" @click="loadCompactionStats">刷新</button>
      </div>
      <div v-if="!compactionStats" class="text-sm text-slate-400">暂无 compaction 统计</div>
      <div v-else class="grid grid-cols-2 gap-3 text-xs text-slate-600 sm:grid-cols-3">
        <div class="rounded bg-slate-50 px-3 py-2">active: <span class="font-semibold text-slate-800">{{ compactionStats.active }}</span></div>
        <div class="rounded bg-slate-50 px-3 py-2">backlog: <span class="font-semibold text-slate-800">{{ compactionStats.backlog }}</span></div>
        <div class="rounded bg-slate-50 px-3 py-2">total: <span class="font-semibold text-slate-800">{{ compactionStats.total }}</span></div>
        <div class="rounded bg-slate-50 px-3 py-2">success: <span class="font-semibold text-green-700">{{ compactionStats.success }}</span></div>
        <div class="rounded bg-slate-50 px-3 py-2">failure: <span class="font-semibold text-red-700">{{ compactionStats.failure }}</span></div>
        <div class="rounded bg-slate-50 px-3 py-2">last_error: <span class="font-semibold text-slate-800">{{ compactionStats.last_error || '—' }}</span></div>
      </div>
    </div>

    <div class="rounded-xl border border-slate-200 bg-white p-6">
      <div class="mb-4 flex items-center justify-between">
        <h3 class="text-sm font-semibold text-slate-800">维护统计</h3>
        <button :disabled="actionLoading === 'stats'" class="rounded-lg bg-slate-100 px-3 py-1.5 text-xs text-slate-600 hover:bg-slate-200" @click="loadMaintenanceStats">{{ actionLoading === 'stats' ? '加载中...' : '刷新统计' }}</button>
      </div>
      <div v-if="!maintenanceStats" class="text-sm text-slate-400">暂无统计</div>
      <div v-else class="grid grid-cols-2 gap-3 text-xs text-slate-600 sm:grid-cols-3">
        <div class="rounded bg-slate-50 px-3 py-2">compact active: <span class="font-semibold text-slate-800">{{ maintenanceStats.compaction_active }}</span></div>
        <div class="rounded bg-slate-50 px-3 py-2">compact backlog: <span class="font-semibold text-slate-800">{{ maintenanceStats.compaction_backlog }}</span></div>
        <div class="rounded bg-slate-50 px-3 py-2">compact skipped: <span class="font-semibold text-slate-800">{{ maintenanceStats.compaction_skipped }}</span></div>
        <div class="rounded bg-slate-50 px-3 py-2">compact failure: <span class="font-semibold text-slate-800">{{ maintenanceStats.compaction_failure }}</span></div>
        <div class="rounded bg-slate-50 px-3 py-2">downsample active: <span class="font-semibold text-slate-800">{{ maintenanceStats.downsample_active }}</span></div>
        <div class="rounded bg-slate-50 px-3 py-2">downsample inflight: <span class="font-semibold text-slate-800">{{ maintenanceStats.downsample_inflight }}</span></div>
        <div class="rounded bg-slate-50 px-3 py-2">downsample skipped: <span class="font-semibold text-slate-800">{{ maintenanceStats.downsample_skipped }}</span></div>
        <div class="rounded bg-slate-50 px-3 py-2">downsample failure: <span class="font-semibold text-slate-800">{{ maintenanceStats.downsample_failure }}</span></div>
        <div class="rounded bg-slate-50 px-3 py-2">errors: <span class="font-semibold text-slate-800">{{ maintenanceStats.maintenance_error_count }}</span></div>
      </div>
      <p v-if="maintenanceStats?.compaction_last_skip" class="mt-3 text-xs text-amber-700">最近 compact 跳过：{{ maintenanceStats.compaction_last_skip }}</p>
    </div>

    <div class="rounded-xl border border-slate-200 bg-white p-6">
      <div class="mb-4 flex items-center justify-between">
        <h3 class="text-sm font-semibold text-slate-800">维护错误</h3>
        <button :disabled="actionLoading === 'errors'" class="rounded-lg bg-slate-100 px-3 py-1.5 text-xs text-slate-600 hover:bg-slate-200" @click="loadMaintenanceErrors">{{ actionLoading === 'errors' ? '加载中...' : '刷新' }}</button>
      </div>
      <div v-if="!maintenanceErrors.length" class="text-sm text-slate-400">暂无维护错误</div>
      <ul v-else class="space-y-1">
        <li v-for="(err, idx) in maintenanceErrors" :key="idx" class="rounded bg-red-50 px-3 py-1.5 text-xs text-red-700">{{ err }}</li>
      </ul>
    </div>
  </div>
</template>
