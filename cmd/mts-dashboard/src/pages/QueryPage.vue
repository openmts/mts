<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount, watch } from 'vue'
import { apiPost, apiGet, apiPostText, APIClientError } from '@/api/client'
import { Search, Database, Hash, Clock, Filter, Layers, Zap, BarChart3, Timer, Square, Copy, Check } from 'lucide-vue-next'

interface QueryResultRow {
  series_id: number; measurement: string; tags: Record<string, string>
  timestamp: number; fields: Record<string, unknown>
}
interface QueryStatsData {
  candidate_shards: number; shards_scanned: number; shards_skipped: number
  parts_scanned: number; parts_skipped: number; samples_read: number
  samples_returned: number; duration_nanos: number; errors: number
}
interface RowsResponse { rows: QueryResultRow[] }
interface StatsResponse { stats: QueryStatsData }
interface DatabaseListResponse { measurements: string[] }
interface MeasurementListResponse { measurements: string[] }
interface StreamRecord {
  type?: string
  row?: QueryResultRow
  column?: unknown
  stats?: QueryStatsData
  error?: { code?: string; message?: string }
}

const databases = ref<string[]>([])
const measurements = ref<string[]>([])
const retentionPolicies = ref<string[]>([])
const measurementsLoading = ref(false)
const queryForm = ref({ database: '', retention_policy: 'autogen', measurement: '', start_time: '', end_time: '', fields: '', limit: '100' })
const queryMode = ref<'rows' | 'columns' | 'explain' | 'stream-row' | 'stream-column'>('rows')
const rows = ref<QueryResultRow[]>([])
const queryStats = ref<QueryStatsData | null>(null)
const rawOutput = ref('')
const streamMeta = ref({ lines: 0, records: 0, errors: 0 })
const actionError = ref('')
const loading = ref(false)
const copyState = ref<'idle' | 'ok' | 'fail'>('idle')
let queryAbort: AbortController | null = null
let copyTimer: ReturnType<typeof setTimeout> | null = null

onMounted(async () => {
  try {
    const data = await apiGet<DatabaseListResponse>('/api/v1/admin/databases')
    databases.value = (data.measurements ?? []).sort()
    if (databases.value.length) {
      queryForm.value.database = databases.value[0]
    }
  } catch (_) { /* 非关键 */ }
})

onBeforeUnmount(() => {
  cancelQuery()
  if (copyTimer) clearTimeout(copyTimer)
})

watch(() => queryForm.value.database, async (db) => {
  measurements.value = []
  retentionPolicies.value = []
  queryForm.value.measurement = ''
  queryForm.value.retention_policy = 'autogen'
  if (!db) return
  measurementsLoading.value = true
  try {
    const [measData, rpData] = await Promise.all([
      apiGet<MeasurementListResponse>(`/api/v1/data/databases/${encodeURIComponent(db)}/measurements`),
      apiGet<{ policies: { name: string }[] }>(`/api/v1/admin/databases/${encodeURIComponent(db)}/retention-policies`),
    ])
    measurements.value = (measData.measurements ?? []).sort()
    retentionPolicies.value = (rpData.policies ?? []).map((p) => p.name)
    if (measurements.value.length) {
      queryForm.value.measurement = measurements.value[0]
    }
  } catch (_) { /* 忽略 */ }
  finally { measurementsLoading.value = false }
})

function buildQuery(): Record<string, unknown> {
  const query: Record<string, unknown> = {}
  if (queryForm.value.database) query.database = queryForm.value.database
  if (queryForm.value.retention_policy) query.retention_policy = queryForm.value.retention_policy
  if (queryForm.value.measurement) query.measurement = queryForm.value.measurement
  if (queryForm.value.start_time) query.start_time = parseInt(queryForm.value.start_time)
  if (queryForm.value.end_time) query.end_time = parseInt(queryForm.value.end_time)
  if (queryForm.value.fields) query.fields = queryForm.value.fields.split(',').map((s) => s.trim()).filter(Boolean)
  if (queryForm.value.limit) query.limit = parseInt(queryForm.value.limit)
  return query
}

function cancelQuery() {
  if (queryAbort) {
    queryAbort.abort()
    queryAbort = null
  }
}

function beginRequest(): AbortSignal {
  cancelQuery()
  queryAbort = new AbortController()
  return queryAbort.signal
}

