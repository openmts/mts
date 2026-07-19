<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount } from 'vue'
import { apiGet } from '@/api/client'
import { Activity, Cpu, HardDrive, Server, RefreshCw, Wrench } from 'lucide-vue-next'

interface HealthResponse { healthy: boolean; ready: boolean; reasons: string[] | null }
interface StorageMemorySnapshot {
  current_bytes: number
  peak_bytes?: number
  memtable_bytes: number
  wal_bytes: number
  query_bytes: number
  compaction_bytes: number
  rejected_writes?: number
  runtime_rss_bytes?: number
  runtime_heap_alloc_bytes?: number
}
interface StorageMemoryResponse { snapshot: StorageMemorySnapshot }
interface CompactionStats { active: number; backlog: number; total: number; success: number; failure: number; last_error: string }
interface CompactionStatsResponse { stats: CompactionStats }
interface MaintenanceStats {
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
interface MaintenanceStatsResponse { stats: MaintenanceStats }

const healthy = ref<boolean | null>(null)
const ready = ref<boolean | null>(null)
const healthReasons = ref<string[]>([])
const memorySnapshot = ref<StorageMemorySnapshot | null>(null)
const compactionStats = ref<CompactionStats | null>(null)
const maintenanceStats = ref<MaintenanceStats | null>(null)
const loadError = ref('')
const loading = ref(false)
const lastRefreshed = ref('')
const autoRefresh = ref(false)
let timer: ReturnType<typeof setInterval> | null = null

function formatBytes(bytes: number): string {
  if (bytes >= 1 << 30) return (bytes / (1 << 30)).toFixed(1) + ' GB'
  if (bytes >= 1 << 20) return (bytes / (1 << 20)).toFixed(1) + ' MB'
  if (bytes >= 1 << 10) return (bytes / (1 << 10)).toFixed(1) + ' KB'
  return bytes + ' B'
}

async function loadOverview() {
  loading.value = true
  loadError.value = ''
  try {
    const [healthData, memData, compData, maintData] = await Promise.all([
      apiGet<HealthResponse>('/healthz'),
      apiGet<StorageMemoryResponse>('/api/v1/admin/stats/storage-memory'),
      apiGet<CompactionStatsResponse>('/api/v1/admin/stats/compaction'),
      apiGet<MaintenanceStatsResponse>('/api/v1/admin/stats/maintenance'),
    ])
    healthy.value = healthData.healthy
    ready.value = healthData.ready
    healthReasons.value = healthData.reasons ?? []
    memorySnapshot.value = memData.snapshot
    compactionStats.value = compData.stats
    maintenanceStats.value = maintData.stats ?? null
    lastRefreshed.value = new Date().toLocaleString()
  } catch (e) {
    loadError.value = e instanceof Error ? e.message : '加载失败'
  } finally {
    loading.value = false
  }
}

function toggleAuto() {
  autoRefresh.value = !autoRefresh.value
  if (timer) {
    clearInterval(timer)
    timer = null
  }
  if (autoRefresh.value) {
    timer = setInterval(() => { void loadOverview() }, 15000)
  }
}

onMounted(() => { void loadOverview() })
onBeforeUnmount(() => { if (timer) clearInterval(timer) })
</script>

<template>
  <div>
    <div class="mb-4 flex flex-wrap items-center justify-between gap-3">
      <div>
        <h2 class="text-sm font-semibold text-slate-800">系统概览</h2>
        <p class="mt-1 text-xs text-slate-500">最近刷新：{{ lastRefreshed || '尚未加载' }}</p>
      </div>
      <div class="flex gap-2">
        <button class="rounded-lg border border-slate-200 bg-white px-3 py-1.5 text-xs" :class="autoRefresh ? 'border-blue-300 text-blue-700' : 'text-slate-700'" @click="toggleAuto">
          {{ autoRefresh ? '自动刷新: 开' : '自动刷新: 关' }}
        </button>
        <button :disabled="loading" class="inline-flex items-center gap-2 rounded-lg border border-slate-200 bg-white px-3 py-1.5 text-xs font-medium text-slate-700 disabled:opacity-50" @click="loadOverview">
          <RefreshCw class="h-3.5 w-3.5" :class="loading ? 'animate-spin' : ''" />
          {{ loading ? '刷新中...' : '刷新' }}
        </button>
      </div>
    </div>

