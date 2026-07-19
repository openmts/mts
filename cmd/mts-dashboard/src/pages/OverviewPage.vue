<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount, computed } from 'vue'
import { apiGet } from '@/api/client'
import { useAuth } from '@/composables/useAuth'
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

const { isAdmin } = useAuth()
const healthy = ref<boolean | null>(null)
const ready = ref<boolean | null>(null)
const healthReasons = ref<string[]>([])
const memorySnapshot = ref<StorageMemorySnapshot | null>(null)
const compactionStats = ref<CompactionStats | null>(null)
const maintenanceStats = ref<MaintenanceStats | null>(null)
const loadError = ref('')
const adminPartialError = ref('')
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
  adminPartialError.value = ''
  try {
    const healthData = await apiGet<HealthResponse>('/healthz')
    healthy.value = healthData.healthy
    ready.value = healthData.ready
    healthReasons.value = healthData.reasons ?? []

    if (isAdmin.value) {
      const results = await Promise.allSettled([
        apiGet<StorageMemoryResponse>('/api/v1/admin/stats/storage-memory'),
        apiGet<CompactionStatsResponse>('/api/v1/admin/stats/compaction'),
        apiGet<MaintenanceStatsResponse>('/api/v1/admin/stats/maintenance'),
      ])
      const errs: string[] = []
      if (results[0].status === 'fulfilled') memorySnapshot.value = results[0].value.snapshot
      else {
        memorySnapshot.value = null
        errs.push(results[0].reason instanceof Error ? results[0].reason.message : '内存统计失败')
      }
      if (results[1].status === 'fulfilled') compactionStats.value = results[1].value.stats
      else {
        compactionStats.value = null
        errs.push(results[1].reason instanceof Error ? results[1].reason.message : '压缩统计失败')
      }
      if (results[2].status === 'fulfilled') maintenanceStats.value = results[2].value.stats ?? null
      else {
        maintenanceStats.value = null
        errs.push(results[2].reason instanceof Error ? results[2].reason.message : '维护统计失败')
      }
      if (errs.length) adminPartialError.value = errs.join('；')
    } else {
      memorySnapshot.value = null
      compactionStats.value = null
      maintenanceStats.value = null
    }
    lastRefreshed.value = new Date().toLocaleTimeString()
  } catch (e) {
    loadError.value = e instanceof Error ? e.message : '加载失败'
  } finally {
    loading.value = false
  }
}

function toggleAutoRefresh() {
  autoRefresh.value = !autoRefresh.value
  if (timer) {
    clearInterval(timer)
    timer = null
  }
  if (autoRefresh.value) {
    timer = setInterval(() => { void loadOverview() }, 10000)
  }
}

onMounted(() => { void loadOverview() })
onBeforeUnmount(() => {
  if (timer) clearInterval(timer)
})

const showAdminPanels = computed(() => isAdmin.value)
</script>

