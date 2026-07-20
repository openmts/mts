<script setup lang="ts">
import { onMounted, onBeforeUnmount, ref, watch, computed } from 'vue'
import { apiPost } from '@/api/client'
import { useQueryWorkbench } from '@/composables/useQueryWorkbench'
import { useQueryHistory } from '@/composables/useQueryHistory'
import { useNotify } from '@/composables/useNotify'
import { formatCaughtError } from '@/utils/apiError'
import { formatMessage } from '@/utils/formatMessage'
import { useI18n } from '@/composables/useI18n'
import { formatEpoch, nowUnixMsString } from '@/utils/time'
import { formatFieldsMap } from '@/utils/fieldValue'
import { rowsToCSV, downloadText } from '@/utils/csv'
import { loadQueryPrefs, saveQueryPrefs } from '@/utils/queryPrefs'
import { isEditableTarget, matchQueryShortcut } from '@/utils/keyboard'
import { isDirty, snapshotForm } from '@/utils/formDirty'
import { registerDirtyChecker } from '@/utils/routeDirty'
import { latencyFromNanos } from '@/utils/queryLatency'
import {
  RESULT_COLUMN_KEYS,
  gridColClass,
  toggleResultColumn,
  type ResultColumnKey,
  type ResultColumnVisibility,
} from '@/utils/resultColumns'
import QueryChart from '@/components/QueryChart.vue'
import VirtualTable from '@/components/VirtualTable.vue'
import EmptyState from '@/components/EmptyState.vue'
import { checkDatabasePermission } from '@/api/authz'
import { useAuth } from '@/composables/useAuth'
import ConfirmDialog from '@/components/ConfirmDialog.vue'
import { Search, Square, Copy, Check, Trash2, History, BarChart3, Download, Star, Pencil, X, Upload, Columns3 } from 'lucide-vue-next'

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
const resultColumns = ref<ResultColumnVisibility>({ ...initialPrefs.resultColumns })
const showColumnPicker = ref(false)
const queryAttempted = ref(false)
const renameDraft = ref('')
const renamingId = ref<string | null>(null)
const clearHistoryOpen = ref(false)
const historyFileInput = ref<HTMLInputElement | null>(null)
const columnKeys = RESULT_COLUMN_KEYS
const resultGridClass = computed(() => gridColClass(resultColumns.value))
const formBaseline = ref(snapshotForm({ mode: queryMode.value, form: queryForm.value }))
const formDirty = computed(() => isDirty(formBaseline.value, { mode: queryMode.value, form: queryForm.value }))
const latency = computed(() => {
  if (!queryStats.value) return null
  return latencyFromNanos(Number(queryStats.value.duration_nanos || 0))
})

const modeOptions = computed(() => [
  { value: 'rows' as const, label: t.value('queryModeRows') },
  { value: 'columns' as const, label: t.value('queryModeColumns') },
  { value: 'explain' as const, label: 'EXPLAIN' },
  { value: 'stream-row' as const, label: t.value('queryModeStreamRow') },
  { value: 'stream-column' as const, label: t.value('queryModeStreamColumn') },
])

const columnLabel = (key: ResultColumnKey) => {
  const map: Record<ResultColumnKey, string> = {
    time: t.value('queryColTime'),
    measurement: t.value('queryColMeasurement'),
    tags: t.value('queryColTags'),
    fields: t.value('queryColFields'),
  }
  return map[key]
}

function persistPrefs() {
  saveQueryPrefs(typeof localStorage !== 'undefined' ? localStorage : null, PREFS_KEY, {
    showChart: showChart.value,
    showRawFields: showRawFields.value,
    showHistory: showHistory.value,
    resultColumns: { ...resultColumns.value },
  })
}