    <p v-if="loadError" class="mb-4 rounded-lg border border-red-200 bg-red-50 p-3 text-sm text-red-700">{{ loadError }}</p>

    <div class="mb-6 grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
      <div class="rounded-xl border border-slate-200 bg-white p-4">
        <div class="flex items-center gap-3">
          <Server class="h-5 w-5 text-slate-500" />
          <div>
            <p class="text-xs text-slate-500">服务状态</p>
            <p v-if="healthy === null" class="text-sm text-slate-400">加载中...</p>
            <p v-else :class="healthy ? 'text-green-600' : 'text-red-600'" class="text-sm font-medium">{{ healthy ? '健康' : '异常' }}</p>
          </div>
        </div>
      </div>
      <div class="rounded-xl border border-slate-200 bg-white p-4">
        <div class="flex items-center gap-3">
          <Activity class="h-5 w-5 text-slate-500" />
          <div>
            <p class="text-xs text-slate-500">就绪状态</p>
            <p v-if="ready === null" class="text-sm text-slate-400">加载中...</p>
            <p v-else :class="ready ? 'text-green-600' : 'text-yellow-600'" class="text-sm font-medium">{{ ready ? '就绪' : '未就绪' }}</p>
          </div>
        </div>
      </div>
      <div class="rounded-xl border border-slate-200 bg-white p-4">
        <div class="flex items-center gap-3">
          <Cpu class="h-5 w-5 text-slate-500" />
          <div>
            <p class="text-xs text-slate-500">存储内存</p>
            <p v-if="!memorySnapshot" class="text-sm text-slate-400">加载中...</p>
            <p v-else class="text-sm font-medium text-slate-700">{{ formatBytes(memorySnapshot.current_bytes) }}</p>
          </div>
        </div>
      </div>
      <div class="rounded-xl border border-slate-200 bg-white p-4">
        <div class="flex items-center gap-3">
          <HardDrive class="h-5 w-5 text-slate-500" />
          <div>
            <p class="text-xs text-slate-500">压缩任务</p>
            <p v-if="!compactionStats" class="text-sm text-slate-400">加载中...</p>
            <p v-else class="text-sm font-medium text-slate-700">{{ compactionStats.active }} 活跃 / {{ compactionStats.total }} 总计</p>
          </div>
        </div>
      </div>
    </div>

    <div v-if="healthReasons.length" class="mb-6 rounded-xl border border-amber-200 bg-amber-50 p-4">
      <h3 class="mb-2 text-sm font-semibold text-amber-800">健康原因</h3>
      <ul class="space-y-1 text-xs text-amber-700">
        <li v-for="(reason, idx) in healthReasons" :key="idx">{{ reason }}</li>
      </ul>
    </div>

    <div v-if="memorySnapshot" class="mb-6 rounded-xl border border-slate-200 bg-white p-6">
      <h2 class="mb-4 text-sm font-semibold text-slate-800">存储内存详情</h2>
      <div class="grid grid-cols-2 gap-4 sm:grid-cols-4">
        <div><span class="text-xs text-slate-500">MemTable</span><p class="text-sm font-medium">{{ formatBytes(memorySnapshot.memtable_bytes) }}</p></div>
        <div><span class="text-xs text-slate-500">WAL</span><p class="text-sm font-medium">{{ formatBytes(memorySnapshot.wal_bytes) }}</p></div>
        <div><span class="text-xs text-slate-500">查询</span><p class="text-sm font-medium">{{ formatBytes(memorySnapshot.query_bytes) }}</p></div>
        <div><span class="text-xs text-slate-500">压缩</span><p class="text-sm font-medium">{{ formatBytes(memorySnapshot.compaction_bytes) }}</p></div>
        <div><span class="text-xs text-slate-500">Peak</span><p class="text-sm font-medium">{{ formatBytes(memorySnapshot.peak_bytes || 0) }}</p></div>
        <div><span class="text-xs text-slate-500">RSS</span><p class="text-sm font-medium">{{ formatBytes(memorySnapshot.runtime_rss_bytes || 0) }}</p></div>
        <div><span class="text-xs text-slate-500">Heap Alloc</span><p class="text-sm font-medium">{{ formatBytes(memorySnapshot.runtime_heap_alloc_bytes || 0) }}</p></div>
        <div><span class="text-xs text-slate-500">Rejected Writes</span><p class="text-sm font-medium">{{ memorySnapshot.rejected_writes || 0 }}</p></div>
      </div>
    </div>

