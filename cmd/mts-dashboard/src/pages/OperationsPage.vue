<script setup lang="ts">
import { ref } from 'vue'
import { apiPost, apiGet } from '@/api/client'
import { HardDrive, ArrowDownWideNarrow, Archive } from 'lucide-vue-next'

interface OpResult { ok: boolean; result?: Record<string, unknown> }
interface MaintenanceErrorsResponse { errors: string[] }
interface MaintenanceStatsResponse {
  stats: {
    compaction_active: number
    compaction_backlog: number
    compaction_skipped: number
    compaction_failure: number
    downsample_active: number
    downsample_inflight: number
    downsample_skipped: number
    downsample_failure: number
    maintenance_error_count: number
  }
}

const flushResult = ref('')
const compactResult = ref('')
const retentionResult = ref('')
const maintenanceErrors = ref<string[]>([])
const maintenanceStats = ref<MaintenanceStatsResponse['stats'] | null>(null)
const actionLoading = ref('')

async function doFlush() {
  if (!confirm('确定执行此操作？')) return
  actionLoading.value = 'flush'
  try {
    const res = await apiPost<OpResult>('/api/v1/admin/flush')
    flushResult.value = res.ok ? 'Flush 执行成功' : 'Flush 执行失败'
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
  } catch (e) {
    retentionResult.value = `失败: ${e instanceof Error ? e.message : '未知错误'}`
  } finally {
    actionLoading.value = ''
  }
}

async function loadMaintenanceErrors() {
  actionLoading.value = 'errors'
  try {
    const res = await apiGet<MaintenanceErrorsResponse>('/api/v1/admin/maintenance/errors')
    maintenanceErrors.value = res.errors ?? []
  } catch (e) {
    maintenanceErrors.value = [`加载失败: ${e instanceof Error ? e.message : '未知错误'}`]
  } finally {
    actionLoading.value = ''
  }
}

async function loadMaintenanceStats() {
  actionLoading.value = 'stats'
  try {
    const res = await apiGet<MaintenanceStatsResponse>('/api/v1/admin/stats/maintenance')
    maintenanceStats.value = res.stats ?? null
  } catch (e) {
    maintenanceStats.value = null
    maintenanceErrors.value = [`维护统计加载失败: ${e instanceof Error ? e.message : '未知错误'}`]
  } finally {
    actionLoading.value = ''
  }
}
</script>

<template>
  <div class="space-y-6">
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
        <p v-if="compactResult" class="mt-2 text-xs" :class="!compactResult.includes('成功') ? 'text-red-600' : 'text-green-600'">{{ compactResult }}</p>
      </div>
      <div class="rounded-xl border border-slate-200 bg-white p-6">
        <div class="mb-3 flex items-center gap-2">
          <Archive class="h-5 w-5 text-slate-500" /><h3 class="text-sm font-semibold text-slate-800">保留策略</h3>
        </div>
        <p class="mb-4 text-xs text-slate-500">应用保留策略删除过期数据</p>
        <button :disabled="actionLoading === 'retention'" class="w-full rounded-lg bg-slate-800 px-4 py-2 text-sm font-medium text-white hover:bg-slate-700 disabled:opacity-50" @click="doApplyRetention">{{ actionLoading === 'retention' ? '执行中...' : '应用保留策略' }}</button>
        <p v-if="retentionResult" class="mt-2 text-xs" :class="!retentionResult.includes('成功') ? 'text-red-600' : 'text-green-600'">{{ retentionResult }}</p>
      </div>
    </div>
    <div class="rounded-xl border border-slate-200 bg-white p-6">
      <div class="mb-4 flex items-center justify-between">
        <h3 class="text-sm font-semibold text-slate-800">维护统计</h3>
        <button :disabled="actionLoading === 'stats'" class="rounded-lg bg-slate-100 px-3 py-1.5 text-xs text-slate-600 hover:bg-slate-200" @click="loadMaintenanceStats">{{ actionLoading === 'stats' ? '加载中...' : '刷新统计' }}</button>
      </div>
      <div v-if="!maintenanceStats" class="text-sm text-slate-400">暂无统计（点击刷新加载）</div>
      <div v-else class="grid grid-cols-2 gap-3 text-xs text-slate-600 sm:grid-cols-3">
        <div class="rounded bg-slate-50 px-3 py-2">compact active: <span class="font-semibold text-slate-800">{{ maintenanceStats.compaction_active }}</span></div>
        <div class="rounded bg-slate-50 px-3 py-2">compact backlog: <span class="font-semibold text-slate-800">{{ maintenanceStats.compaction_backlog }}</span></div>
        <div class="rounded bg-slate-50 px-3 py-2">compact failure: <span class="font-semibold text-slate-800">{{ maintenanceStats.compaction_failure }}</span></div>
        <div class="rounded bg-slate-50 px-3 py-2">downsample active: <span class="font-semibold text-slate-800">{{ maintenanceStats.downsample_active }}</span></div>
        <div class="rounded bg-slate-50 px-3 py-2">downsample inflight: <span class="font-semibold text-slate-800">{{ maintenanceStats.downsample_inflight }}</span></div>
        <div class="rounded bg-slate-50 px-3 py-2">errors: <span class="font-semibold text-slate-800">{{ maintenanceStats.maintenance_error_count }}</span></div>
      </div>
    </div>
    <div class="rounded-xl border border-slate-200 bg-white p-6">
      <div class="mb-4 flex items-center justify-between">
        <h3 class="text-sm font-semibold text-slate-800">维护错误</h3>
        <button :disabled="actionLoading === 'errors'" class="rounded-lg bg-slate-100 px-3 py-1.5 text-xs text-slate-600 hover:bg-slate-200" @click="loadMaintenanceErrors">{{ actionLoading === 'errors' ? '加载中...' : '刷新' }}</button>
      </div>
      <div v-if="!maintenanceErrors.length" class="text-sm text-slate-400">暂无维护错误（点击刷新加载）</div>
      <ul v-else class="space-y-1">
        <li v-for="(err, idx) in maintenanceErrors" :key="idx" class="rounded bg-red-50 px-3 py-1.5 text-xs text-red-700">{{ err }}</li>
      </ul>
    </div>
  </div>
</template>
