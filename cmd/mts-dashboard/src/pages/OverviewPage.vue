<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { apiGet } from '@/api/client'
import { Activity, Cpu, HardDrive, Server } from 'lucide-vue-next'

interface HealthResponse { healthy: boolean; ready: boolean; reasons: string[] | null }
interface StorageMemorySnapshot { current_bytes: number; memtable_bytes: number; wal_bytes: number; query_bytes: number; compaction_bytes: number }
interface StorageMemoryResponse { snapshot: StorageMemorySnapshot }
interface CompactionStats { active: number; backlog: number; total: number; success: number; failure: number; last_error: string }
interface CompactionStatsResponse { stats: CompactionStats }

const healthy = ref<boolean | null>(null)
const ready = ref<boolean | null>(null)
const healthReasons = ref<string[]>([])
const memorySnapshot = ref<StorageMemorySnapshot | null>(null)
const compactionStats = ref<CompactionStats | null>(null)
const loadError = ref('')

function formatBytes(bytes: number): string {
  if (bytes >= 1 << 30) return (bytes / (1 << 30)).toFixed(1) + ' GB'
  if (bytes >= 1 << 20) return (bytes / (1 << 20)).toFixed(1) + ' MB'
  if (bytes >= 1 << 10) return (bytes / (1 << 10)).toFixed(1) + ' KB'
  return bytes + ' B'
}

onMounted(async () => {
  try {
    const [healthData, memData, compData] = await Promise.all([
      apiGet<HealthResponse>('/healthz'),
      apiGet<StorageMemoryResponse>('/api/v1/admin/stats/storage-memory'),
      apiGet<CompactionStatsResponse>('/api/v1/admin/stats/compaction'),
    ])
    healthy.value = healthData.healthy
    ready.value = healthData.ready
    healthReasons.value = healthData.reasons ?? []
    memorySnapshot.value = memData.snapshot
    compactionStats.value = compData.stats
  } catch (e) {
    loadError.value = e instanceof Error ? e.message : '加载失败'
  }
})
</script>

<template>
  <div>
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
    <div v-if="memorySnapshot" class="mb-6 rounded-xl border border-slate-200 bg-white p-6">
      <h2 class="mb-4 text-sm font-semibold text-slate-800">存储内存详情</h2>
      <div class="grid grid-cols-2 gap-4 sm:grid-cols-4">
        <div><span class="text-xs text-slate-500">MemTable</span><p class="text-sm font-medium">{{ formatBytes(memorySnapshot.memtable_bytes) }}</p></div>
        <div><span class="text-xs text-slate-500">WAL</span><p class="text-sm font-medium">{{ formatBytes(memorySnapshot.wal_bytes) }}</p></div>
        <div><span class="text-xs text-slate-500">查询</span><p class="text-sm font-medium">{{ formatBytes(memorySnapshot.query_bytes) }}</p></div>
        <div><span class="text-xs text-slate-500">压缩</span><p class="text-sm font-medium">{{ formatBytes(memorySnapshot.compaction_bytes) }}</p></div>
      </div>
    </div>
    <div v-if="compactionStats" class="rounded-xl border border-slate-200 bg-white p-6">
      <h2 class="mb-4 text-sm font-semibold text-slate-800">Compaction 统计</h2>
      <div class="grid grid-cols-2 gap-4 sm:grid-cols-4">
        <div><span class="text-xs text-slate-500">成功</span><p class="text-sm font-medium text-green-600">{{ compactionStats.success }}</p></div>
        <div><span class="text-xs text-slate-500">失败</span><p class="text-sm font-medium text-red-600">{{ compactionStats.failure }}</p></div>
        <div><span class="text-xs text-slate-500">积压</span><p class="text-sm font-medium text-yellow-600">{{ compactionStats.backlog }}</p></div>
        <div><span class="text-xs text-slate-500">活跃</span><p class="text-sm font-medium text-blue-600">{{ compactionStats.active }}</p></div>
      </div>
      <p v-if="compactionStats.last_error" class="mt-3 text-xs text-red-500">最近错误: {{ compactionStats.last_error }}</p>
    </div>
    <div v-if="healthReasons.length" class="mt-6 rounded-xl border border-slate-200 bg-white p-6">
      <h2 class="mb-2 text-sm font-semibold text-slate-800">健康检查详情</h2>
      <ul class="list-inside list-disc text-sm text-slate-600">
        <li v-for="reason in healthReasons" :key="reason">{{ reason }}</li>
      </ul>
    </div>
  </div>
</template>
