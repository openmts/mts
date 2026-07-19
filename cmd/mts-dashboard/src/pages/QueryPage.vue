<script setup lang="ts">
import { onMounted, onBeforeUnmount, ref, watch, computed } from 'vue'
import { apiPost } from '@/api/client'
import { useQueryWorkbench } from '@/composables/useQueryWorkbench'
import { useQueryHistory } from '@/composables/useQueryHistory'
import { useNotify } from '@/composables/useNotify'
import { useI18n } from '@/composables/useI18n'
import { formatEpoch, nowUnixMsString } from '@/utils/time'
import { formatFieldsMap } from '@/utils/fieldValue'
import { rowsToCSV, downloadText } from '@/utils/csv'
import { loadQueryPrefs, saveQueryPrefs } from '@/utils/queryPrefs'
import { isEditableTarget, matchQueryShortcut } from '@/utils/keyboard'
import QueryChart from '@/components/QueryChart.vue'
import VirtualTable from '@/components/VirtualTable.vue'
import { checkDatabasePermission } from '@/api/authz'
import { useAuth } from '@/composables/useAuth'
import ConfirmDialog from '@/components/ConfirmDialog.vue'
import { Search, Square, Copy, Check, Trash2, History, BarChart3, Download, Star, Pencil, X, Upload } from 'lucide-vue-next'

const {
  databases, measurements, retentionPolicies, measurementsLoading, metaSource, metaHint,
  queryForm, queryMode, rows, columnSeries, queryStats, rawOutput, streamMeta, actionError, loading,
  loadDatabases, loadDbChildren, executeQuery, cancelQuery, resultTextForCopy, buildQuery,
} = useQueryWorkbench()
const history = useQueryHistory()
const { success, error: notifyError } = useNotify()
const { t } = useI18n()
const { currentUser, isAdmin } = useAuth()
const authzHint = ref('')
const authzChecking = ref(false)

const copyState = ref<'idle' | 'ok' | 'fail'>('idle')
let copyTimer: ReturnType<typeof setTimeout> | null = null
const deleteOpen = ref(false)
const deleteConfirmText = ref('')
const deleteLoading = ref(false)
const deleteResult = ref('')
const PREFS_KEY = 'mts_query_prefs'
const initialPrefs = loadQueryPrefs(typeof localStorage !== 'undefined' ? localStorage : null, PREFS_KEY)
const showHistory = ref(initialPrefs.showHistory)
const showChart = ref(initialPrefs.showChart)
const showRawFields = ref(initialPrefs.showRawFields)
const renameDraft = ref('')
const renamingId = ref<string | null>(null)
const clearHistoryOpen = ref(false)
const historyFileInput = ref<HTMLInputElement | null>(null)

const modeOptions = [
  { value: 'rows' as const, label: '行式' },
  { value: 'columns' as const, label: '列式' },
  { value: 'explain' as const, label: 'EXPLAIN' },
  { value: 'stream-row' as const, label: '流式行' },
  { value: 'stream-column' as const, label: '流式列' },
]

function persistPrefs() {
  saveQueryPrefs(typeof localStorage !== 'undefined' ? localStorage : null, PREFS_KEY, {
    showChart: showChart.value,
    showRawFields: showRawFields.value,
    showHistory: showHistory.value,
  })
}

function onQueryKeydown(e: KeyboardEvent) {
  // 对话框打开时交给 ConfirmDialog；输入框内仅允许 Ctrl/Cmd+Enter 与 Escape
  if (deleteOpen.value || clearHistoryOpen.value) return
  const action = matchQueryShortcut(e)
  if (!action) return
  if (isEditableTarget(e.target) && action !== 'run' && action !== 'cancel') return
  if (action === 'run') {
    e.preventDefault()
    if (!loading.value) void runQuery()
    return
  }
  if (action === 'cancel') {
    e.preventDefault()
    if (loading.value) cancelQuery()
    else if (showHistory.value) showHistory.value = false
    return
  }
  if (action === 'copy') {
    e.preventDefault()
    void copyResults()
    return
  }
  if (action === 'toggle-history') {
    e.preventDefault()
    showHistory.value = !showHistory.value
  }
}

