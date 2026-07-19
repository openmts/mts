<script setup lang="ts">
import { onMounted, onBeforeUnmount, ref, watch } from 'vue'
import { apiPost } from '@/api/client'
import { useQueryWorkbench } from '@/composables/useQueryWorkbench'
import { useNotify } from '@/composables/useNotify'
import { formatEpoch, nowUnixMsString } from '@/utils/time'
import {
  Search, Database, Hash, Clock, Filter, Layers, Zap, BarChart3, Timer, Square, Copy, Check, Trash2,
} from 'lucide-vue-next'

const {
  databases, measurements, retentionPolicies, measurementsLoading, queryForm, queryMode,
  rows, queryStats, rawOutput, streamMeta, actionError, loading,
  loadDatabases, loadDbChildren, executeQuery, cancelQuery, resultTextForCopy, buildQuery,
} = useQueryWorkbench()
const { success, error: notifyError } = useNotify()

const copyState = ref<'idle' | 'ok' | 'fail'>('idle')
let copyTimer: ReturnType<typeof setTimeout> | null = null
const deleteOpen = ref(false)
const deleteConfirmText = ref('')
const deleteLoading = ref(false)
const deleteResult = ref('')

const modeOptions = [
  { value: 'rows' as const, label: '行式查询', desc: '按行返回数据' },
  { value: 'columns' as const, label: '列式查询', desc: '按列返回数据' },
  { value: 'explain' as const, label: 'EXPLAIN', desc: '执行计划 JSON' },
  { value: 'stream-row' as const, label: '流式行', desc: 'NDJSON 真流式' },
  { value: 'stream-column' as const, label: '流式列', desc: 'NDJSON 列流' },
]

onMounted(async () => {
  try {
    await loadDatabases()
  } catch (e) {
    actionError.value = e instanceof Error ? e.message : '加载数据库失败'
  }
})

onBeforeUnmount(() => {
  cancelQuery()
  if (copyTimer) clearTimeout(copyTimer)
})

watch(() => queryForm.value.database, async (db) => {
  try {
    await loadDbChildren(db)
  } catch (e) {
    actionError.value = e instanceof Error ? e.message : '加载 measurement 失败'
  }
})

function formatTimestamp(v: number): string {
  // 查询默认 precision=ms；若数值极大按 ns 展示
  if (Math.abs(v) >= 1e15) return formatEpoch(v, 'ns')
  return formatEpoch(v, 'ms')
}

function fillNowMs(which: 'start' | 'end') {
  const s = nowUnixMsString()
  if (which === 'start') queryForm.value.start_time = s
  else queryForm.value.end_time = s
}