    <div v-if="compactionStats" class="mb-6 rounded-xl border border-slate-200 bg-white p-6">
      <h2 class="mb-4 text-sm font-semibold text-slate-800">Compaction 统计</h2>
      <div class="grid grid-cols-2 gap-4 sm:grid-cols-4">
        <div><span class="text-xs text-slate-500">成功</span><p class="text-sm font-medium text-green-600">{{ compactionStats.success }}</p></div>
        <div><span class="text-xs text-slate-500">失败</span><p class="text-sm font-medium text-red-600">{{ compactionStats.failure }}</p></div>
        <div><span class="text-xs text-slate-500">积压</span><p class="text-sm font-medium text-yellow-600">{{ compactionStats.backlog }}</p></div>
        <div><span class="text-xs text-slate-500">活跃</span><p class="text-sm font-medium text-blue-600">{{ compactionStats.active }}</p></div>
      </div>
      <p v-if="compactionStats.last_error" class="mt-3 text-xs text-red-600">最近错误：{{ compactionStats.last_error }}</p>
    </div>

    <div class="rounded-xl border border-slate-200 bg-white p-6">
      <div class="mb-4 flex items-center gap-2">
        <Wrench class="h-4 w-4 text-slate-500" />
        <h2 class="text-sm font-semibold text-slate-800">维护统计</h2>
      </div>
      <div v-if="!maintenanceStats" class="text-sm text-slate-400">暂无维护统计</div>
      <div v-else class="grid grid-cols-2 gap-3 text-xs text-slate-600 sm:grid-cols-3 lg:grid-cols-4">
        <div class="rounded bg-slate-50 px-3 py-2">compact active: <span class="font-semibold text-slate-800">{{ maintenanceStats.compaction_active }}</span></div>
        <div class="rounded bg-slate-50 px-3 py-2">compact backlog: <span class="font-semibold text-slate-800">{{ maintenanceStats.compaction_backlog }}</span></div>
        <div class="rounded bg-slate-50 px-3 py-2">compact skipped: <span class="font-semibold text-slate-800">{{ maintenanceStats.compaction_skipped }}</span></div>
        <div class="rounded bg-slate-50 px-3 py-2">compact failure: <span class="font-semibold text-slate-800">{{ maintenanceStats.compaction_failure }}</span></div>
        <div class="rounded bg-slate-50 px-3 py-2">downsample active: <span class="font-semibold text-slate-800">{{ maintenanceStats.downsample_active }}</span></div>
        <div class="rounded bg-slate-50 px-3 py-2">downsample inflight: <span class="font-semibold text-slate-800">{{ maintenanceStats.downsample_inflight }}</span></div>
        <div class="rounded bg-slate-50 px-3 py-2">downsample failure: <span class="font-semibold text-slate-800">{{ maintenanceStats.downsample_failure }}</span></div>
        <div class="rounded bg-slate-50 px-3 py-2">errors: <span class="font-semibold text-slate-800">{{ maintenanceStats.maintenance_error_count }}</span></div>
      </div>
      <p v-if="maintenanceStats?.compaction_last_skip" class="mt-3 text-xs text-amber-700">最近 compact 跳过：{{ maintenanceStats.compaction_last_skip }}</p>
    </div>
  </div>
</template>