function parseStreamText(text: string) {
  const lines = text.split('\n').map((line) => line.trim()).filter(Boolean)
  let records = 0
  let errors = 0
  let endStats: QueryStatsData | null = null
  let streamError = ''
  for (const line of lines) {
    try {
      const rec = JSON.parse(line) as StreamRecord
      const type = rec.type ?? ''
      if (type === 'row' || type === 'column') {
        records += 1
      } else if (type === 'end' && rec.stats) {
        endStats = rec.stats
      } else if (type === 'error') {
        errors += 1
        streamError = rec.error?.message || streamError || '流式查询错误'
      }
    } catch (_) {
      errors += 1
    }
  }
  streamMeta.value = { lines: lines.length, records, errors }
  if (endStats) {
    queryStats.value = endStats
  }
  if (streamError) {
    actionError.value = streamError
  }
}

async function executeQuery() {
  actionError.value = ''
  loading.value = true
  rows.value = []
  queryStats.value = null
  rawOutput.value = ''
  streamMeta.value = { lines: 0, records: 0, errors: 0 }
  copyState.value = 'idle'
  const query = buildQuery()
  const signal = beginRequest()
  try {
    if (queryMode.value === 'rows') {
      const data = await apiPost<RowsResponse>('/api/v1/data/query/rows', { query }, { signal })
      rows.value = data.rows ?? []
    } else if (queryMode.value === 'explain') {
      const data = await apiPost<{ result: { columns: QueryResultRow[]; explain: Record<string, unknown>; stats: QueryStatsData } }>('/api/v1/data/query/explain', { query }, { signal })
      rows.value = (data.result?.columns as QueryResultRow[]) ?? []
      queryStats.value = data.result?.stats ?? null
      if (data.result?.explain) {
        rawOutput.value = JSON.stringify(data.result.explain, null, 2)
      }
    } else if (queryMode.value === 'columns') {
      const data = await apiPost<{ columns: unknown[] }>('/api/v1/data/query/columns', { query }, { signal })
      rawOutput.value = JSON.stringify(data.columns, null, 2)
    } else if (queryMode.value === 'stream-row' || queryMode.value === 'stream-column') {
      const format = queryMode.value === 'stream-column' ? 'column' : 'row'
      const { text } = await apiPostText('/api/v1/data/query/stream', { query, format }, { signal })
      rawOutput.value = text
      parseStreamText(text)
    }
  } catch (e) {
    if (e instanceof APIClientError && e.code === 'canceled') {
      actionError.value = '查询已取消'
    } else {
      actionError.value = e instanceof Error ? e.message : '查询失败'
    }
  } finally {
    loading.value = false
    queryAbort = null
  }
  if (!loading.value && actionError.value !== '查询已取消') {
    try {
      const statsData = await apiGet<StatsResponse>('/api/v1/data/query/stats')
      if (statsData.stats) {
        queryStats.value = statsData.stats
      }
    } catch (_) { /* 非关键 */ }
  }
}

function formatTimestamp(ns: number): string { return new Date(ns / 1e6).toISOString() }

const modeOptions = [
  { value: 'rows' as const, label: '行式查询', desc: '按行返回数据' },
  { value: 'columns' as const, label: '列式查询', desc: '按列返回数据' },
  { value: 'explain' as const, label: 'EXPLAIN', desc: '含执行计划' },
  { value: 'stream-row' as const, label: '流式行', desc: 'NDJSON 行流' },
  { value: 'stream-column' as const, label: '流式列', desc: 'NDJSON 列流（推荐宽表）' },
]

const deleteResult = ref('')
const deleteLoading = ref(false)

async function executeDelete() {
  actionError.value = ''
  deleteResult.value = ''
  if (!queryForm.value.database || !queryForm.value.measurement) {
    actionError.value = '删除需要数据库和 measurement'
    return
  }
  if (!queryForm.value.start_time || !queryForm.value.end_time) {
    actionError.value = '删除需要 start_time 与 end_time'
    return
  }
  if (!confirm('确认按当前时间范围删除匹配数据？该操作通过 tombstone 生效。')) return
  deleteLoading.value = true
  try {
    const request: Record<string, unknown> = {
      database: queryForm.value.database,
      retention_policy: queryForm.value.retention_policy || 'autogen',
      measurement: queryForm.value.measurement,
      start_time: parseInt(queryForm.value.start_time),
      end_time: parseInt(queryForm.value.end_time),
    }
    await apiPost('/api/v1/data/delete', { request })
    deleteResult.value = '删除请求已提交'
  } catch (e) {
    actionError.value = e instanceof Error ? e.message : '删除失败'
  } finally {
    deleteLoading.value = false
  }
}

function resultTextForCopy(): string {
  if (rawOutput.value) return rawOutput.value
  if (rows.value.length) return JSON.stringify(rows.value, null, 2)
  return ''
}