function onToggleColumn(key: ResultColumnKey) {
  resultColumns.value = toggleResultColumn(resultColumns.value, key)
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

let unregisterDirty: (() => void) | null = null

onMounted(async () => {
  unregisterDirty = registerDirtyChecker('query', () => formDirty.value)
  window.addEventListener('keydown', onQueryKeydown)
  window.addEventListener('beforeunload', onBeforeUnload)
  try { await loadDatabases() }
  catch (e) { actionError.value = formatCaughtError(e) }
  // 初始 meta 加载可能改 database/measurement，完成后记为 clean
  markFormClean()
})
onBeforeUnmount(() => {
  unregisterDirty?.()
  unregisterDirty = null
  window.removeEventListener('keydown', onQueryKeydown)
  window.removeEventListener('beforeunload', onBeforeUnload)
  cancelQuery()
  if (copyTimer) clearTimeout(copyTimer)
})
watch([showChart, showRawFields, showHistory, resultColumns], () => { persistPrefs() }, { deep: true })
watch(() => queryForm.value.database, async (db) => {
  try { await loadDbChildren(db) }
  catch (e) { actionError.value = formatCaughtError(e) }
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
    authzHint.value = t.value('queryNeedDatabase')
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
      ? formatMessage(t.value('queryAuthzPass'), { user: currentUser.value || 'current', db: queryForm.value.database, perm })
      : formatMessage(t.value('queryAuthzDeny'), { user: currentUser.value || 'current', db: queryForm.value.database, perm })
    if (allowed) success(authzHint.value)
    else notifyError(authzHint.value)
  } catch (e) {
    authzHint.value = formatCaughtError(e)
    notifyError(authzHint.value)
  } finally {
    authzChecking.value = false
  }
}

async function runQuery() {
  queryAttempted.value = true
  await executeQuery()
  if (!actionError.value) {
    history.push({ mode: queryMode.value, form: { ...queryForm.value } })
    formBaseline.value = snapshotForm({ mode: queryMode.value, form: queryForm.value })
  } else {
    notifyError(actionError.value)
  }
}

function markFormClean() {
  formBaseline.value = snapshotForm({ mode: queryMode.value, form: queryForm.value })
}

function onBeforeUnload(e: BeforeUnloadEvent) {
  if (!formDirty.value) return
  e.preventDefault()
  e.returnValue = ''
}

function applyHistory(id: string) {
  const item = history.items.value.find((x) => x.id === id)
  if (!item) return
  queryMode.value = item.mode
  queryForm.value = { ...queryForm.value, ...item.form }
  showHistory.value = false
  markFormClean()
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
  success(t.value('queryHistoryCleared'))
}

function exportHistory() {
  const payload = history.exportPayload()
  downloadText(`mts-query-history-${Date.now()}.json`, JSON.stringify(payload, null, 2), 'application/json')
  success(formatMessage(t.value('queryHistoryExported'), { count: payload.items.length }))
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
    success(formatMessage(t.value('queryHistoryImported'), { count: res.count }))
    showHistory.value = true
  } catch (e) {
    notifyError(formatCaughtError(e))
  }
}

async function copyResults() {
  const text = resultTextForCopy()
  if (!text) { actionError.value = t.value('queryCopyEmpty'); return }
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
    deleteResult.value = t.value('queryDeleteSubmitted')
    success(deleteResult.value)
    deleteOpen.value = false
    deleteConfirmText.value = ''
  } catch (e) {
    deleteResult.value = formatCaughtError(e)
    notifyError(deleteResult.value)
  } finally {
    deleteLoading.value = false
  }
}

function exportCSV() {
  if (!rows.value.length) {
    actionError.value = t.value('queryExportEmpty')
    notifyError(actionError.value)
    return
  }
  downloadText(`mts-query-${Date.now()}.csv`, rowsToCSV(rows.value))
  success(t.value('queryCsvExported'))
}

const historyPreview = computed(() => history.items.value.slice(0, 20))
const columnRows = computed(() => {
  // columns: [{field_name, timestamps, values, tags, measurement}]
  return (columnSeries.value as Array<Record<string, unknown>>).map((c) => ({
    measurement: String(c.measurement ?? ''),
    field: String(c.field_name ?? c.FieldName ?? ''),
    tags: c.tags && typeof c.tags === 'object' ? JSON.stringify(c.tags) : t.value('emptyValue'),
    points: Array.isArray(c.timestamps) ? c.timestamps.length : (Array.isArray(c.values) ? (c.values as unknown[]).length : 0),
  }))
})
</script>