<template>
  <div class="space-y-6">
    <div class="flex flex-wrap items-center justify-between gap-3">
      <div>
        <h1 class="text-lg font-semibold text-slate-800">概览</h1>
        <p class="text-xs text-slate-500">服务健康与运行快照</p>
      </div>
      <div class="flex items-center gap-2">
        <span v-if="lastRefreshed" class="text-xs text-slate-400">刷新于 {{ lastRefreshed }}</span>
        <button class="inline-flex items-center gap-1 rounded-lg border border-slate-200 bg-white px-3 py-1.5 text-xs text-slate-600 hover:bg-slate-50" @click="toggleAutoRefresh">
          <RefreshCw class="h-3.5 w-3.5" :class="autoRefresh ? 'animate-spin' : ''" />
          {{ autoRefresh ? '自动刷新中' : '自动刷新' }}
        </button>
        <button class="inline-flex items-center gap-1 rounded-lg bg-slate-800 px-3 py-1.5 text-xs font-medium text-white hover:bg-slate-700" :disabled="loading" @click="loadOverview">
          <RefreshCw class="h-3.5 w-3.5" /> 刷新
        </button>
      </div>
    </div>

    <p v-if="loadError" class="rounded-xl border border-red-200 bg-red-50 p-4 text-sm text-red-700">{{ loadError }}</p>
    <p v-else-if="adminPartialError" class="rounded-xl border border-amber-200 bg-amber-50 p-3 text-sm text-amber-800">部分管理统计不可用：{{ adminPartialError }}</p>

    <div class="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
      <div class="rounded-xl border border-slate-200 bg-white p-5">
        <div class="mb-2 flex items-center gap-2 text-slate-500"><Activity class="h-4 w-4" /><span class="text-xs">健康</span></div>
        <p class="text-2xl font-semibold" :class="healthy ? 'text-green-600' : healthy === false ? 'text-red-600' : 'text-slate-400'">
          {{ healthy === null ? '—' : healthy ? 'Healthy' : 'Unhealthy' }}
        </p>
      </div>
      <div class="rounded-xl border border-slate-200 bg-white p-5">
        <div class="mb-2 flex items-center gap-2 text-slate-500"><Server class="h-4 w-4" /><span class="text-xs">就绪</span></div>
        <p class="text-2xl font-semibold" :class="ready ? 'text-green-600' : ready === false ? 'text-red-600' : 'text-slate-400'">
          {{ ready === null ? '—' : ready ? 'Ready' : 'Not Ready' }}
        </p>
      </div>
      <div class="rounded-xl border border-slate-200 bg-white p-5 sm:col-span-2">
        <div class="mb-2 flex items-center gap-2 text-slate-500"><HardDrive class="h-4 w-4" /><span class="text-xs">原因</span></div>
        <p v-if="!healthReasons.length" class="text-sm text-slate-400">无</p>
        <ul v-else class="list-disc pl-5 text-sm text-slate-600">
          <li v-for="(r, i) in healthReasons" :key="i">{{ r }}</li>
        </ul>
      </div>
    </div>

    <div v-if="!showAdminPanels" class="rounded-xl border border-slate-200 bg-slate-50 p-4 text-sm text-slate-600">
      当前为普通用户：仅展示健康检查。内存/压缩/维护统计需管理员权限。
    </div>

    <template v-else>
      <div class="rounded-xl border border-slate-200 bg-white p-6">
        <div class="mb-4 flex items-center gap-2">
          <Cpu class="h-4 w-4 text-slate-500" />
          <h2 class="text-sm font-semibold text-slate-800">存储内存</h2>
        </div>
        <div v-if="!memorySnapshot" class="text-sm text-slate-400">暂无内存统计</div>
        <div v-else class="grid grid-cols-2 gap-3 text-xs text-slate-600 sm:grid-cols-3 lg:grid-cols-4">
          <div class="rounded bg-slate-50 px-3 py-2">current: <span class="font-semibold text-slate-800">{{ formatBytes(memorySnapshot.current_bytes) }}</span></div>
          <div class="rounded bg-slate-50 px-3 py-2">peak: <span class="font-semibold text-slate-800">{{ formatBytes(memorySnapshot.peak_bytes ?? 0) }}</span></div>
          <div class="rounded bg-slate-50 px-3 py-2">memtable: <span class="font-semibold text-slate-800">{{ formatBytes(memorySnapshot.memtable_bytes) }}</span></div>
          <div class="rounded bg-slate-50 px-3 py-2">wal: <span class="font-semibold text-slate-800">{{ formatBytes(memorySnapshot.wal_bytes) }}</span></div>
          <div class="rounded bg-slate-50 px-3 py-2">query: <span class="font-semibold text-slate-800">{{ formatBytes(memorySnapshot.query_bytes) }}</span></div>
          <div class="rounded bg-slate-50 px-3 py-2">compaction: <span class="font-semibold text-slate-800">{{ formatBytes(memorySnapshot.compaction_bytes) }}</span></div>
          <div class="rounded bg-slate-50 px-3 py-2">rss: <span class="font-semibold text-slate-800">{{ formatBytes(memorySnapshot.runtime_rss_bytes ?? 0) }}</span></div>
          <div class="rounded bg-slate-50 px-3 py-2">heap: <span class="font-semibold text-slate-800">{{ formatBytes(memorySnapshot.runtime_heap_alloc_bytes ?? 0) }}</span></div>
        </div>
      </div>

      <div class="rounded-xl border border-slate-200 bg-white p-6">
        <div class="mb-4 flex items-center gap-2">
          <HardDrive class="h-4 w-4 text-slate-500" />
          <h2 class="text-sm font-semibold text-slate-800">压缩统计</h2>
        </div>
        <div v-if="!compactionStats" class="text-sm text-slate-400">暂无压缩统计</div>
        <div v-else class="grid grid-cols-2 gap-4 sm:grid-cols-3 lg:grid-cols-5">
          <div><span class="text-xs text-slate-500">总计</span><p class="text-sm font-medium">{{ compactionStats.total }}</p></div>
          <div><span class="text-xs text-slate-500">成功</span><p class="text-sm font-medium text-green-600">{{ compactionStats.success }}</p></div>
          <div><span class="text-xs text-slate-500">失败</span><p class="text-sm font-medium text-red-600">{{ compactionStats.failure }}</p></div>
          <div><span class="text-xs text-slate-500">积压</span><p class="text-sm font-medium text-yellow-600">{{ compactionStats.backlog }}</p></div>
          <div><span class="text-xs text-slate-500">活跃</span><p class="text-sm font-medium text-blue-600">{{ compactionStats.active }}</p></div>
        </div>
        <p v-if="compactionStats?.last_error" class="mt-3 text-xs text-red-600">最近错误：{{ compactionStats.last_error }}</p>
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
    </template>
  </div>
</template>
