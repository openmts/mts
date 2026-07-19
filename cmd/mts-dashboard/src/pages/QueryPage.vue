<script setup lang="ts">
import { onMounted, onBeforeUnmount, ref, watch, computed } from 'vue'
import { apiPost } from '@/api/client'
import { useQueryWorkbench } from '@/composables/useQueryWorkbench'
import { useQueryHistory } from '@/composables/useQueryHistory'
import { useNotify } from '@/composables/useNotify'
import { useI18n } from '@/composables/useI18n'
import { formatEpoch, nowUnixMsString } from '@/utils/time'
import QueryChart from '@/components/QueryChart.vue'
import ConfirmDialog from '@/components/ConfirmDialog.vue'
import {
  Search, Database, Hash, Clock, Filter, Layers, Zap, BarChart3, Timer, Square, Copy, Check, Trash2, History,
} from 'lucide-vue-next'

const {
  databases, measurements, retentionPolicies, measurementsLoading, queryForm, queryMode,
  rows, queryStats, rawOutput, streamMeta, actionError, loading,
  loadDatabases, loadDbChildren, executeQuery, cancelQuery, resultTextForCopy, buildQuery,
} = useQueryWorkbench()
const history = useQueryHistory()
const { success, error: notifyError } = useNotify()
const { t } = useI18n()

const copyState = ref<'idle' | 'ok' | 'fail'>('idle')
let copyTimer: ReturnType<typeof setTimeout> | null = null
const deleteOpen = ref(false)
const deleteConfirmText = ref('')
const deleteLoading = ref(false)
const deleteResult = ref('')
const showHistory = ref(false)
const showChart = ref(true)

const modeOptions = [
  { value: 'rows' as const, label: '行式查询', desc: '按行返回数据' },
  { value: 'columns' as const, label: '列式查询', desc: '按列返回数据' },
  { value: 'explain' as const, label: 'EXPLAIN', desc: '执行计划 JSON' },
  { value: 'stream-row' as const, label: '流式行', desc: 'NDJSON 真流式' },
  { value: 'stream-column' as const, label: '流式列', desc: 'NDJSON 列流' },
]

onMounted(async () => {
  try { await loadDatabases() }
  catch (e) { actionError.value = e instanceof Error ? e.message : '加载数据库失败' }
})
onBeforeUnmount(() => {
  cancelQuery()
  if (copyTimer) clearTimeout(copyTimer)
})
watch(() => queryForm.value.database, async (db) => {
  try { await loadDbChildren(db) }
  catch (e) { actionError.value = e instanceof Error ? e.message : '加载 measurement 失败' }
})

function formatTimestamp(v: number): string {
  if (Math.abs(v) >= 1e15) return formatEpoch(v, 'ns')
  return formatEpoch(v, 'ms')
}
function fillNowMs(which: 'start' | 'end') {
  const s = nowUnixMsString()
  if (which === 'start') queryForm.value.start_time = s
  else queryForm.value.end_time = s
}

async function runQuery() {
  await executeQuery()
  if (!actionError.value) {
    history.push({
      mode: queryMode.value,
      form: { ...queryForm.value },
    })
  } else {
    notifyError(actionError.value)
  }
}

function applyHistory(id: string) {
  const item = history.items.value.find((x) => x.id === id)
  if (!item) return
  queryMode.value = item.mode
  queryForm.value = { ...item.form }
  showHistory.value = false
}

async function copyResults() {
  const text = resultTextForCopy()
  if (!text) { actionError.value = '暂无可复制内容'; return }
  try {
    if (navigator.clipboard?.writeText) await navigator.clipboard.writeText(text)
    else {
      const area = document.createElement('textarea')
      area.value = text
      area.style.position = 'fixed'
      area.style.left = '-9999px'
      document.body.appendChild(area)
      area.select()
      document.execCommand('copy')
      document.body.removeChild(area)
    }
    copyState.value = 'ok'
    success(streamMeta.value.previewOnly ? t.value('copyPreview') : t.value('copy'))
  } catch {
    copyState.value = 'fail'
  }
  if (copyTimer) clearTimeout(copyTimer)
  copyTimer = setTimeout(() => { copyState.value = 'idle' }, 1500)
}