<template>
  <div class="space-y-4">
    <div class="flex flex-wrap items-center justify-between gap-2">
      <div class="flex flex-wrap items-center gap-2">
        <button
          v-for="m in modeOptions" :key="m.value"
          class="rounded-lg border px-3 py-1.5 text-xs"
          :class="queryMode === m.value ? 'border-slate-800 bg-slate-800 text-white dark:bg-slate-100 dark:text-slate-900' : 'border-slate-200 bg-white dark:border-slate-700 dark:bg-slate-900'"
          @click="queryMode = m.value"
        >{{ m.label }}</button>
        <span
          v-if="formDirty"
          class="rounded-full bg-amber-100 px-2 py-0.5 text-[11px] font-medium text-amber-800 dark:bg-amber-950/50 dark:text-amber-200"
          :title="t('queryDirtyTitle')"
        >{{ t('queryDirtyBadge') }}</span>
      </div>
      <div class="flex gap-2">
        <button class="inline-flex items-center gap-1 rounded-lg border px-3 py-1.5 text-xs dark:border-slate-700" @click="showHistory = !showHistory" title="Ctrl/⌘+H"><History class="h-3.5 w-3.5" />{{ t('queryHistoryBtn') }}</button>
        <button class="inline-flex items-center gap-1 rounded-lg border px-3 py-1.5 text-xs dark:border-slate-700" @click="showChart = !showChart"><BarChart3 class="h-3.5 w-3.5" />{{ t('queryChartBtn') }}</button>
        <button class="inline-flex items-center gap-1 rounded-lg border px-3 py-1.5 text-xs dark:border-slate-700" @click="showRawFields = !showRawFields">{{ showRawFields ? t('queryScalarFields') : t('queryRawFields') }}</button>
        <button class="mts-btn" :disabled="authzChecking" @click="checkAuthz('read')">{{ t('queryAuthzCheck') }}</button>
      </div>
    </div>

    <p v-if="authzHint" class="rounded-lg border border-slate-200 bg-slate-50 px-3 py-2 text-xs text-slate-700 dark:border-slate-700 dark:bg-slate-900 dark:text-slate-200">{{ authzHint }}</p>
    <p v-if="metaHint" class="rounded-lg border border-amber-200 bg-amber-50 dark:border-amber-900 dark:bg-amber-950/40 px-3 py-2 text-xs text-amber-900 dark:border-amber-900 dark:bg-amber-950 dark:text-amber-100">{{ metaHint }}（{{ t('queryMetaSource') }}: {{ metaSource }}）</p>

    <div v-if="showHistory" class="rounded-xl border border-slate-200 bg-white p-3 dark:border-slate-700 dark:bg-slate-900">
      <div class="mb-2 flex flex-wrap items-center justify-between gap-2 text-sm font-semibold">
        <span>{{ t('queryHistory') }}</span>
        <div class="flex flex-wrap items-center gap-2">
          <button
            type="button"
            class="inline-flex items-center gap-1 text-xs text-slate-500 hover:text-slate-700 dark:text-slate-400 dark:hover:text-slate-200"
            @click="exportHistory"
          ><Download class="h-3 w-3" />{{ t('queryExport') }}</button>
          <button
            type="button"
            class="inline-flex items-center gap-1 text-xs text-slate-500 hover:text-slate-700 dark:text-slate-400 dark:hover:text-slate-200"
            @click="triggerImportHistory"
          ><Upload class="h-3 w-3" />{{ t('queryImport') }}</button>
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
      <p class="mb-2 text-[11px] text-slate-400 dark:text-slate-500">{{ t('queryShortcutsHint') }}</p>
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
            <button type="button" class="rounded border px-2 py-1 text-xs dark:border-slate-600" @click="commitRename">{{ t('querySave') }}</button>
            <button type="button" class="rounded border px-2 py-1 text-xs dark:border-slate-600" @click="cancelRename">{{ t('cancel') }}</button>
          </div>
          <div v-else class="flex items-start gap-2">
            <button
              type="button"
              class="mt-0.5 shrink-0 rounded p-0.5"
              :class="h.pinned ? 'text-amber-500' : 'text-slate-300 hover:text-amber-400 dark:text-slate-600'"
              :title="h.pinned ? t('queryUnpin') : t('queryPin')"
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
              <button type="button" class="rounded p-1 text-slate-400 hover:bg-slate-100 hover:text-slate-700 dark:hover:bg-slate-700" :title="t('queryRename')" @click.stop="startRename(h.id)">
                <Pencil class="h-3.5 w-3.5" />
              </button>
              <button type="button" class="rounded p-1 text-slate-400 hover:bg-slate-100 hover:text-red-600 dark:hover:bg-slate-700" :title="t('delete')" @click.stop="history.remove(h.id)">
                <X class="h-3.5 w-3.5" />
              </button>
            </div>
          </div>
        </li>
      </ul>
      <EmptyState v-if="!historyPreview.length" compact :title="t('queryHistoryEmpty')" :description="t('queryHistoryEmptyDesc')" />
    </div>

    <div class="grid grid-cols-1 gap-3 rounded-xl border border-slate-200 bg-white p-3 dark:border-slate-700 dark:bg-slate-900 sm:p-4 md:grid-cols-2 lg:grid-cols-3">
      <label class="text-xs text-slate-500 dark:text-slate-400 dark:text-slate-500">{{ t('database') }}
        <input v-model="queryForm.database" list="db-list" class="mt-1 w-full rounded border px-2 py-1.5 text-sm dark:border-slate-600 dark:bg-slate-800" :placeholder="t('queryPlaceholderManual')" />
        <datalist id="db-list"><option v-for="db in databases" :key="db" :value="db" /></datalist>
      </label>
      <label class="text-xs text-slate-500 dark:text-slate-400 dark:text-slate-500">{{ t('measurement') }}
        <input v-model="queryForm.measurement" list="meas-list" class="mt-1 w-full rounded border px-2 py-1.5 text-sm dark:border-slate-600 dark:bg-slate-800" :disabled="measurementsLoading" />
        <datalist id="meas-list"><option v-for="m in measurements" :key="m" :value="m" /></datalist>
      </label>
      <label class="text-xs text-slate-500 dark:text-slate-400 dark:text-slate-500">{{ t('retentionPolicy') }}
        <input v-model="queryForm.retention_policy" list="rp-list" class="mt-1 w-full rounded border px-2 py-1.5 text-sm dark:border-slate-600 dark:bg-slate-800" />
        <datalist id="rp-list">
          <option v-for="rp in (retentionPolicies.length ? retentionPolicies : ['autogen'])" :key="rp" :value="rp" />
        </datalist>
      </label>
      <label class="text-xs text-slate-500 dark:text-slate-400 dark:text-slate-500">{{ t('queryStartMs') }}
        <div class="mt-1 flex gap-1">
          <input v-model="queryForm.start_time" class="w-full rounded border px-2 py-1.5 text-sm dark:border-slate-600 dark:bg-slate-800" />
          <button class="rounded border px-2 text-xs" @click="fillNowMs('start')">{{ t('queryNow') }}</button>
        </div>
      </label>
      <label class="text-xs text-slate-500 dark:text-slate-400 dark:text-slate-500">{{ t('queryEndMs') }}
        <div class="mt-1 flex gap-1">
          <input v-model="queryForm.end_time" class="w-full rounded border px-2 py-1.5 text-sm dark:border-slate-600 dark:bg-slate-800" />
          <button class="rounded border px-2 text-xs" @click="fillNowMs('end')">{{ t('queryNow') }}</button>
        </div>
      </label>
      <label class="text-xs text-slate-500 dark:text-slate-400 dark:text-slate-500">{{ t('fields') }}
        <input v-model="queryForm.fields" :placeholder="t('queryPhFields')" class="mt-1 w-full rounded border px-2 py-1.5 text-sm dark:border-slate-600 dark:bg-slate-800" />
      </label>
      <label class="text-xs text-slate-500 dark:text-slate-400 dark:text-slate-500">{{ t('queryTagsExpr') }}
        <input v-model="queryForm.tags" :placeholder="t('queryPhTags')" class="mt-1 w-full rounded border px-2 py-1.5 text-sm dark:border-slate-600 dark:bg-slate-800" />
      </label>
      <label class="text-xs text-slate-500 dark:text-slate-400 dark:text-slate-500">{{ t('order') }}
        <select v-model="queryForm.order" class="mt-1 w-full rounded border px-2 py-1.5 text-sm dark:border-slate-600 dark:bg-slate-800">
          <option value="">{{ t('queryDefault') }}</option>
          <option value="asc">{{ t('queryOrderAsc') }}</option>
          <option value="desc">{{ t('queryOrderDesc') }}</option>
        </select>
      </label>
      
      <label class="text-xs mts-muted">{{ t('queryAggFunc') }}
        <input v-model="queryForm.aggregates" class="mts-input mt-1 font-mono text-xs" :placeholder="t('queryPhAggregates')" />
      </label>
      <label class="text-xs mts-muted">{{ t('queryWindow') }}
        <input v-model="queryForm.window" class="mts-input mt-1 font-mono text-xs" :placeholder="t('queryPhWindow')" />
      </label>
      <label class="text-xs mts-muted">{{ t('queryGroupTags') }}
        <input v-model="queryForm.group_tags" class="mts-input mt-1 font-mono text-xs" :placeholder="t('queryPhGroupTags')" />
      </label>
