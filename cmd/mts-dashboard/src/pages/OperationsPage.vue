<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { apiPost, apiGet } from '@/api/client'
import { useAuth } from '@/composables/useAuth'
import PermissionDenied from '@/components/PermissionDenied.vue'
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

const { isAdmin } = useAuth()
const flushResult = ref('')
const compactResult = ref('')
const retentionResult = ref('')
const maintenanceErrors = ref<string[]>([])
const maintenanceStats = ref<MaintenanceStatsResponse['stats'] | null>(null)
const compactionStats = ref<CompactionStats | null>(null)
const loading = ref<Record<string, boolean>>({})
const pageError = ref('')
const lastLoaded = ref('')

function setLoading(key: string, v: boolean) {
  loading.value = { ...loading.value, [key]: v }
}

async function doFlush() {
  if (!confirm('确定执行 Flush？')) return
  setLoading('flush', true)
  try {
    const res = await apiPost<OpResult>('/api/v1/admin/flush')
    flushResult.value = res.ok ? 'Flush 执行成功' : 'Flush 执行失败'
    await refreshObservability()
  } catch (e) {
    flushResult.value = `失败: ${e instanceof Error ? e.message : '未知错误'}`
  } finally {
    setLoading('flush', false)
  }
}

async function doCompact() {
  if (!confirm('确定执行 Compact？')) return
  setLoading('compact', true)
  try {
    const res = await apiPost<OpResult>('/api/v1/admin/compact')
    compactResult.value = res.ok ? 'Compact 执行成功' : 'Compact 执行失败'
    if (res.result) compactResult.value += ` · ${JSON.stringify(res.result)}`
    await refreshObservability()
  } catch (e) {
    compactResult.value = `失败: ${e instanceof Error ? e.message : '未知错误'}`
  } finally {
    setLoading('compact', false)
  }
}

async function doApplyRetention() {
  if (!confirm('确定应用保留策略？')) return
  setLoading('retention', true)
  try {
    const res = await apiPost<OpResult>('/api/v1/admin/retention/apply', { now_unix_nanos: Date.now() * 1e6 })
    retentionResult.value = res.ok ? '保留策略应用成功' : '保留策略应用失败'
    if (res.result) retentionResult.value += ` · ${JSON.stringify(res.result)}`
    await refreshObservability()
  } catch (e) {
    retentionResult.value = `失败: ${e instanceof Error ? e.message : '未知错误'}`
  } finally {
    setLoading('retention', false)
  }
}

async function refreshObservability() {
  pageError.value = ''
  setLoading('refresh', true)
  const results = await Promise.allSettled([
    apiGet<MaintenanceStatsResponse>('/api/v1/admin/stats/maintenance'),
    apiGet<MaintenanceErrorsResponse>('/api/v1/admin/maintenance/errors'),
    apiGet<CompactionStatsResponse>('/api/v1/admin/stats/compaction'),
  ])
  if (results[0].status === 'fulfilled') maintenanceStats.value = results[0].value.stats ?? null
  else pageError.value = `维护统计: ${results[0].reason instanceof Error ? results[0].reason.message : '失败'}`
  if (results[1].status === 'fulfilled') maintenanceErrors.value = results[1].value.errors ?? []
  else maintenanceErrors.value = [`加载失败: ${results[1].reason instanceof Error ? results[1].reason.message : '未知'}`]
  if (results[2].status === 'fulfilled') compactionStats.value = results[2].value.stats ?? null
  lastLoaded.value = new Date().toLocaleString()
  setLoading('refresh', false)
}

onMounted(() => {
  if (isAdmin.value) void refreshObservability()
})
</script>