async function copyResults() {
  const text = resultTextForCopy()
  if (!text) {
    actionError.value = '暂无可复制内容'
    return
  }
  try {
    if (navigator.clipboard?.writeText) await navigator.clipboard.writeText(text)
    else {
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
    success('结果已复制')
  } catch (_) {
    copyState.value = 'fail'
  }
  if (copyTimer) clearTimeout(copyTimer)
  copyTimer = setTimeout(() => { copyState.value = 'idle' }, 2000)
}

function openDelete() {
  deleteResult.value = ''
  deleteConfirmText.value = ''
  deleteOpen.value = true
}

async function executeDelete() {
  actionError.value = ''
  deleteResult.value = ''
  if (!queryForm.value.database || !queryForm.value.measurement) {
    actionError.value = '删除需要数据库和 measurement'
    return
  }
  if (!queryForm.value.start_time || !queryForm.value.end_time) {
    actionError.value = '删除需要 start_time 与 end_time（毫秒整数）'
    return
  }
  const expected = `删除 ${queryForm.value.database}/${queryForm.value.measurement}`
  if (deleteConfirmText.value.trim() !== expected) {
    actionError.value = `请输入确认文案：${expected}`
    return
  }
  deleteLoading.value = true
  try {
    const query = buildQuery()
    const request = {
      database: query.database,
      retention_policy: query.retention_policy || 'autogen',
      measurement: query.measurement,
      start_time: query.start_time,
      end_time: query.end_time,
      precision: 'ms',
    }
    await apiPost('/api/v1/data/delete', { request })
    deleteResult.value = '删除请求已提交（tombstone）'
    deleteOpen.value = false
    success('删除请求已提交')
  } catch (e) {
    actionError.value = e instanceof Error ? e.message : '删除失败'
    notifyError(actionError.value)
  } finally {
    deleteLoading.value = false
  }
}
</script>

<template>
  <div class="space-y-6">
    <p v-if="actionError" class="rounded-xl border border-red-200 bg-red-50 p-4 text-sm text-red-700">{{ actionError }}</p>
    <p v-if="deleteResult" class="rounded-xl border border-green-200 bg-green-50 p-4 text-sm text-green-700">{{ deleteResult }}</p>

    <div class="overflow-hidden rounded-2xl border border-slate-200 bg-white shadow-sm">
      <div class="border-b border-slate-100 bg-slate-50/50 px-6 py-4">
        <h3 class="text-sm font-semibold text-slate-800">查询条件</h3>
        <p class="mt-1 text-xs text-slate-500">时间字段使用毫秒 Unix 时间（precision=ms），避免纳秒 Number 精度丢失。</p>
      </div>
      <div class="p-6">
        <div class="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
          <div class="sm:col-span-2">
            <label class="mb-1.5 flex items-center gap-1.5 text-xs font-medium text-slate-500"><Database class="h-3 w-3" />数据库 & Measurement</label>
            <div class="flex gap-2">
              <select v-model="queryForm.database" class="flex-1 rounded-lg border border-slate-200 px-3 py-2 text-sm">
                <option v-if="!databases.length" value="" disabled>无数据</option>
                <option v-for="db in databases" :key="db" :value="db">{{ db }}</option>
              </select>
              <select v-model="queryForm.measurement" :disabled="!queryForm.database || measurementsLoading" class="flex-1 rounded-lg border border-slate-200 px-3 py-2 text-sm disabled:bg-slate-50">
                <option v-if="measurementsLoading" value="" disabled>加载中...</option>
                <option v-else-if="!measurements.length" value="" disabled>无数据</option>
                <option v-for="m in measurements" :key="m" :value="m">{{ m }}</option>
              </select>
            </div>
          </div>
          <div>
            <label class="mb-1.5 flex items-center gap-1.5 text-xs font-medium text-slate-500"><Clock class="h-3 w-3" />保留策略</label>
            <select v-model="queryForm.retention_policy" class="w-full rounded-lg border border-slate-200 px-3 py-2 text-sm">
              <option value="autogen">autogen</option>
              <option v-for="rp in retentionPolicies" :key="rp" :value="rp">{{ rp }}</option>
            </select>
          </div>
          <div>
            <label class="mb-1.5 flex items-center gap-1.5 text-xs font-medium text-slate-500"><Hash class="h-3 w-3" />Limit</label>
            <input v-model="queryForm.limit" type="text" inputmode="numeric" class="w-full rounded-lg border border-slate-200 px-3 py-2 text-sm" />
          </div>
          <div>
            <label class="mb-1.5 flex items-center gap-1.5 text-xs font-medium text-slate-500"><Timer class="h-3 w-3" />Start (ms)</label>
            <div class="flex gap-1">
              <input v-model="queryForm.start_time" type="text" inputmode="numeric" placeholder="可选" class="w-full rounded-lg border border-slate-200 px-3 py-2 font-mono text-sm" />
              <button type="button" class="rounded-lg border border-slate-200 px-2 text-xs" @click="fillNowMs('start')">now</button>
            </div>
          </div>
          <div>
            <label class="mb-1.5 flex items-center gap-1.5 text-xs font-medium text-slate-500"><Timer class="h-3 w-3" />End (ms)</label>
            <div class="flex gap-1">
              <input v-model="queryForm.end_time" type="text" inputmode="numeric" placeholder="可选" class="w-full rounded-lg border border-slate-200 px-3 py-2 font-mono text-sm" />
              <button type="button" class="rounded-lg border border-slate-200 px-2 text-xs" @click="fillNowMs('end')">now</button>
            </div>
          </div>
          <div class="sm:col-span-2">
            <label class="mb-1.5 flex items-center gap-1.5 text-xs font-medium text-slate-500"><Filter class="h-3 w-3" />Fields</label>
            <input v-model="queryForm.fields" type="text" placeholder="value,cpu" class="w-full rounded-lg border border-slate-200 px-3 py-2 text-sm" />
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
          <button :disabled="loading" class="flex items-center gap-2 rounded-lg bg-slate-800 px-4 py-2 text-sm font-medium text-white disabled:opacity-50" @click="executeQuery">
            <Search class="h-4 w-4" />{{ loading ? '查询中...' : '执行查询' }}
          </button>
          <button :disabled="!loading" class="flex items-center gap-2 rounded-lg border border-amber-200 bg-white px-4 py-2 text-sm font-medium text-amber-700 disabled:opacity-50" @click="cancelQuery">
            <Square class="h-4 w-4" />取消查询
          </button>
          <button :disabled="!rows.length && !rawOutput" class="flex items-center gap-2 rounded-lg border border-slate-200 bg-white px-4 py-2 text-sm font-medium text-slate-700 disabled:opacity-50" @click="copyResults">
            <Check v-if="copyState === 'ok'" class="h-4 w-4 text-green-600" />
            <Copy v-else class="h-4 w-4" />
            {{ copyState === 'ok' ? '已复制' : copyState === 'fail' ? '复制失败' : '复制结果' }}
          </button>
          <button class="flex items-center gap-2 rounded-lg border border-red-200 bg-white px-4 py-2 text-sm font-medium text-red-600" @click="openDelete">
            <Trash2 class="h-4 w-4" />范围删除...
          </button>
        </div>
      </div>
    </div>

    <!-- 删除确认 -->
    <div v-if="deleteOpen" class="rounded-2xl border border-red-200 bg-red-50/60 p-6 shadow-sm">
      <h3 class="text-sm font-semibold text-red-800">危险操作：按时间范围删除</h3>
      <p class="mt-2 text-xs text-red-700">
        将删除
        <strong>{{ queryForm.database || '—' }}</strong> /
        <strong>{{ queryForm.measurement || '—' }}</strong>
        在 [{{ queryForm.start_time || '—' }}, {{ queryForm.end_time || '—' }}] (ms) 范围数据（tombstone）。
      </p>
      <p class="mt-2 text-xs text-slate-600">请输入确认文案：
        <code class="rounded bg-white px-1">删除 {{ queryForm.database }}/{{ queryForm.measurement }}</code>
      </p>
      <input v-model="deleteConfirmText" class="mt-2 w-full rounded-lg border border-red-200 bg-white px-3 py-2 text-sm" placeholder="确认文案" />
      <div class="mt-3 flex gap-2">
        <button :disabled="deleteLoading" class="rounded-lg bg-red-700 px-4 py-2 text-sm text-white disabled:opacity-50" @click="executeDelete">{{ deleteLoading ? '提交中...' : '确认删除' }}</button>
        <button class="rounded-lg border border-slate-200 bg-white px-4 py-2 text-sm" @click="deleteOpen = false">取消</button>
      </div>
    </div>

    <div v-if="queryStats" class="overflow-hidden rounded-2xl border border-slate-200 bg-white shadow-sm">
      <div class="border-b border-slate-100 bg-slate-50/50 px-6 py-3">
        <h3 class="flex items-center gap-2 text-sm font-semibold text-slate-800"><BarChart3 class="h-4 w-4 text-slate-400" />查询统计</h3>
      </div>
      <div class="grid grid-cols-2 gap-px bg-slate-100 sm:grid-cols-5">
        <div class="bg-white p-4"><p class="text-[11px] uppercase text-slate-400">扫描 Shards</p><p class="mt-1 text-lg font-semibold">{{ queryStats.shards_scanned }}</p></div>
        <div class="bg-white p-4"><p class="text-[11px] uppercase text-slate-400">跳过 Shards</p><p class="mt-1 text-lg font-semibold">{{ queryStats.shards_skipped }}</p></div>
        <div class="bg-white p-4"><p class="text-[11px] uppercase text-slate-400">读取样本</p><p class="mt-1 text-lg font-semibold text-blue-600">{{ queryStats.samples_read }}</p></div>
        <div class="bg-white p-4"><p class="text-[11px] uppercase text-slate-400">返回样本</p><p class="mt-1 text-lg font-semibold text-green-600">{{ queryStats.samples_returned }}</p></div>
        <div class="bg-white p-4"><p class="text-[11px] uppercase text-slate-400">耗时</p><p class="mt-1 text-lg font-semibold text-amber-600">{{ (queryStats.duration_nanos / 1e6).toFixed(1) }}<span class="text-sm text-slate-400">ms</span></p></div>
      </div>
    </div>

    <div v-if="rows.length" class="overflow-hidden rounded-2xl border border-slate-200 bg-white shadow-sm">
      <div class="flex items-center justify-between border-b border-slate-100 bg-slate-50/50 px-6 py-3">
        <h3 class="flex items-center gap-2 text-sm font-semibold"><Layers class="h-4 w-4 text-slate-400" />查询结果</h3>
        <span class="rounded-full bg-slate-100 px-2.5 py-0.5 text-xs text-slate-500">{{ rows.length }} 行</span>
      </div>
      <div class="overflow-x-auto">
        <table class="w-full text-sm">
          <thead>
            <tr class="border-b border-slate-100 bg-slate-50/30 text-left">
              <th class="px-6 py-2.5 text-[11px] font-semibold uppercase text-slate-500">时间</th>
              <th class="px-6 py-2.5 text-[11px] font-semibold uppercase text-slate-500">Measurement</th>
              <th class="px-6 py-2.5 text-[11px] font-semibold uppercase text-slate-500">Tags</th>
              <th class="px-6 py-2.5 text-[11px] font-semibold uppercase text-slate-500">Fields</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-slate-50">
            <tr v-for="(row, idx) in rows" :key="idx" class="hover:bg-slate-50/50">
              <td class="px-6 py-3 font-mono text-xs text-slate-600">{{ formatTimestamp(row.timestamp) }}</td>
              <td class="px-6 py-3"><span class="rounded-md bg-blue-50 px-2 py-0.5 text-xs font-medium text-blue-700">{{ row.measurement }}</span></td>
              <td class="px-6 py-3 font-mono text-xs text-slate-500">{{ row.tags && Object.keys(row.tags).length ? JSON.stringify(row.tags) : '—' }}</td>
              <td class="px-6 py-3 font-mono text-xs text-slate-600">{{ JSON.stringify(row.fields) }}</td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <div v-if="rawOutput" class="overflow-hidden rounded-2xl border border-slate-200 bg-white shadow-sm">
      <div class="flex flex-wrap items-center justify-between gap-2 border-b border-slate-100 bg-slate-50/50 px-6 py-3">
        <h3 class="flex items-center gap-2 text-sm font-semibold"><Zap class="h-4 w-4 text-slate-400" />原始输出 / EXPLAIN / 流式预览</h3>
        <div class="flex flex-wrap gap-2 text-xs text-slate-500">
          <span v-if="streamMeta.lines" class="rounded-full bg-slate-100 px-2.5 py-0.5">{{ streamMeta.lines }} 行</span>
          <span v-if="streamMeta.records" class="rounded-full bg-blue-50 px-2.5 py-0.5 text-blue-700">{{ streamMeta.records }} 记录</span>
          <span v-if="streamMeta.errors" class="rounded-full bg-red-50 px-2.5 py-0.5 text-red-600">{{ streamMeta.errors }} 错误</span>
          <button class="rounded-lg border border-slate-200 bg-white px-2.5 py-1" @click="copyResults">复制</button>
        </div>
      </div>
      <pre class="max-h-96 overflow-auto bg-slate-950 p-6 font-mono text-xs leading-relaxed text-emerald-400">{{ rawOutput }}</pre>
    </div>
  </div>
</template>