async function copyResults() {
  const text = resultTextForCopy()
  if (!text) {
    actionError.value = '暂无可复制内容'
    return
  }
  try {
    if (navigator.clipboard?.writeText) {
      await navigator.clipboard.writeText(text)
    } else {
      const area = document.createElement('textarea')
      area.value = text
      area.style.position = 'fixed'
      area.style.left = '-9999px'
      document.body.appendChild(area)
      area.select()
      const ok = document.execCommand('copy')
      document.body.removeChild(area)
      if (!ok) throw new Error('copy failed')
    }
    copyState.value = 'ok'
  } catch (_) {
    copyState.value = 'fail'
  }
  if (copyTimer) clearTimeout(copyTimer)
  copyTimer = setTimeout(() => { copyState.value = 'idle' }, 2000)
}
</script>

<template>
  <div class="space-y-6">
    <p v-if="actionError" class="rounded-xl border border-red-200 bg-red-50 p-4 text-sm text-red-700">{{ actionError }}</p>
    <p v-if="deleteResult" class="rounded-xl border border-green-200 bg-green-50 p-4 text-sm text-green-700">{{ deleteResult }}</p>

    <!-- 查询条件 -->
    <div class="overflow-hidden rounded-2xl border border-slate-200 bg-white shadow-sm">
      <div class="border-b border-slate-100 bg-slate-50/50 px-6 py-4">
        <h3 class="text-sm font-semibold text-slate-800">查询条件</h3>
      </div>
      <div class="p-6">
        <div class="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
          <div class="sm:col-span-2 lg:col-span-2">
            <label class="mb-1.5 flex items-center gap-1.5 text-xs font-medium text-slate-500">
              <Database class="h-3 w-3" />数据库 & Measurement
            </label>
            <div class="flex gap-2">
              <select v-model="queryForm.database" class="flex-1 rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm text-slate-700 shadow-sm transition focus:border-slate-400 focus:outline-none focus:ring-2 focus:ring-slate-100 disabled:bg-slate-50 disabled:text-slate-400">
                <option v-if="!databases.length" value="" disabled>无数据</option>
                <option v-for="db in databases" :key="db" :value="db">{{ db }}</option>
              </select>
              <select v-model="queryForm.measurement" :disabled="!queryForm.database || measurementsLoading || (!measurementsLoading && !measurements.length)" class="flex-1 rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm text-slate-700 shadow-sm transition focus:border-slate-400 focus:outline-none focus:ring-2 focus:ring-slate-100 disabled:bg-slate-50 disabled:text-slate-400">
                <option v-if="!queryForm.database" value="" disabled>请先选择数据库</option>
                <option v-else-if="measurementsLoading" value="" disabled>加载中...</option>
                <option v-else-if="!measurements.length" value="" disabled>无数据</option>
                <option v-for="m in measurements" v-else :key="m" :value="m">{{ m }}</option>
              </select>
            </div>
          </div>
          <div>
            <label class="mb-1.5 flex items-center gap-1.5 text-xs font-medium text-slate-500">
              <Clock class="h-3 w-3" />保留策略
            </label>
            <select v-model="queryForm.retention_policy" :disabled="!queryForm.database || measurementsLoading" class="w-full rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm text-slate-700 shadow-sm transition focus:border-slate-400 focus:outline-none focus:ring-2 focus:ring-slate-100 disabled:bg-slate-50 disabled:text-slate-400">
              <option v-if="!queryForm.database" value="autogen" disabled>请先选择数据库</option>
              <option v-else-if="measurementsLoading" value="autogen" disabled>加载中...</option>
              <option value="autogen">autogen</option>
              <option v-for="rp in retentionPolicies" :key="rp" :value="rp">{{ rp }}</option>
            </select>
          </div>
          <div>
            <label class="mb-1.5 flex items-center gap-1.5 text-xs font-medium text-slate-500">
              <Hash class="h-3 w-3" />Limit
            </label>
            <input v-model="queryForm.limit" type="number" min="1" class="w-full rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm text-slate-700 shadow-sm transition focus:border-slate-400 focus:outline-none focus:ring-2 focus:ring-slate-100" />
          </div>
          <div>
            <label class="mb-1.5 flex items-center gap-1.5 text-xs font-medium text-slate-500">
              <Timer class="h-3 w-3" />Start Time (ns)
            </label>
            <input v-model="queryForm.start_time" type="text" placeholder="可选" class="w-full rounded-lg border border-slate-200 bg-white px-3 py-2 font-mono text-sm text-slate-700 shadow-sm transition focus:border-slate-400 focus:outline-none focus:ring-2 focus:ring-slate-100" />
          </div>
          <div>
            <label class="mb-1.5 flex items-center gap-1.5 text-xs font-medium text-slate-500">
              <Timer class="h-3 w-3" />End Time (ns)
            </label>
            <input v-model="queryForm.end_time" type="text" placeholder="可选" class="w-full rounded-lg border border-slate-200 bg-white px-3 py-2 font-mono text-sm text-slate-700 shadow-sm transition focus:border-slate-400 focus:outline-none focus:ring-2 focus:ring-slate-100" />
          </div>
          <div class="sm:col-span-2">
            <label class="mb-1.5 flex items-center gap-1.5 text-xs font-medium text-slate-500">
              <Filter class="h-3 w-3" />Fields（逗号分隔）
            </label>
            <input v-model="queryForm.fields" type="text" placeholder="例如 value,cpu" class="w-full rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm text-slate-700 shadow-sm transition focus:border-slate-400 focus:outline-none focus:ring-2 focus:ring-slate-100" />
          </div>
        </div>

        <div class="mt-5">
          <label class="mb-2 block text-xs font-medium text-slate-500">查询模式</label>
          <div class="flex flex-wrap gap-2">
            <button
              v-for="opt in modeOptions"
              :key="opt.value"
              type="button"
              class="rounded-lg border px-3 py-1.5 text-xs transition"
              :class="queryMode === opt.value ? 'border-slate-800 bg-slate-800 text-white' : 'border-slate-200 bg-white text-slate-600 hover:bg-slate-50'"
              @click="queryMode = opt.value"
            >
              <span class="font-medium">{{ opt.label }}</span>
              <span class="ml-1 opacity-70">{{ opt.desc }}</span>
            </button>
          </div>
        </div>

        <div class="mt-6 flex flex-wrap items-center gap-3">
          <button
            :disabled="loading"
            class="flex items-center gap-2 rounded-lg bg-slate-800 px-4 py-2 text-sm font-medium text-white shadow-sm transition hover:bg-slate-700 disabled:opacity-50"
            @click="executeQuery"
          >
            <Search class="h-4 w-4" />
            {{ loading ? '查询中...' : '执行查询' }}
          </button>
          <button
            :disabled="!loading"
            class="flex items-center gap-2 rounded-lg border border-amber-200 bg-white px-4 py-2 text-sm font-medium text-amber-700 shadow-sm transition hover:bg-amber-50 disabled:opacity-50"
            @click="cancelQuery"
          >
            <Square class="h-4 w-4" />
            取消查询
          </button>
          <button
            :disabled="deleteLoading || !queryForm.database || !queryForm.measurement"
            class="flex items-center gap-2 rounded-lg border border-red-200 bg-white px-4 py-2 text-sm font-medium text-red-600 shadow-sm transition hover:bg-red-50 disabled:opacity-50"
            @click="executeDelete"
          >
            {{ deleteLoading ? '删除中...' : '按范围删除' }}
          </button>
          <button
            :disabled="!rows.length && !rawOutput"
            class="flex items-center gap-2 rounded-lg border border-slate-200 bg-white px-4 py-2 text-sm font-medium text-slate-700 shadow-sm transition hover:bg-slate-50 disabled:opacity-50"
            @click="copyResults"
          >
            <Check v-if="copyState === 'ok'" class="h-4 w-4 text-green-600" />
            <Copy v-else class="h-4 w-4" />
            {{ copyState === 'ok' ? '已复制' : copyState === 'fail' ? '复制失败' : '复制结果' }}
          </button>
        </div>
      </div>
    </div>

    <!-- 查询统计 -->
    <div v-if="queryStats" class="overflow-hidden rounded-2xl border border-slate-200 bg-white shadow-sm">
      <div class="border-b border-slate-100 bg-slate-50/50 px-6 py-3">
        <h3 class="flex items-center gap-2 text-sm font-semibold text-slate-800">
          <BarChart3 class="h-4 w-4 text-slate-400" />查询统计
        </h3>
      </div>
      <div class="grid grid-cols-2 gap-px bg-slate-100 sm:grid-cols-5">
        <div class="bg-white p-4">
          <p class="text-[11px] font-medium uppercase tracking-wide text-slate-400">扫描 Shards</p>
          <p class="mt-1 text-lg font-semibold text-slate-800">{{ queryStats.shards_scanned }}</p>
        </div>
        <div class="bg-white p-4">
          <p class="text-[11px] font-medium uppercase tracking-wide text-slate-400">跳过 Shards</p>
          <p class="mt-1 text-lg font-semibold text-slate-800">{{ queryStats.shards_skipped }}</p>
        </div>
        <div class="bg-white p-4">
          <p class="text-[11px] font-medium uppercase tracking-wide text-slate-400">读取样本</p>
          <p class="mt-1 text-lg font-semibold text-blue-600">{{ queryStats.samples_read }}</p>
        </div>
        <div class="bg-white p-4">
          <p class="text-[11px] font-medium uppercase tracking-wide text-slate-400">返回样本</p>
          <p class="mt-1 text-lg font-semibold text-green-600">{{ queryStats.samples_returned }}</p>
        </div>
        <div class="bg-white p-4">
          <p class="text-[11px] font-medium uppercase tracking-wide text-slate-400">耗时</p>
          <p class="mt-1 text-lg font-semibold text-amber-600">{{ (queryStats.duration_nanos / 1e6).toFixed(1) }}<span class="text-sm font-normal text-slate-400">ms</span></p>
        </div>
      </div>
    </div>

    <!-- 查询结果 -->
    <div v-if="rows.length" class="overflow-hidden rounded-2xl border border-slate-200 bg-white shadow-sm">
      <div class="flex items-center justify-between border-b border-slate-100 bg-slate-50/50 px-6 py-3">
        <h3 class="flex items-center gap-2 text-sm font-semibold text-slate-800">
          <Layers class="h-4 w-4 text-slate-400" />查询结果
        </h3>
        <span class="rounded-full bg-slate-100 px-2.5 py-0.5 text-xs font-medium text-slate-500">{{ rows.length }} 行</span>
      </div>
      <div class="overflow-x-auto">
        <table class="w-full text-sm">
          <thead>
            <tr class="border-b border-slate-100 bg-slate-50/30 text-left">
              <th class="px-6 py-2.5 text-[11px] font-semibold uppercase tracking-wider text-slate-500">时间</th>
              <th class="px-6 py-2.5 text-[11px] font-semibold uppercase tracking-wider text-slate-500">Measurement</th>
              <th class="px-6 py-2.5 text-[11px] font-semibold uppercase tracking-wider text-slate-500">Tags</th>
              <th class="px-6 py-2.5 text-[11px] font-semibold uppercase tracking-wider text-slate-500">Fields</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-slate-50">
            <tr v-for="(row, idx) in rows" :key="idx" class="transition hover:bg-slate-50/50">
              <td class="px-6 py-3 font-mono text-xs text-slate-600">{{ formatTimestamp(row.timestamp) }}</td>
              <td class="px-6 py-3">
                <span class="rounded-md bg-blue-50 px-2 py-0.5 text-xs font-medium text-blue-700">{{ row.measurement }}</span>
              </td>
              <td class="px-6 py-3 font-mono text-xs text-slate-500">
                <span v-if="row.tags && Object.keys(row.tags).length">{{ JSON.stringify(row.tags) }}</span>
                <span v-else class="text-slate-300">—</span>
              </td>
              <td class="px-6 py-3 font-mono text-xs text-slate-600">{{ JSON.stringify(row.fields) }}</td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <!-- 原始输出 / 流式结果 -->
    <div v-if="rawOutput" class="overflow-hidden rounded-2xl border border-slate-200 bg-white shadow-sm">
      <div class="flex flex-wrap items-center justify-between gap-2 border-b border-slate-100 bg-slate-50/50 px-6 py-3">
        <h3 class="flex items-center gap-2 text-sm font-semibold text-slate-800">
          <Zap class="h-4 w-4 text-slate-400" />原始输出
        </h3>
        <div class="flex flex-wrap items-center gap-2 text-xs text-slate-500">
          <span v-if="streamMeta.lines" class="rounded-full bg-slate-100 px-2.5 py-0.5">{{ streamMeta.lines }} 行 NDJSON</span>
          <span v-if="streamMeta.records" class="rounded-full bg-blue-50 px-2.5 py-0.5 text-blue-700">{{ streamMeta.records }} 条记录</span>
          <span v-if="streamMeta.errors" class="rounded-full bg-red-50 px-2.5 py-0.5 text-red-600">{{ streamMeta.errors }} 错误</span>
          <button
            class="rounded-lg border border-slate-200 bg-white px-2.5 py-1 text-xs font-medium text-slate-600 hover:bg-slate-50"
            @click="copyResults"
          >复制</button>
        </div>
      </div>
      <pre class="max-h-96 overflow-auto bg-slate-950 p-6 font-mono text-xs leading-relaxed text-emerald-400">{{ rawOutput }}</pre>
    </div>
  </div>
</template>
