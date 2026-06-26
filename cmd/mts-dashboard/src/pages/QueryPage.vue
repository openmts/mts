<script setup lang="ts">
import { ref } from 'vue'
import { apiPost, apiGet, getAdminToken } from '@/api/client'
import { Search } from 'lucide-vue-next'

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

const queryForm = ref({ database: '', measurement: '', start_unix_nanos: '', end_unix_nanos: '', fields: '', limit: '100' })
const queryMode = ref<'rows' | 'columns' | 'explain' | 'stream'>('rows')
const rows = ref<QueryResultRow[]>([])
const queryStats = ref<QueryStatsData | null>(null)
const rawOutput = ref('')
const actionError = ref('')
const loading = ref(false)

async function executeQuery() {
  actionError.value = ''
  loading.value = true
  rows.value = []
  queryStats.value = null
  rawOutput.value = ''
  const query: Record<string, unknown> = {}
  if (queryForm.value.database) query.database = queryForm.value.database
  if (queryForm.value.measurement) query.measurement = queryForm.value.measurement
  if (queryForm.value.start_unix_nanos) query.start_unix_nanos = parseInt(queryForm.value.start_unix_nanos)
  if (queryForm.value.end_unix_nanos) query.end_unix_nanos = parseInt(queryForm.value.end_unix_nanos)
  if (queryForm.value.fields) query.fields = queryForm.value.fields.split(',').map((s) => s.trim())
  if (queryForm.value.limit) query.limit = parseInt(queryForm.value.limit)
  try {
    if (queryMode.value === 'rows') {
      const data = await apiPost<RowsResponse>('/api/v1/data/query/rows', { query })
      rows.value = data.rows ?? []
    } else if (queryMode.value === 'explain') {
      const data = await apiPost<{ result: { columns: QueryResultRow[]; explain: Record<string, unknown>; stats: QueryStatsData } }>('/api/v1/data/query/explain', { query })
      rows.value = (data.result?.columns as QueryResultRow[]) ?? []
      queryStats.value = data.result?.stats ?? null
    } else if (queryMode.value === 'columns') {
      const data = await apiPost<{ columns: unknown[] }>('/api/v1/data/query/columns', { query })
      rawOutput.value = JSON.stringify(data.columns, null, 2)
    } else if (queryMode.value === 'stream') {
      const token = getAdminToken()
      const resp = await fetch('/api/v1/data/query/stream', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', 'X-MTS-Admin-Token': token },
        body: JSON.stringify({ query }),
      })
      rawOutput.value = await resp.text()
    }
  } catch (e) {
    actionError.value = e instanceof Error ? e.message : '查询失败'
  } finally {
    loading.value = false
  }
  try {
    const statsData = await apiGet<StatsResponse>('/api/v1/data/query/stats')
    queryStats.value = statsData.stats
  } catch (_) { /* 统计获取失败不影响主流程 */ }
}

function formatTimestamp(ns: number): string { return new Date(ns / 1e6).toISOString() }
</script>