<label class="text-xs text-slate-500 dark:text-slate-400 dark:text-slate-500">{{ t('queryOffsetLimit') }}
        <div class="mt-1 flex gap-1">
          <input v-model="queryForm.offset" :placeholder="t('queryPhOffset')" class="w-1/2 rounded border px-2 py-1.5 text-sm dark:border-slate-600 dark:bg-slate-800" />
          <input v-model="queryForm.limit" :placeholder="t('queryPhLimit')" class="w-1/2 rounded border px-2 py-1.5 text-sm dark:border-slate-600 dark:bg-slate-800" />
        </div>
      </label>
    </div>

    <div class="flex flex-wrap gap-2">
      <button class="inline-flex items-center gap-1 rounded-lg bg-slate-800 px-4 py-2 text-sm text-white disabled:opacity-50 dark:bg-slate-100 dark:bg-slate-800 dark:text-slate-900" :disabled="loading" @click="runQuery">
        <Search class="h-4 w-4" /> {{ loading ? t('loading') : t('query') }}
      </button>
      <button class="inline-flex items-center gap-1 rounded-lg border px-3 py-2 text-sm dark:border-slate-700" :disabled="!loading" @click="cancelQuery"><Square class="h-4 w-4" />{{ t('queryCancel') }}</button>
      <button class="inline-flex items-center gap-1 rounded-lg border border-red-200 px-3 py-2 text-sm text-red-700 dark:text-red-200" @click="deleteOpen = true"><Trash2 class="h-4 w-4" />{{ t('queryRangeDelete') }}</button>
      <button class="inline-flex items-center gap-1 rounded-lg border px-3 py-2 text-sm dark:border-slate-700" @click="copyResults">
        <component :is="copyState === 'ok' ? Check : Copy" class="h-4 w-4" />
        {{ streamMeta.previewOnly ? t('copyPreview') : t('copy') }}
      </button>
        <button class="mts-btn" :disabled="!rows.length" @click="exportCSV">
          <Download class="h-3.5 w-3.5" /> CSV
        </button>
    </div>

    <p v-if="actionError" class="mts-alert-error">{{ actionError }}</p>
    <p v-if="deleteResult" class="mts-alert-ok">{{ deleteResult }}</p>

    <div v-if="queryStats" class="space-y-2">
      <div class="grid grid-cols-2 gap-2 sm:grid-cols-5">
        <div class="rounded-xl border bg-white p-3 dark:border-slate-700 dark:bg-slate-900"><p class="text-[11px] text-slate-400 dark:text-slate-500">{{ t('queryStatScan') }}</p><p class="text-lg font-semibold">{{ queryStats.shards_scanned }}</p></div>
        <div class="rounded-xl border bg-white p-3 dark:border-slate-700 dark:bg-slate-900"><p class="text-[11px] text-slate-400 dark:text-slate-500">{{ t('queryStatSkip') }}</p><p class="text-lg font-semibold">{{ queryStats.shards_skipped }}</p></div>
        <div class="rounded-xl border bg-white p-3 dark:border-slate-700 dark:bg-slate-900"><p class="text-[11px] text-slate-400 dark:text-slate-500">{{ t('queryStatRead') }}</p><p class="text-lg font-semibold text-blue-600">{{ queryStats.samples_read }}</p></div>
        <div class="rounded-xl border bg-white p-3 dark:border-slate-700 dark:bg-slate-900"><p class="text-[11px] text-slate-400 dark:text-slate-500">{{ t('queryStatReturn') }}</p><p class="text-lg font-semibold text-green-600">{{ queryStats.samples_returned }}</p></div>
        <div class="rounded-xl border bg-white p-3 dark:border-slate-700 dark:bg-slate-900"><p class="text-[11px] text-slate-400 dark:text-slate-500">{{ t('queryStatDuration') }}</p><p class="text-lg font-semibold text-amber-600">{{ (queryStats.duration_nanos / 1e6).toFixed(1) }}ms</p></div>
      </div>
      <div v-if="latency" class="mts-card px-3 py-2">
        <div class="mb-1 flex flex-wrap items-center justify-between gap-2 text-xs">
          <span class="text-slate-600 dark:text-slate-300">{{ t('queryLatencyWaterline') }} · <span class="font-medium">{{ latency.label }}</span></span>
          <span class="font-mono text-slate-500 dark:text-slate-400">{{ latency.durationMs.toFixed(1) }} ms · {{ t('queryLatencyLegend') }}</span>
        </div>
        <div class="h-2 overflow-hidden rounded-full bg-slate-100 dark:bg-slate-800">
          <div class="h-full rounded-full transition-all" :class="latency.toneClass" :style="{ width: latency.barPercent + '%' }" />
        </div>
      </div>
    </div>

    <QueryChart v-if="showChart && rows.length" :rows="rows" />

    <div
      v-if="queryAttempted && !loading && !actionError && !rows.length && !columnRows.length && !rawOutput"
      class="mts-card"
    >
      <EmptyState
        :title="t('queryNoMatchTitle')"
        :description="t('queryNoMatchDesc')"
      />
    </div>

    <div v-if="rows.length" class="overflow-hidden rounded-2xl border bg-white dark:border-slate-700 dark:bg-slate-900">
      <div class="flex flex-wrap items-center justify-between gap-2 border-b px-4 py-2 text-sm dark:border-slate-800">
        <span class="font-semibold">{{ t('queryRowResult') }}</span>
        <div class="flex flex-wrap items-center gap-2">
          <span class="text-xs text-slate-500 dark:text-slate-400">{{ formatMessage(t('queryRowCountVirtual'), { count: rows.length }) }}</span>
          <div class="relative">
            <button
              type="button"
              class="mts-btn"
              :title="t('queryColVisibility')"
              @click="showColumnPicker = !showColumnPicker"
            >
              <Columns3 class="h-3.5 w-3.5" /> {{ t('queryColumns') }}
            </button>
            <div
              v-if="showColumnPicker"
              class="absolute right-0 z-20 mt-1 w-44 rounded-lg border border-slate-200 bg-white p-2 shadow-lg dark:border-slate-700 dark:bg-slate-900"
            >
              <label
                v-for="k in columnKeys"
                :key="k"
                class="flex cursor-pointer items-center gap-2 rounded px-2 py-1 text-xs hover:bg-slate-50 dark:hover:bg-slate-800"
              >
                <input
                  type="checkbox"
                  :checked="resultColumns[k]"
                  @change="onToggleColumn(k)"
                />
                <span>{{ columnLabel(k) }}</span>
              </label>
            </div>
          </div>
        </div>
      </div>
      <div class="overflow-x-auto">
        <div
          class="grid min-w-[480px] border-b px-4 py-2 text-left text-[11px] uppercase text-slate-500 dark:border-slate-800 dark:text-slate-400"
          :class="resultGridClass"
        >
          <span v-if="resultColumns.time">{{ t('queryColTime') }}</span>
          <span v-if="resultColumns.measurement">{{ t('queryColMeasurement') }}</span>
          <span v-if="resultColumns.tags">{{ t('queryColTags') }}</span>
          <span v-if="resultColumns.fields">{{ t('queryColFields') }}</span>
        </div>
        <VirtualTable :items="rows" :row-height="40" :height="400">
          <template #default="{ item: row }">
            <div
              class="grid min-w-[480px] items-center border-b px-4 text-xs dark:border-slate-800"
              :class="resultGridClass"
            >
              <span v-if="resultColumns.time" class="font-mono">{{ formatTimestamp(row.timestamp) }}</span>
              <span v-if="resultColumns.measurement">{{ row.measurement }}</span>
              <span v-if="resultColumns.tags" class="truncate font-mono text-slate-500 dark:text-slate-400">{{ row.tags && Object.keys(row.tags).length ? JSON.stringify(row.tags) : t('emptyValue') }}</span>
              <span v-if="resultColumns.fields" class="truncate font-mono">{{ showRawFields ? JSON.stringify(row.fields) : formatFieldsMap(row.fields as any) }}</span>
            </div>
          </template>
        </VirtualTable>
      </div>
    </div>

    <div v-if="columnRows.length" class="overflow-hidden rounded-2xl border bg-white dark:border-slate-700 dark:bg-slate-900">
      <div class="flex justify-between border-b px-4 py-2 text-sm dark:border-slate-800"><span class="font-semibold">{{ t('queryColumnSummary') }}</span><span class="text-xs text-slate-500 dark:text-slate-400 dark:text-slate-500">{{ formatMessage(t('querySeriesCount'), { count: columnRows.length }) }}</span></div>
      <table class="w-full text-sm">
        <thead>
          <tr class="border-b text-left text-[11px] uppercase text-slate-500 dark:text-slate-400 dark:text-slate-500 dark:border-slate-800">
            <th class="px-4 py-2">{{ t('queryColMeasurement') }}</th><th class="px-4 py-2">{{ t('field') }}</th><th class="px-4 py-2">{{ t('queryColTags') }}</th><th class="px-4 py-2">{{ t('queryColPoints') }}</th>
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
        <span class="font-semibold">{{ t('queryRawOutput') }}</span>
        <div class="flex gap-2 text-xs text-slate-500 dark:text-slate-400 dark:text-slate-500">
          <span v-if="streamMeta.lines">{{ formatMessage(t('queryStreamLines'), { count: streamMeta.lines }) }}</span>
          <span v-if="streamMeta.previewOnly" class="text-amber-700 dark:text-amber-200">{{ formatMessage(t('queryStreamPreviewOnly'), { limit: streamMeta.previewLimit }) }}</span>
        </div>
      </div>
      <pre class="max-h-96 overflow-auto bg-slate-950 p-4 font-mono text-xs text-emerald-400">{{ rawOutput }}</pre>
    </div>

    <ConfirmDialog
      v-model:open="deleteOpen"
      :title="t('queryDeleteTitle')"
      :message="t('queryDeleteMsg')"
      require-text="DELETE"
      :confirm-label="t('delete')"
      danger
      :loading="deleteLoading"
      @confirm="doRangeDelete"
    />
    <ConfirmDialog
      v-model:open="clearHistoryOpen"
      :title="t('queryClearHistoryTitle')"
      :message="t('queryClearHistoryMsg')"
      :confirm-label="t('clearHistory')"
      danger
      @confirm="confirmClearHistory"
    />
  </div>
</template>