async function doRangeDelete() {
  if (deleteConfirmText.value !== 'DELETE') return
  deleteLoading.value = true
  deleteResult.value = ''
  try {
    const query = buildQuery()
    const body = {
      request: {
        database: query.database,
        retention_policy: query.retention_policy,
        measurement: query.measurement,
        start_time: query.start_time,
        end_time: query.end_time,
      },
    }
    await apiPost('/api/v1/data/delete', body)
    deleteResult.value = '范围删除已提交'
    success(deleteResult.value)
    deleteOpen.value = false
    deleteConfirmText.value = ''
  } catch (e) {
    deleteResult.value = e instanceof Error ? e.message : '删除失败'
    notifyError(deleteResult.value)
  } finally {
    deleteLoading.value = false
  }
}

const historyPreview = computed(() => history.items.value.slice(0, 12))
</script>

<template>
  <div class="space-y-4">
    <div class="flex flex-wrap items-center justify-between gap-2">
      <div class="flex flex-wrap gap-2">
        <button
          v-for="m in modeOptions"
          :key="m.value"
          class="rounded-lg border px-3 py-1.5 text-xs"
          :class="queryMode === m.value ? 'border-slate-800 bg-slate-800 text-white dark:border-slate-100 dark:bg-slate-100 dark:text-slate-900' : 'border-slate-200 bg-white text-slate-600 dark:border-slate-700 dark:bg-slate-900 dark:text-slate-300'"
          @click="queryMode = m.value"
        >{{ m.label }}</button>
      </div>
      <div class="flex gap-2">
        <button class="inline-flex items-center gap-1 rounded-lg border border-slate-200 bg-white px-3 py-1.5 text-xs dark:border-slate-700 dark:bg-slate-900" @click="showHistory = !showHistory">
          <History class="h-3.5 w-3.5" /> {{ t('queryHistory') }}
        </button>
        <button class="inline-flex items-center gap-1 rounded-lg border border-slate-200 bg-white px-3 py-1.5 text-xs dark:border-slate-700 dark:bg-slate-900" @click="showChart = !showChart">
          <BarChart3 class="h-3.5 w-3.5" /> {{ t('chart') }}
        </button>
      </div>
    </div>

    <div v-if="showHistory" class="rounded-xl border border-slate-200 bg-white p-3 dark:border-slate-700 dark:bg-slate-900">
      <div class="mb-2 flex items-center justify-between">
        <h3 class="text-sm font-semibold">{{ t('queryHistory') }}</h3>
        <button class="text-xs text-slate-500 hover:text-red-600" @click="history.clear()">{{ t('clearHistory') }}</button>
      </div>
      <div v-if="!historyPreview.length" class="text-xs text-slate-400">暂无历史</div>
      <div v-else class="space-y-1">
        <button
          v-for="h in historyPreview"
          :key="h.id"
          class="flex w-full items-center justify-between rounded-lg px-2 py-1.5 text-left text-xs hover:bg-slate-50 dark:hover:bg-slate-800"
          @click="applyHistory(h.id)"
        >
          <span class="truncate">{{ h.form.database }}/{{ h.form.measurement || '*' }} · {{ h.mode }}</span>
          <span class="text-slate-400">{{ new Date(h.at).toLocaleString() }}</span>
        </button>
      </div>
    </div>

    <div class="grid gap-3 rounded-xl border border-slate-200 bg-white p-4 dark:border-slate-700 dark:bg-slate-900 md:grid-cols-3">
      <label class="text-xs text-slate-500">Database
        <select v-model="queryForm.database" class="mt-1 w-full rounded border border-slate-300 px-2 py-1.5 text-sm dark:border-slate-600 dark:bg-slate-800">
          <option v-for="db in databases" :key="db" :value="db">{{ db }}</option>
        </select>
      </label>
      <label class="text-xs text-slate-500">Measurement
        <select v-model="queryForm.measurement" class="mt-1 w-full rounded border border-slate-300 px-2 py-1.5 text-sm dark:border-slate-600 dark:bg-slate-800" :disabled="measurementsLoading">
          <option value="">(all)</option>
          <option v-for="m in measurements" :key="m" :value="m">{{ m }}</option>
        </select>
      </label>
      <label class="text-xs text-slate-500">RP
        <select v-model="queryForm.retention_policy" class="mt-1 w-full rounded border border-slate-300 px-2 py-1.5 text-sm dark:border-slate-600 dark:bg-slate-800">
          <option v-for="rp in (retentionPolicies.length ? retentionPolicies : ['autogen'])" :key="rp" :value="rp">{{ rp }}</option>
        </select>
      </label>
      <label class="text-xs text-slate-500">Start (ms)
        <div class="mt-1 flex gap-1">
          <input v-model="queryForm.start_time" class="w-full rounded border border-slate-300 px-2 py-1.5 text-sm dark:border-slate-600 dark:bg-slate-800" />
          <button class="rounded border px-2 text-xs" @click="fillNowMs('start')">now</button>
        </div>
      </label>
      <label class="text-xs text-slate-500">End (ms)
        <div class="mt-1 flex gap-1">
          <input v-model="queryForm.end_time" class="w-full rounded border border-slate-300 px-2 py-1.5 text-sm dark:border-slate-600 dark:bg-slate-800" />
          <button class="rounded border px-2 text-xs" @click="fillNowMs('end')">now</button>
        </div>
      </label>
      <label class="text-xs text-slate-500">Fields / Limit
        <div class="mt-1 flex gap-1">
          <input v-model="queryForm.fields" placeholder="f1,f2" class="w-full rounded border border-slate-300 px-2 py-1.5 text-sm dark:border-slate-600 dark:bg-slate-800" />
          <input v-model="queryForm.limit" class="w-20 rounded border border-slate-300 px-2 py-1.5 text-sm dark:border-slate-600 dark:bg-slate-800" />
        </div>
      </label>
    </div>

    <div class="flex flex-wrap gap-2">
      <button class="inline-flex items-center gap-1 rounded-lg bg-slate-800 px-4 py-2 text-sm font-medium text-white disabled:opacity-50 dark:bg-slate-100 dark:text-slate-900" :disabled="loading" @click="runQuery">
        <Search class="h-4 w-4" /> {{ loading ? t('loading') : t('query') }}
      </button>
      <button class="inline-flex items-center gap-1 rounded-lg border border-slate-200 px-3 py-2 text-sm dark:border-slate-700" :disabled="!loading" @click="cancelQuery">
        <Square class="h-4 w-4" /> 取消
      </button>
      <button class="inline-flex items-center gap-1 rounded-lg border border-red-200 px-3 py-2 text-sm text-red-700 dark:border-red-900" @click="deleteOpen = true">
        <Trash2 class="h-4 w-4" /> 范围删除
      </button>
      <button class="inline-flex items-center gap-1 rounded-lg border border-slate-200 px-3 py-2 text-sm dark:border-slate-700" @click="copyResults">
        <component :is="copyState === 'ok' ? Check : Copy" class="h-4 w-4" />
        {{ streamMeta.previewOnly ? t('copyPreview') : t('copy') }}
      </button>
    </div>

    <p v-if="actionError" class="rounded-xl border border-red-200 bg-red-50 p-3 text-sm text-red-700 dark:border-red-900 dark:bg-red-950 dark:text-red-200">{{ actionError }}</p>
    <p v-if="deleteResult" class="rounded-xl border border-slate-200 bg-slate-50 p-3 text-sm dark:border-slate-700 dark:bg-slate-900">{{ deleteResult }}</p>

    <div v-if="queryStats" class="grid grid-cols-2 gap-2 sm:grid-cols-5">
      <div class="rounded-xl border border-slate-200 bg-white p-3 dark:border-slate-700 dark:bg-slate-900"><p class="text-[11px] text-slate-400">扫描 Shards</p><p class="text-lg font-semibold">{{ queryStats.shards_scanned }}</p></div>
      <div class="rounded-xl border border-slate-200 bg-white p-3 dark:border-slate-700 dark:bg-slate-900"><p class="text-[11px] text-slate-400">跳过 Shards</p><p class="text-lg font-semibold">{{ queryStats.shards_skipped }}</p></div>
      <div class="rounded-xl border border-slate-200 bg-white p-3 dark:border-slate-700 dark:bg-slate-900"><p class="text-[11px] text-slate-400">读取样本</p><p class="text-lg font-semibold text-blue-600">{{ queryStats.samples_read }}</p></div>
      <div class="rounded-xl border border-slate-200 bg-white p-3 dark:border-slate-700 dark:bg-slate-900"><p class="text-[11px] text-slate-400">返回样本</p><p class="text-lg font-semibold text-green-600">{{ queryStats.samples_returned }}</p></div>
      <div class="rounded-xl border border-slate-200 bg-white p-3 dark:border-slate-700 dark:bg-slate-900"><p class="text-[11px] text-slate-400">耗时</p><p class="text-lg font-semibold text-amber-600">{{ (queryStats.duration_nanos / 1e6).toFixed(1) }}ms</p></div>
    </div>

    <QueryChart v-if="showChart && rows.length" :rows="rows" />

    <div v-if="rows.length" class="overflow-hidden rounded-2xl border border-slate-200 bg-white shadow-sm dark:border-slate-700 dark:bg-slate-900">
      <div class="flex items-center justify-between border-b border-slate-100 px-4 py-2 dark:border-slate-800">
        <h3 class="text-sm font-semibold">结果</h3>
        <span class="text-xs text-slate-500">{{ rows.length }} 行</span>
      </div>
      <div class="overflow-x-auto">
        <table class="w-full text-sm">
          <thead>
            <tr class="border-b border-slate-100 bg-slate-50/30 text-left dark:border-slate-800 dark:bg-slate-950/40">
              <th class="px-4 py-2 text-[11px] uppercase text-slate-500">时间</th>
              <th class="px-4 py-2 text-[11px] uppercase text-slate-500">Measurement</th>
              <th class="px-4 py-2 text-[11px] uppercase text-slate-500">Tags</th>
              <th class="px-4 py-2 text-[11px] uppercase text-slate-500">Fields</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="(row, idx) in rows" :key="idx" class="border-b border-slate-50 dark:border-slate-800">
              <td class="px-4 py-2 font-mono text-xs">{{ formatTimestamp(row.timestamp) }}</td>
              <td class="px-4 py-2 text-xs">{{ row.measurement }}</td>
              <td class="px-4 py-2 font-mono text-xs text-slate-500">{{ row.tags && Object.keys(row.tags).length ? JSON.stringify(row.tags) : '—' }}</td>
              <td class="px-4 py-2 font-mono text-xs">{{ JSON.stringify(row.fields) }}</td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <div v-if="rawOutput" class="overflow-hidden rounded-2xl border border-slate-200 bg-white shadow-sm dark:border-slate-700 dark:bg-slate-900">
      <div class="flex flex-wrap items-center justify-between gap-2 border-b border-slate-100 px-4 py-2 dark:border-slate-800">
        <h3 class="text-sm font-semibold">原始输出 / EXPLAIN / 流式结果</h3>
        <div class="flex flex-wrap gap-2 text-xs text-slate-500">
          <span v-if="streamMeta.lines" class="rounded-full bg-slate-100 px-2 py-0.5 dark:bg-slate-800">{{ streamMeta.lines }} 行</span>
          <span v-if="streamMeta.records" class="rounded-full bg-blue-50 px-2 py-0.5 text-blue-700">{{ streamMeta.records }} 记录</span>
          <span v-if="streamMeta.errors" class="rounded-full bg-red-50 px-2 py-0.5 text-red-600">{{ streamMeta.errors }} 错误</span>
          <span v-if="streamMeta.previewOnly" class="rounded-full bg-amber-50 px-2 py-0.5 text-amber-700">仅预览前 {{ streamMeta.previewLimit }} 行</span>
        </div>
      </div>
      <pre class="max-h-96 overflow-auto bg-slate-950 p-4 font-mono text-xs text-emerald-400">{{ rawOutput }}</pre>
    </div>

    <ConfirmDialog
      v-model:open="deleteOpen"
      title="范围删除"
      message="将按当前查询条件删除数据，不可恢复。请输入 DELETE 确认。"
      require-text="DELETE"
      confirm-label="删除"
      danger
      :loading="deleteLoading"
      @confirm="doRangeDelete"
    />
  </div>
</template>