<template>
  <div class="space-y-6">
    <p v-if="actionError" class="rounded-lg border border-red-200 bg-red-50 p-3 text-sm text-red-700">{{ actionError }}</p>
    <div class="rounded-xl border border-slate-200 bg-white p-6">
      <h3 class="mb-4 text-sm font-semibold text-slate-800">查询条件</h3>
      <div class="mb-4 flex gap-3">
        <select v-model="queryMode" class="rounded-lg border border-slate-300 px-3 py-2 text-sm">
          <option value="rows">行式查询</option>
          <option value="columns">列式查询</option>
          <option value="explain">EXPLAIN</option>
          <option value="stream">流式查询</option>
        </select>
      </div>
      <div class="grid grid-cols-2 gap-3 lg:grid-cols-3">
        <div><label class="mb-1 block text-xs text-slate-500">数据库</label><input v-model="queryForm.database" placeholder="default" class="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm" /></div>
        <div><label class="mb-1 block text-xs text-slate-500">Measurement</label><input v-model="queryForm.measurement" class="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm" /></div>
        <div><label class="mb-1 block text-xs text-slate-500">开始时间 (纳秒)</label><input v-model="queryForm.start_unix_nanos" placeholder="0" type="number" class="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm" /></div>
        <div><label class="mb-1 block text-xs text-slate-500">结束时间 (纳秒)</label><input v-model="queryForm.end_unix_nanos" placeholder="now" type="number" class="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm" /></div>
        <div><label class="mb-1 block text-xs text-slate-500">字段 (逗号分隔)</label><input v-model="queryForm.fields" placeholder="value" class="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm" /></div>
        <div><label class="mb-1 block text-xs text-slate-500">Limit</label><input v-model="queryForm.limit" type="number" class="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm" /></div>
      </div>
      <button :disabled="loading" class="mt-4 inline-flex items-center gap-2 rounded-lg bg-slate-800 px-6 py-2 text-sm font-medium text-white hover:bg-slate-700 disabled:opacity-50" @click="executeQuery"><Search class="h-4 w-4" />{{ loading ? '查询中...' : '执行查询' }}</button>
    </div>
    <div v-if="queryStats" class="rounded-xl border border-slate-200 bg-white p-6">
      <h3 class="mb-3 text-sm font-semibold text-slate-800">查询统计</h3>
      <div class="grid grid-cols-3 gap-3 sm:grid-cols-5">
        <div><span class="text-xs text-slate-500">扫描 Shards</span><p class="text-sm font-medium">{{ queryStats.shards_scanned }}</p></div>
        <div><span class="text-xs text-slate-500">跳过 Shards</span><p class="text-sm font-medium">{{ queryStats.shards_skipped }}</p></div>
        <div><span class="text-xs text-slate-500">读取样本</span><p class="text-sm font-medium">{{ queryStats.samples_read }}</p></div>
        <div><span class="text-xs text-slate-500">返回样本</span><p class="text-sm font-medium">{{ queryStats.samples_returned }}</p></div>
        <div><span class="text-xs text-slate-500">耗时</span><p class="text-sm font-medium">{{ (queryStats.duration_nanos / 1e6).toFixed(2) }}ms</p></div>
      </div>
    </div>
    <div v-if="rows.length" class="rounded-xl border border-slate-200 bg-white p-6">
      <h3 class="mb-3 text-sm font-semibold text-slate-800">结果 ({{ rows.length }} 行)</h3>
      <div class="overflow-auto max-h-96">
        <table class="w-full text-sm">
          <thead><tr class="border-b border-slate-200 text-left"><th class="pb-2 pr-4 text-xs font-medium text-slate-500">时间</th><th class="pb-2 pr-4 text-xs font-medium text-slate-500">Measurement</th><th class="pb-2 pr-4 text-xs font-medium text-slate-500">Tags</th><th class="pb-2 text-xs font-medium text-slate-500">Fields</th></tr></thead>
          <tbody><tr v-for="(row, idx) in rows" :key="idx" class="border-b border-slate-100 last:border-b-0"><td class="py-2 pr-4 font-mono text-xs text-slate-600">{{ formatTimestamp(row.timestamp) }}</td><td class="py-2 pr-4 text-xs text-slate-600">{{ row.measurement }}</td><td class="py-2 pr-4 text-xs text-slate-500">{{ JSON.stringify(row.tags) }}</td><td class="py-2 text-xs text-slate-600">{{ JSON.stringify(row.fields) }}</td></tr></tbody>
        </table>
      </div>
    </div>
    <div v-if="rawOutput" class="rounded-xl border border-slate-200 bg-white p-6">
      <h3 class="mb-3 text-sm font-semibold text-slate-800">原始输出</h3>
      <pre class="overflow-auto rounded-lg bg-slate-900 p-4 text-xs text-green-400 max-h-96">{{ rawOutput }}</pre>
    </div>
  </div>
</template>