onMounted(async () => {
  window.addEventListener('keydown', onQueryKeydown)
  try { await loadDatabases() }
  catch (e) { actionError.value = e instanceof Error ? e.message : '加载数据库失败' }
})
onBeforeUnmount(() => {
  window.removeEventListener('keydown', onQueryKeydown)
  cancelQuery()
  if (copyTimer) clearTimeout(copyTimer)
})
watch([showChart, showRawFields, showHistory], () => { persistPrefs() })
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

async function checkAuthz(perm: 'read' | 'write' = 'read') {
  authzHint.value = ''
  if (!queryForm.value.database.trim()) {
    authzHint.value = '请先填写 database'
    return
  }
  authzChecking.value = true
  try {
    const allowed = await checkDatabasePermission({
      database: queryForm.value.database.trim(),
      permission: perm,
      user_name: isAdmin.value ? undefined : currentUser.value || undefined,
    })
    authzHint.value = allowed
      ? `权限预检通过：${currentUser.value || 'current'} 对 ${queryForm.value.database} 具备 ${perm}`
      : `权限预检拒绝：${currentUser.value || 'current'} 对 ${queryForm.value.database} 无 ${perm}`
    if (allowed) success(authzHint.value)
    else notifyError(authzHint.value)
  } catch (e) {
    authzHint.value = e instanceof Error ? e.message : '权限预检失败'
    notifyError(authzHint.value)
  } finally {
    authzChecking.value = false
  }
}

async function runQuery() {
  await executeQuery()
  if (!actionError.value) {
    history.push({ mode: queryMode.value, form: { ...queryForm.value } })
  } else {
    notifyError(actionError.value)
  }
}

function applyHistory(id: string) {
  const item = history.items.value.find((x) => x.id === id)
  if (!item) return
  queryMode.value = item.mode
  queryForm.value = { ...queryForm.value, ...item.form }
  showHistory.value = false
}

function startRename(id: string) {
  const item = history.items.value.find((x) => x.id === id)
  if (!item) return
  renamingId.value = id
  renameDraft.value = item.name || history.titleOf(item)
}

function commitRename() {
  if (!renamingId.value) return
  history.rename(renamingId.value, renameDraft.value)
  renamingId.value = null
  renameDraft.value = ''
}

function cancelRename() {
  renamingId.value = null
  renameDraft.value = ''
}

function confirmClearHistory() {
  history.clear({ keepPinned: true })
  clearHistoryOpen.value = false
  success('已清空未收藏历史')
}

function exportHistory() {
  const payload = history.exportPayload()
  downloadText(`mts-query-history-${Date.now()}.json`, JSON.stringify(payload, null, 2), 'application/json')
  success(`已导出 ${payload.items.length} 条历史`)
}

function triggerImportHistory() {
  historyFileInput.value?.click()
}

async function onHistoryFileChange(ev: Event) {
  const input = ev.target as HTMLInputElement
  const file = input.files?.[0]
  input.value = ''
  if (!file) return
  try {
    const text = await file.text()
    const raw = JSON.parse(text) as unknown
    const res = history.importPayload(raw, { merge: true })
    if (!res.ok) {
      notifyError(res.error)
      return
    }
    success(`已合并导入 ${res.count} 条历史`)
    showHistory.value = true
  } catch (e) {
    notifyError(e instanceof Error ? e.message : '导入历史失败')
  }
}