<template>
  <PermissionDenied v-if="!isAdmin" />
  <div v-else class="space-y-6">
    <div class="flex flex-wrap items-center justify-between gap-3">
      <div>
        <h2 class="text-sm font-semibold text-slate-800">运维操作</h2>
        <p class="mt-1 text-xs text-slate-500">最近观测刷新：{{ lastLoaded || '尚未加载' }}</p>
      </div>
      <button :disabled="loading.refresh" class="rounded-lg bg-slate-100 px-3 py-1.5 text-xs text-slate-600 hover:bg-slate-200 disabled:opacity-50" @click="refreshObservability">
        {{ loading.refresh ? '刷新中...' : '刷新观测' }}
      </button>
    </div>

    <p v-if="pageError" class="rounded-lg border border-red-200 bg-red-50 p-3 text-sm text-red-700">{{ pageError }}</p>

    <div class="grid grid-cols-1 gap-4 sm:grid-cols-3">
      <div class="rounded-xl border border-slate-200 bg-white p-6">
        <div class="mb-3 flex items-center gap-2"><HardDrive class="h-5 w-5 text-slate-500" /><h3 class="text-sm font-semibold">Flush</h3></div>
        <p class="mb-4 text-xs text-slate-500">将 MemTable 刷写到 SSTable</p>
        <button :disabled="loading.flush" class="w-full rounded-lg bg-slate-800 px-4 py-2 text-sm text-white disabled:opacity-50" @click="doFlush">{{ loading.flush ? '执行中...' : '执行 Flush' }}</button>
        <p v-if="flushResult" class="mt-2 text-xs" :class="flushResult.includes('成功') ? 'text-green-600' : 'text-red-600'">{{ flushResult }}</p>
      </div>
      <div class="rounded-xl border border-slate-200 bg-white p-6">
        <div class="mb-3 flex items-center gap-2"><ArrowDownWideNarrow class="h-5 w-5 text-slate-500" /><h3 class="text-sm font-semibold">Compact</h3></div>
        <p class="mb-4 text-xs text-slate-500">触发 Compaction</p>
        <button :disabled="loading.compact" class="w-full rounded-lg bg-slate-800 px-4 py-2 text-sm text-white disabled:opacity-50" @click="doCompact">{{ loading.compact ? '执行中...' : '执行 Compact' }}</button>
        <p v-if="compactResult" class="mt-2 break-all text-xs" :class="compactResult.includes('成功') ? 'text-green-600' : 'text-red-600'">{{ compactResult }}</p>
      </div>
      <div class="rounded-xl border border-slate-200 bg-white p-6">
        <div class="mb-3 flex items-center gap-2"><Archive class="h-5 w-5 text-slate-500" /><h3 class="text-sm font-semibold">保留策略</h3></div>
        <p class="mb-4 text-xs text-slate-500">应用保留策略删除过期数据</p>
        <button :disabled="loading.retention" class="w-full rounded-lg bg-slate-800 px-4 py-2 text-sm text-white disabled:opacity-50" @click="doApplyRetention">{{ loading.retention ? '执行中...' : '应用保留策略' }}</button>
        <p v-if="retentionResult" class="mt-2 break-all text-xs" :class="retentionResult.includes('成功') ? 'text-green-600' : 'text-red-600'">{{ retentionResult }}</p>
      </div>
    </div>

    <div class="rounded-xl border border-slate-200 bg-white p-6">
      <div class="mb-4 flex items-center gap-2"><Activity class="h-4 w-4 text-slate-500" /><h3 class="text-sm font-semibold">Compaction 详情</h3></div>
      <div v-if="!compactionStats" class="text-sm text-slate-400">暂无 compaction 统计</div>
      <div v-else class="grid grid-cols-2 gap-3 text-xs sm:grid-cols-3">
        <div class="rounded bg-slate-50 px-3 py-2">active: <span class="font-semibold">{{ compactionStats.active }}</span></div>
        <div class="rounded bg-slate-50 px-3 py-2">backlog: <span class="font-semibold">{{ compactionStats.backlog }}</span></div>
        <div class="rounded bg-slate-50 px-3 py-2">total: <span class="font-semibold">{{ compactionStats.total }}</span></div>
        <div class="rounded bg-slate-50 px-3 py-2">success: <span class="font-semibold text-green-700">{{ compactionStats.success }}</span></div>
        <div class="rounded bg-slate-50 px-3 py-2">failure: <span class="font-semibold text-red-700">{{ compactionStats.failure }}</span></div>
        <div class="rounded bg-slate-50 px-3 py-2">last_error: <span class="font-semibold">{{ compactionStats.last_error || '—' }}</span></div>
      </div>
    </div>

    <div class="rounded-xl border border-slate-200 bg-white p-6">
      <h3 class="mb-4 text-sm font-semibold">维护统计</h3>
      <div v-if="!maintenanceStats" class="text-sm text-slate-400">暂无统计</div>
      <div v-else class="grid grid-cols-2 gap-3 text-xs sm:grid-cols-3">
        <div class="rounded bg-slate-50 px-3 py-2">compact active: <span class="font-semibold">{{ maintenanceStats.compaction_active }}</span></div>
        <div class="rounded bg-slate-50 px-3 py-2">compact backlog: <span class="font-semibold">{{ maintenanceStats.compaction_backlog }}</span></div>
        <div class="rounded bg-slate-50 px-3 py-2">compact skipped: <span class="font-semibold">{{ maintenanceStats.compaction_skipped }}</span></div>
        <div class="rounded bg-slate-50 px-3 py-2">compact failure: <span class="font-semibold">{{ maintenanceStats.compaction_failure }}</span></div>
        <div class="rounded bg-slate-50 px-3 py-2">downsample active: <span class="font-semibold">{{ maintenanceStats.downsample_active }}</span></div>
        <div class="rounded bg-slate-50 px-3 py-2">downsample inflight: <span class="font-semibold">{{ maintenanceStats.downsample_inflight }}</span></div>
        <div class="rounded bg-slate-50 px-3 py-2">errors: <span class="font-semibold">{{ maintenanceStats.maintenance_error_count }}</span></div>
      </div>
      <p v-if="maintenanceStats?.compaction_last_skip" class="mt-3 text-xs text-amber-700">最近 compact 跳过：{{ maintenanceStats.compaction_last_skip }}</p>
    </div>

    <div class="rounded-xl border border-slate-200 bg-white p-6">
      <h3 class="mb-4 text-sm font-semibold">维护错误</h3>
      <div v-if="!maintenanceErrors.length" class="text-sm text-slate-400">暂无维护错误</div>
      <ul v-else class="space-y-1">
        <li v-for="(err, idx) in maintenanceErrors" :key="idx" class="rounded bg-red-50 px-3 py-1.5 text-xs text-red-700">{{ err }}</li>
      </ul>
    </div>
  </div>
</template>