async function copyResults() {
  const text = resultTextForCopy()
  if (!text) { actionError.value = '暂无可复制内容'; return }
  try {
    if (navigator.clipboard?.writeText) await navigator.clipboard.writeText(text)
    else {
      const area = document.createElement('textarea')
      area.value = text
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
    await apiPost('/api/v1/data/delete', {
      request: {
        database: query.database,
        retention_policy: query.retention_policy,
        measurement: query.measurement,
        tags: query.tags,
        start_time: query.start_time,
        end_time: query.end_time,
        precision: query.precision,
      },
    })
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

function exportCSV() {
  if (!rows.value.length) {
    actionError.value = '无可导出的行结果'
    notifyError(actionError.value)
    return
  }
  downloadText(`mts-query-${Date.now()}.csv`, rowsToCSV(rows.value))
  success('CSV 已导出')
}

const historyPreview = computed(() => history.items.value.slice(0, 20))
const columnRows = computed(() => {
  // columns: [{field_name, timestamps, values, tags, measurement}]
  return (columnSeries.value as Array<Record<string, unknown>>).map((c) => ({
    measurement: String(c.measurement ?? ''),
    field: String(c.field_name ?? c.FieldName ?? ''),
    tags: c.tags && typeof c.tags === 'object' ? JSON.stringify(c.tags) : '—',
    points: Array.isArray(c.timestamps) ? c.timestamps.length : (Array.isArray(c.values) ? (c.values as unknown[]).length : 0),
  }))
})
</script>

<template>
  <div class="space-y-4">
    <div class="flex flex-wrap items-center justify-between gap-2">
      <div class="flex flex-wrap gap-2">
        <button
          v-for="m in modeOptions" :key="m.value"
          class="rounded-lg border px-3 py-1.5 text-xs"
          :class="queryMode === m.value ? 'border-slate-800 bg-slate-800 text-white dark:bg-slate-100 dark:text-slate-900' : 'border-slate-200 bg-white dark:border-slate-700 dark:bg-slate-900'"
          @click="queryMode = m.value"
        >{{ m.label }}</button>
      </div>
      <div class="flex gap-2">
        <button class="inline-flex items-center gap-1 rounded-lg border px-3 py-1.5 text-xs dark:border-slate-700" @click="showHistory = !showHistory" title="Ctrl/⌘+H"><History class="h-3.5 w-3.5" />历史</button>
        <button class="inline-flex items-center gap-1 rounded-lg border px-3 py-1.5 text-xs dark:border-slate-700" @click="showChart = !showChart"><BarChart3 class="h-3.5 w-3.5" />图</button>
        <button class="inline-flex items-center gap-1 rounded-lg border px-3 py-1.5 text-xs dark:border-slate-700" @click="showRawFields = !showRawFields">{{ showRawFields ? '标量字段' : '原始字段' }}</button>
        <button class="mts-btn" :disabled="authzChecking" @click="checkAuthz('read')">权限预检</button>
      </div>
    </div>

    <p v-if="authzHint" class="rounded-lg border border-slate-200 bg-slate-50 px-3 py-2 text-xs text-slate-700 dark:border-slate-700 dark:bg-slate-900 dark:text-slate-200">{{ authzHint }}</p>
    <p v-if="metaHint" class="rounded-lg border border-amber-200 bg-amber-50 dark:border-amber-900 dark:bg-amber-950/40 px-3 py-2 text-xs text-amber-900 dark:border-amber-900 dark:bg-amber-950 dark:text-amber-100">{{ metaHint }}（来源: {{ metaSource }}）</p>

    <div v-if="showHistory" class="rounded-xl border border-slate-200 bg-white p-3 dark:border-slate-700 dark:bg-slate-900">
      <div class="mb-2 flex flex-wrap items-center justify-between gap-2 text-sm font-semibold">
        <span>{{ t('queryHistory') }}</span>
        <div class="flex flex-wrap items-center gap-2">
          <button
            type="button"
            class="inline-flex items-center gap-1 text-xs text-slate-500 hover:text-slate-700 dark:text-slate-400 dark:hover:text-slate-200"
            @click="exportHistory"
          ><Download class="h-3 w-3" />导出</button>
          <button
            type="button"
            class="inline-flex items-center gap-1 text-xs text-slate-500 hover:text-slate-700 dark:text-slate-400 dark:hover:text-slate-200"
            @click="triggerImportHistory"
          ><Upload class="h-3 w-3" />导入</button>
          <button
            type="button"
            class="text-xs text-slate-500 hover:text-slate-700 dark:text-slate-400 dark:hover:text-slate-200"
            :disabled="!historyPreview.length"
            @click="clearHistoryOpen = true"
          >{{ t('clearHistory') }}</button>
          <input
            ref="historyFileInput"
            type="file"
            accept="application/json,.json"
            class="hidden"
            @change="onHistoryFileChange"
          />
        </div>
      </div>
      <p class="mb-2 text-[11px] text-slate-400 dark:text-slate-500">快捷键：Ctrl/⌘+Enter 查询 · Esc 取消 · Ctrl/⌘+H 历史 · Ctrl/⌘+Shift+C 复制</p>
      <ul class="max-h-64 space-y-1 overflow-auto sm:max-h-80">
        <li
          v-for="h in historyPreview"
          :key="h.id"
          class="rounded-lg border border-transparent px-2 py-1.5 hover:border-slate-200 hover:bg-slate-50 dark:hover:border-slate-700 dark:hover:bg-slate-800"
        >
          <div v-if="renamingId === h.id" class="flex flex-wrap items-center gap-1">
            <input
              v-model="renameDraft"
              class="min-w-0 flex-1 rounded border px-2 py-1 text-xs dark:border-slate-600 dark:bg-slate-800"
              @keyup.enter="commitRename"
              @keyup.escape="cancelRename"
            />
            <button type="button" class="rounded border px-2 py-1 text-xs dark:border-slate-600" @click="commitRename">保存</button>
            <button type="button" class="rounded border px-2 py-1 text-xs dark:border-slate-600" @click="cancelRename">取消</button>
          </div>
          <div v-else class="flex items-start gap-2">
            <button
              type="button"
              class="mt-0.5 shrink-0 rounded p-0.5"
              :class="h.pinned ? 'text-amber-500' : 'text-slate-300 hover:text-amber-400 dark:text-slate-600'"
              :title="h.pinned ? '取消收藏' : '收藏'"
              @click.stop="history.togglePin(h.id)"
            >
              <Star class="h-3.5 w-3.5" :fill="h.pinned ? 'currentColor' : 'none'" />
            </button>
            <button type="button" class="min-w-0 flex-1 text-left" @click="applyHistory(h.id)">
              <div class="truncate text-xs font-medium text-slate-800 dark:text-slate-100">{{ history.titleOf(h) }}</div>
              <div class="mt-0.5 flex flex-wrap gap-x-2 text-[11px] text-slate-400 dark:text-slate-500">
                <span>{{ h.mode }}</span>
                <span class="truncate">{{ h.form.database }}/{{ h.form.measurement || '*' }}</span>
                <span>{{ new Date(h.at).toLocaleString() }}</span>
              </div>
            </button>
            <div class="flex shrink-0 gap-0.5">
              <button type="button" class="rounded p-1 text-slate-400 hover:bg-slate-100 hover:text-slate-700 dark:hover:bg-slate-700" title="重命名" @click.stop="startRename(h.id)">
                <Pencil class="h-3.5 w-3.5" />
              </button>
              <button type="button" class="rounded p-1 text-slate-400 hover:bg-slate-100 hover:text-red-600 dark:hover:bg-slate-700" title="删除" @click.stop="history.remove(h.id)">
                <X class="h-3.5 w-3.5" />
              </button>
            </div>
          </div>
        </li>
      </ul>
      <p v-if="!historyPreview.length" class="text-xs text-slate-400 dark:text-slate-500">暂无历史</p>
    </div>

    <div class="grid grid-cols-1 gap-3 rounded-xl border border-slate-200 bg-white p-3 dark:border-slate-700 dark:bg-slate-900 sm:p-4 md:grid-cols-2 lg:grid-cols-3">
      <label class="text-xs text-slate-500 dark:text-slate-400 dark:text-slate-500">Database
        <input v-model="queryForm.database" list="db-list" class="mt-1 w-full rounded border px-2 py-1.5 text-sm dark:border-slate-600 dark:bg-slate-800" placeholder="手动输入或选择" />
        <datalist id="db-list"><option v-for="db in databases" :key="db" :value="db" /></datalist>
      </label>
      <label class="text-xs text-slate-500 dark:text-slate-400 dark:text-slate-500">Measurement
        <input v-model="queryForm.measurement" list="meas-list" class="mt-1 w-full rounded border px-2 py-1.5 text-sm dark:border-slate-600 dark:bg-slate-800" :disabled="measurementsLoading" />
        <datalist id="meas-list"><option v-for="m in measurements" :key="m" :value="m" /></datalist>
      </label>
      <label class="text-xs text-slate-500 dark:text-slate-400 dark:text-slate-500">RP
        <input v-model="queryForm.retention_policy" list="rp-list" class="mt-1 w-full rounded border px-2 py-1.5 text-sm dark:border-slate-600 dark:bg-slate-800" />
        <datalist id="rp-list">
          <option v-for="rp in (retentionPolicies.length ? retentionPolicies : ['autogen'])" :key="rp" :value="rp" />
        </datalist>
      </label>
      <label class="text-xs text-slate-500 dark:text-slate-400 dark:text-slate-500">Start (ms)
        <div class="mt-1 flex gap-1">
          <input v-model="queryForm.start_time" class="w-full rounded border px-2 py-1.5 text-sm dark:border-slate-600 dark:bg-slate-800" />
          <button class="rounded border px-2 text-xs" @click="fillNowMs('start')">now</button>
        </div>
      </label>
      <label class="text-xs text-slate-500 dark:text-slate-400 dark:text-slate-500">End (ms)
        <div class="mt-1 flex gap-1">
          <input v-model="queryForm.end_time" class="w-full rounded border px-2 py-1.5 text-sm dark:border-slate-600 dark:bg-slate-800" />
          <button class="rounded border px-2 text-xs" @click="fillNowMs('end')">now</button>
        </div>
      </label>
      <label class="text-xs text-slate-500 dark:text-slate-400 dark:text-slate-500">Fields
        <input v-model="queryForm.fields" placeholder="f1,f2" class="mt-1 w-full rounded border px-2 py-1.5 text-sm dark:border-slate-600 dark:bg-slate-800" />
      </label>
      <label class="text-xs text-slate-500 dark:text-slate-400 dark:text-slate-500">Tags (key=value,...)
        <input v-model="queryForm.tags" placeholder="host=s1,region=cn" class="mt-1 w-full rounded border px-2 py-1.5 text-sm dark:border-slate-600 dark:bg-slate-800" />
      </label>
      <label class="text-xs text-slate-500 dark:text-slate-400 dark:text-slate-500">Order
        <select v-model="queryForm.order" class="mt-1 w-full rounded border px-2 py-1.5 text-sm dark:border-slate-600 dark:bg-slate-800">
          <option value="">默认</option>
          <option value="asc">time asc</option>
          <option value="desc">time desc</option>
        </select>
      </label>
      
      <label class="text-xs mts-muted">聚合 func:field
        <input v-model="queryForm.aggregates" class="mts-input mt-1 font-mono text-xs" placeholder="mean:usage,max:usage" />
      </label>
      <label class="text-xs mts-muted">窗口 window
        <input v-model="queryForm.window" class="mts-input mt-1 font-mono text-xs" placeholder="1m" />
      </label>
      <label class="text-xs mts-muted">Group tags
        <input v-model="queryForm.group_tags" class="mts-input mt-1 font-mono text-xs" placeholder="host,region" />
      </label>
<label class="text-xs text-slate-500 dark:text-slate-400 dark:text-slate-500">Offset / Limit
        <div class="mt-1 flex gap-1">
          <input v-model="queryForm.offset" placeholder="offset" class="w-1/2 rounded border px-2 py-1.5 text-sm dark:border-slate-600 dark:bg-slate-800" />
          <input v-model="queryForm.limit" placeholder="limit" class="w-1/2 rounded border px-2 py-1.5 text-sm dark:border-slate-600 dark:bg-slate-800" />
        </div>
      </label>
    </div>

    <div class="flex flex-wrap gap-2">
      <button class="inline-flex items-center gap-1 rounded-lg bg-slate-800 px-4 py-2 text-sm text-white disabled:opacity-50 dark:bg-slate-100 dark:bg-slate-800 dark:text-slate-900" :disabled="loading" @click="runQuery">
        <Search class="h-4 w-4" /> {{ loading ? t('loading') : t('query') }}
      </button>
      <button class="inline-flex items-center gap-1 rounded-lg border px-3 py-2 text-sm dark:border-slate-700" :disabled="!loading" @click="cancelQuery"><Square class="h-4 w-4" />取消</button>
      <button class="inline-flex items-center gap-1 rounded-lg border border-red-200 px-3 py-2 text-sm text-red-700 dark:text-red-200" @click="deleteOpen = true"><Trash2 class="h-4 w-4" />范围删除</button>
      <button class="inline-flex items-center gap-1 rounded-lg border px-3 py-2 text-sm dark:border-slate-700" @click="copyResults">
        <component :is="copyState === 'ok' ? Check : Copy" class="h-4 w-4" />
        {{ streamMeta.previewOnly ? t('copyPreview') : t('copy') }}
      </button>
        <button class="mts-btn" :disabled="!rows.length" @click="exportCSV">
          <Download class="h-3.5 w-3.5" /> CSV
        </button>
    </div>

    <p v-if="actionError" class="rounded-xl border border-red-200 bg-red-50 dark:border-red-900 dark:bg-red-950/40 p-3 text-sm text-red-700 dark:text-red-200">{{ actionError }}</p>
    <p v-if="deleteResult" class="rounded-xl border border-slate-200 bg-slate-50 dark:border-slate-700 dark:bg-slate-900/60 p-3 text-sm">{{ deleteResult }}</p>

    <div v-if="queryStats" class="grid grid-cols-2 gap-2 sm:grid-cols-5">
      <div class="rounded-xl border bg-white p-3 dark:border-slate-700 dark:bg-slate-900"><p class="text-[11px] text-slate-400 dark:text-slate-500">扫描</p><p class="text-lg font-semibold">{{ queryStats.shards_scanned }}</p></div>
      <div class="rounded-xl border bg-white p-3 dark:border-slate-700 dark:bg-slate-900"><p class="text-[11px] text-slate-400 dark:text-slate-500">跳过</p><p class="text-lg font-semibold">{{ queryStats.shards_skipped }}</p></div>
      <div class="rounded-xl border bg-white p-3 dark:border-slate-700 dark:bg-slate-900"><p class="text-[11px] text-slate-400 dark:text-slate-500">读取</p><p class="text-lg font-semibold text-blue-600">{{ queryStats.samples_read }}</p></div>
      <div class="rounded-xl border bg-white p-3 dark:border-slate-700 dark:bg-slate-900"><p class="text-[11px] text-slate-400 dark:text-slate-500">返回</p><p class="text-lg font-semibold text-green-600">{{ queryStats.samples_returned }}</p></div>
      <div class="rounded-xl border bg-white p-3 dark:border-slate-700 dark:bg-slate-900"><p class="text-[11px] text-slate-400 dark:text-slate-500">耗时</p><p class="text-lg font-semibold text-amber-600">{{ (queryStats.duration_nanos / 1e6).toFixed(1) }}ms</p></div>
    </div>

    <QueryChart v-if="showChart && rows.length" :rows="rows" />

    <div v-if="rows.length" class="overflow-hidden rounded-2xl border bg-white dark:border-slate-700 dark:bg-slate-900">
      <div class="flex justify-between border-b px-4 py-2 text-sm dark:border-slate-800">
        <span class="font-semibold">行结果</span>
        <span class="text-xs text-slate-500 dark:text-slate-400">{{ rows.length }} 行（虚拟滚动）</span>
      </div>
      <div class="overflow-x-auto">
        <div class="grid min-w-[720px] grid-cols-4 border-b px-4 py-2 text-left text-[11px] uppercase text-slate-500 dark:border-slate-800 dark:text-slate-400">
          <span>时间</span><span>Measurement</span><span>Tags</span><span>Fields</span>
        </div>
        <VirtualTable :items="rows" :row-height="40" :height="400">
          <template #default="{ item: row }">
            <div class="grid min-w-[720px] grid-cols-4 items-center border-b px-4 text-xs dark:border-slate-800">
              <span class="font-mono">{{ formatTimestamp(row.timestamp) }}</span>
              <span>{{ row.measurement }}</span>
              <span class="truncate font-mono text-slate-500 dark:text-slate-400">{{ row.tags && Object.keys(row.tags).length ? JSON.stringify(row.tags) : '—' }}</span>
              <span class="truncate font-mono">{{ showRawFields ? JSON.stringify(row.fields) : formatFieldsMap(row.fields as any) }}</span>
            </div>
          </template>
        </VirtualTable>
      </div>
    </div>

    <div v-if="columnRows.length" class="overflow-hidden rounded-2xl border bg-white dark:border-slate-700 dark:bg-slate-900">
      <div class="flex justify-between border-b px-4 py-2 text-sm dark:border-slate-800"><span class="font-semibold">列结果摘要</span><span class="text-xs text-slate-500 dark:text-slate-400 dark:text-slate-500">{{ columnRows.length }} series</span></div>
      <table class="w-full text-sm">
        <thead>
          <tr class="border-b text-left text-[11px] uppercase text-slate-500 dark:text-slate-400 dark:text-slate-500 dark:border-slate-800">
            <th class="px-4 py-2">Measurement</th><th class="px-4 py-2">Field</th><th class="px-4 py-2">Tags</th><th class="px-4 py-2">Points</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="(c, i) in columnRows" :key="i" class="border-b dark:border-slate-800">
            <td class="px-4 py-2 text-xs">{{ c.measurement }}</td>
            <td class="px-4 py-2 text-xs">{{ c.field }}</td>
            <td class="px-4 py-2 font-mono text-xs text-slate-500 dark:text-slate-400 dark:text-slate-500">{{ c.tags }}</td>
            <td class="px-4 py-2 text-xs">{{ c.points }}</td>
          </tr>
        </tbody>
      </table>
    </div>

    <div v-if="rawOutput" class="overflow-hidden rounded-2xl border bg-white dark:border-slate-700 dark:bg-slate-900">
      <div class="flex flex-wrap items-center justify-between gap-2 border-b px-4 py-2 text-sm dark:border-slate-800">
        <span class="font-semibold">原始输出 / EXPLAIN / 流式</span>
        <div class="flex gap-2 text-xs text-slate-500 dark:text-slate-400 dark:text-slate-500">
          <span v-if="streamMeta.lines">{{ streamMeta.lines }} 行</span>
          <span v-if="streamMeta.previewOnly" class="text-amber-700 dark:text-amber-200">仅预览前 {{ streamMeta.previewLimit }}</span>
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
    <ConfirmDialog
      v-model:open="clearHistoryOpen"
      title="清空查询历史"
      message="将清空未收藏的历史记录；已收藏条目会保留。"
      confirm-label="清空"
      danger
      @confirm="confirmClearHistory"
    />
  </div>
</template>
