<script setup lang="ts">
import { onMounted, onBeforeUnmount, ref, watch, computed, nextTick } from 'vue'
import { useRoute } from 'vue-router'
import {
  parseQueryPrefill,
  timeRangeToQueryFormTimes,
  queryFormToPrefill,
} from '@/utils/routePrefill'
import { useHashScroll } from '@/composables/useHashScroll'
import { hashTargetId, scheduleScrollToHash } from '@/utils/hashScroll'
import { apiPost } from '@/api/client'
import { useQueryWorkbench } from '@/composables/useQueryWorkbench'
import { useQueryHistory } from '@/composables/useQueryHistory'
import { filterQueryHistory } from '@/utils/queryHistory'
import { useNotify } from '@/composables/useNotify'
import { formatCaughtError } from '@/utils/apiError'
import { formatMessage } from '@/utils/formatMessage'
import { copyText } from '@/utils/clipboard'
import { useI18n } from '@/composables/useI18n'
import { formatEpoch, nowUnixMsString } from '@/utils/time'
import { formatFieldsMap } from '@/utils/fieldValue'
import { stampFilename } from '@/utils/download'
import { useExportJob } from '@/composables/useExportJob'
import ExportJobBanner from '@/components/ExportJobBanner.vue'
import {
  collectQueryCSVColumns,
  queryCSVHeader,
  queryCSVRow,
} from '@/utils/csv'
import { loadQueryPrefs, saveQueryPrefs } from '@/utils/queryPrefs'
import { isEditableTarget, matchQueryShortcut } from '@/utils/keyboard'
import { isDirty, snapshotForm } from '@/utils/formDirty'
import { useMutationGuard } from '@/composables/useMutationGuard'
import { registerDirtyChecker } from '@/utils/routeDirty'
import { latencyFromNanos } from '@/utils/queryLatency'
import { filterSeriesList, seriesLabel } from '@/utils/seriesMeta'
import { buildQueryDeleteScope, formatQueryDeleteScopeMessage } from '@/utils/queryDeleteScope'
import { detailStatCards, primaryStatCards, toneClass } from '@/utils/queryStatsView'
import type { MessageKey } from '@/i18n/messages'
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
  fieldOptions, seriesOptions, seriesTotal, seriesTruncated, seriesLoading, seriesError,
  loadMeasurementMeta, refreshSeriesWithTags, applySeriesTags,
  queryForm, queryMode, rows, columnSeries, queryStats, rawOutput, streamMeta, actionError, loading,
  engineStatsSource, engineStatsLoading, engineStatsError, engineStatsAt, loadEngineStats,
  loadDatabases, loadDbChildren, executeQuery, cancelQuery, resultTextForCopy, buildQuery,
} = useQueryWorkbench()
const history = useQueryHistory()
const seriesFilter = ref('')
const route = useRoute()
useHashScroll()
const { offline, writeBlocked, blockReason, blockedMessageKey } = useMutationGuard()
const { success, error: notifyError } = useNotify()
const {
  exportJob,
  exportBusy,
  cancelExport,
  resetExport,
  runTextExport,
  runJSONExport,
} = useExportJob()
const { t, locale } = useI18n()
const { currentUser, isAdmin } = useAuth()
const authzHint = ref('')
const authzChecking = ref(false)

const copyState = ref<'idle' | 'ok' | 'fail'>('idle')
let copyTimer: ReturnType<typeof setTimeout> | null = null
const deleteOpen = ref(false)
const deleteLoading = ref(false)
const deleteResult = ref('')
const PREFS_KEY = 'mts_query_prefs'
const initialPrefs = loadQueryPrefs(typeof localStorage !== 'undefined' ? localStorage : null, PREFS_KEY)
const showHistory = ref(initialPrefs.showHistory)
const showChart = ref(initialPrefs.showChart)
const formBaseline = ref(snapshotForm({ mode: queryMode.value, form: queryForm.value }))
const formDirty = computed(() => isDirty(formBaseline.value, { mode: queryMode.value, form: queryForm.value }))

function applyQueryHash(hash?: string | null) {
  const raw = hash ?? (typeof window !== 'undefined' ? window.location.hash : route.hash)
  const id = hashTargetId(raw)
  if (id === 'query-history') showHistory.value = true
  if (id === 'query-chart') showChart.value = true
  // 面板打开后再滚一次，避免 v-if 未挂载
  void nextTick(() => {
    scheduleScrollToHash(raw)
  })
}

/** 深链只读预填：改时间/库表/筛选，不自动执行查询 */
function applyQueryPrefillFromRoute() {
  const pre = parseQueryPrefill(route.query as Record<string, unknown>)
  let changed = false
  if (pre.start_time || pre.end_time) {
    if (pre.start_time && queryForm.value.start_time !== pre.start_time) {
      queryForm.value.start_time = pre.start_time
      changed = true
    }
    if (pre.end_time && queryForm.value.end_time !== pre.end_time) {
      queryForm.value.end_time = pre.end_time
      changed = true
    }
  } else if (pre.range) {
    const times = timeRangeToQueryFormTimes(pre.range)
    if (queryForm.value.start_time !== times.start_time || queryForm.value.end_time !== times.end_time) {
      queryForm.value.start_time = times.start_time
      queryForm.value.end_time = times.end_time
      changed = true
    }
  }
  const assignStr = (key: 'database' | 'measurement' | 'retention_policy' | 'fields' | 'tags' | 'limit' | 'window' | 'aggregates' | 'group_tags' | 'predicates', val?: string) => {
    if (val == null || val === '') return
    if (queryForm.value[key] !== val) {
      queryForm.value[key] = val
      changed = true
    }
  }
  assignStr('database', pre.database)
  assignStr('measurement', pre.measurement)
  assignStr('retention_policy', pre.retention_policy)
  assignStr('fields', pre.fields)
  assignStr('tags', pre.tags)
  assignStr('limit', pre.limit)
  if (pre.order === 'asc' || pre.order === 'desc' || pre.order === '') {
    if (queryForm.value.order !== pre.order) {
      queryForm.value.order = pre.order
      changed = true
    }
  }
  assignStr('window', pre.window)
  assignStr('aggregates', pre.aggregates)
  assignStr('group_tags', pre.group_tags)
  assignStr('predicates', pre.predicates)
  if (changed) {
    formBaseline.value = snapshotForm({ mode: queryMode.value, form: queryForm.value })
    success(t.value('queryPrefillApplied'))
  }
}

async function copyShareLink() {
  const path = queryFormToPrefill(queryForm.value, { hash: '#query-form' })
  const url = `${window.location.origin}${path}`
  const res = await copyText(url)
  if (res.ok) success(t.value('queryShareCopied'))
  else notifyError(res.error || t.value('failed'))
}

watch(
  () => route.hash,
  (h) => applyQueryHash(h),
  { immediate: true },
)
watch(
  () => route.fullPath,
  () => {
    applyQueryPrefillFromRoute()
  },
  { immediate: true },
)
const showRawFields = ref(initialPrefs.showRawFields)
const resultColumns = ref<ResultColumnVisibility>({ ...initialPrefs.resultColumns })
const showColumnPicker = ref(false)
const queryAttempted = ref(false)
const renameDraft = ref('')
const renamingId = ref<string | null>(null)
const clearHistoryOpen = ref(false)
const historyFileInput = ref<HTMLInputElement | null>(null)
const columnKeys = RESULT_COLUMN_KEYS
const QUERY_ROW_HEIGHT = 40
const QUERY_LIST_HEIGHT = 400
const COLUMN_ROW_HEIGHT = 40
const COLUMN_LIST_HEIGHT = 320
const resultGridClass = computed(() => gridColClass(resultColumns.value))
const latency = computed(() => {
  if (!queryStats.value) return null
  return latencyFromNanos(
    Number(queryStats.value.duration_nanos || 0),
    undefined,
    locale.value === 'en' ? 'en' : 'zh',
  )
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
watch(
  () => [queryForm.value.database, queryForm.value.measurement] as const,
  async ([db, measurement]) => {
    seriesFilter.value = ''
    try {
      await loadMeasurementMeta(db, measurement)
    } catch (e) {
      actionError.value = formatCaughtError(e)
    }
  },
)

function formatTimestamp(v: number): string {
  if (Math.abs(v) >= 1e15) return formatEpoch(v, 'ns')
  return formatEpoch(v, 'ms')
}
const filteredSeriesOptions = computed(() => filterSeriesList(seriesOptions.value, seriesFilter.value))
const primaryStatsCards = computed(() => (queryStats.value ? primaryStatCards(queryStats.value) : []))
const detailStatsCards = computed(() => (queryStats.value ? detailStatCards(queryStats.value) : []))
const engineStatsAtLabel = computed(() => {
  if (!engineStatsAt.value) return ''
  return formatEpoch(engineStatsAt.value, 'ms')
})
function statLabel(key: string): string {
  return t.value(key as MessageKey)
}

function fillNowMs(which: 'start' | 'end') {
  const s = nowUnixMsString()
  if (which === 'start') queryForm.value.start_time = s
  else queryForm.value.end_time = s
}

function seriesOptionLabel(s: { id?: number; tags?: Record<string, string>; measurement?: string }): string {
  return seriesLabel(s)
}

function onSeriesSelect(raw: string) {
  if (!raw) return
  const idx = Number(raw)
  if (!Number.isInteger(idx) || idx < 0 || idx >= seriesOptions.value.length) return
  applySeriesTags(seriesOptions.value[idx])
}

function onSeriesSelectFiltered(raw: string) {
  if (!raw) return
  const idx = Number(raw)
  const list = filteredSeriesOptions.value
  if (!Number.isInteger(idx) || idx < 0 || idx >= list.length) return
  applySeriesTags(list[idx])
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

async function exportHistory() {
  if (exportBusy.value) return
  const payload = history.exportPayload()
  const count = payload.items?.length ?? 0
  const outcome = await runJSONExport({
    label: 'JSON',
    filename: stampFilename('mts-query-history', 'json'),
    total: Math.max(count, 1),
    build: async ({ isCancelled, progress }) => {
      progress(0, Math.max(count, 1))
      if (isCancelled()) return null
      progress(Math.max(count, 1), Math.max(count, 1))
      return payload
    },
  })
  if (outcome === 'done') success(formatMessage(t.value('queryHistoryExported'), { count }))
  else if (outcome === 'cancelled') success(t.value('exportCancelledToast'))
  else if (outcome === 'error') notifyError(exportJob.value.error || t.value('failed'))
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

const deleteScopeMessage = computed(() => {
  try {
    const query = buildQuery()
    const scope = buildQueryDeleteScope(query)
    return formatQueryDeleteScopeMessage(scope, {
      database: t.value('database'),
      retention: t.value('retentionPolicy'),
      measurement: t.value('measurement'),
      tags: t.value('queryTagsExpr'),
      start: t.value('queryStartMs'),
      end: t.value('queryEndMs'),
      noTags: t.value('queryDeleteNoTags'),
      warnNoTime: t.value('queryDeleteNoTimeWarn'),
    })
  } catch (e) {
    return formatCaughtError(e)
  }
})

function openRangeDelete() {
  if (writeBlocked.value) {
    notifyError(t.value(blockedMessageKey('offlineDeleteBlocked')))
    return
  }
  deleteOpen.value = true
}

async function doRangeDelete() {
  if (writeBlocked.value) {
    deleteResult.value = t.value(blockedMessageKey('offlineDeleteBlocked'))
    notifyError(deleteResult.value)
    return
  }
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
  } catch (e) {
    deleteResult.value = formatCaughtError(e)
    notifyError(deleteResult.value)
  } finally {
    deleteLoading.value = false
  }
}

async function exportCSV() {
  if (!rows.value.length) {
    actionError.value = t.value('queryExportEmpty')
    notifyError(actionError.value)
    return
  }
  if (exportBusy.value) return
  const list = rows.value.slice()
  const cols = collectQueryCSVColumns(list)
  const outcome = await runTextExport({
    label: 'CSV',
    filename: stampFilename('mts-query', 'csv'),
    mime: 'text/csv;charset=utf-8',
    total: list.length,
    build: async ({ isCancelled, progress }) => {
      const lines: string[] = [queryCSVHeader(cols.tags, cols.fields)]
      progress(0, list.length)
      const chunk = 400
      for (let i = 0; i < list.length; i++) {
        if (isCancelled()) return null
        lines.push(queryCSVRow(list[i], cols.tags, cols.fields))
        const done = i + 1
        if (done === list.length || done % chunk === 0) {
          progress(done, list.length)
          if (done < list.length) await new Promise((r) => setTimeout(r, 0))
        }
      }
      return lines.join('\n')
    },
  })
  if (outcome === 'done') success(t.value('queryCsvExported'))
  else if (outcome === 'cancelled') success(t.value('exportCancelledToast'))
  else if (outcome === 'error') notifyError(exportJob.value.error || t.value('failed'))
}

const HISTORY_ROW_HEIGHT = 56
const HISTORY_LIST_HEIGHT = 320
const historyFilter = ref('')
const historyTotal = computed(() => history.items.value.length)
const historyItems = computed(() => filterQueryHistory(history.items.value, historyFilter.value))
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
  <div class="space-y-4" data-testid="query-page">
    <div id="query-toolbar" class="scroll-mt-20 flex flex-wrap items-center justify-between gap-2">
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
        <button class="inline-flex items-center gap-1 rounded-lg border px-3 py-1.5 text-xs dark:border-slate-700" @click="showHistory = !showHistory" :title="t('queryHistoryShortcutTitle')"><History class="h-3.5 w-3.5" />{{ t('queryHistoryBtn') }}</button>
        <button class="inline-flex items-center gap-1 rounded-lg border px-3 py-1.5 text-xs dark:border-slate-700" @click="showChart = !showChart"><BarChart3 class="h-3.5 w-3.5" />{{ t('queryChartBtn') }}</button>
        <button class="inline-flex items-center gap-1 rounded-lg border px-3 py-1.5 text-xs dark:border-slate-700" @click="showRawFields = !showRawFields">{{ showRawFields ? t('queryScalarFields') : t('queryRawFields') }}</button>
        <button class="mts-btn" :disabled="authzChecking" @click="checkAuthz('read')">{{ t('queryAuthzCheck') }}</button>
      </div>
    </div>

    <p v-if="authzHint" class="rounded-lg border border-slate-200 bg-slate-50 px-3 py-2 text-xs text-slate-700 dark:border-slate-700 dark:bg-slate-900 dark:text-slate-200">{{ authzHint }}</p>
    <p v-if="metaHint || metaSource === 'manual'" class="rounded-lg border border-amber-200 bg-amber-50 dark:border-amber-900 dark:bg-amber-950/40 px-3 py-2 text-xs text-amber-900 dark:border-amber-900 dark:bg-amber-950 dark:text-amber-100">{{ metaHint || t('metaDbManualHint') }}（{{ t('queryMetaSource') }}: {{ metaSource }}）</p>

    <div
      id="query-history"
      v-if="showHistory"
      class="scroll-mt-20 rounded-xl border border-slate-200 bg-white p-3 dark:border-slate-700 dark:bg-slate-900"
      data-testid="query-history-panel"
    >
      <div class="mb-2 flex flex-wrap items-center justify-between gap-2 text-sm font-semibold">
        <span>{{ t('queryHistory') }}</span>
        <div class="flex flex-wrap items-center gap-2">
          <span class="text-[11px] font-normal text-slate-400" data-testid="query-history-count">{{ historyItems.length }}</span>
          <button
            type="button"
            class="inline-flex items-center gap-1 text-xs text-slate-500 hover:text-slate-700 dark:text-slate-400 dark:hover:text-slate-200"
            data-testid="query-export-history" :disabled="exportBusy"
            @click="exportHistory"
          ><Download class="h-3 w-3" />{{ t('queryExport') }}</button>
          <button
            type="button"
            class="inline-flex items-center gap-1 text-xs text-slate-500 hover:text-slate-700 dark:text-slate-400 dark:hover:text-slate-200"
            data-testid="query-import-history"
            @click="triggerImportHistory"
          ><Upload class="h-3 w-3" />{{ t('queryImport') }}</button>
          <button
            type="button"
            class="text-xs text-slate-500 hover:text-slate-700 dark:text-slate-400 dark:hover:text-slate-200"
            data-testid="query-clear-history"
            :disabled="!historyTotal"
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
      <label class="mb-2 block text-xs text-slate-500 dark:text-slate-400">
        {{ t('queryHistoryFilter') }}
        <input
          v-model="historyFilter"
          data-testid="query-history-filter"
          class="mt-1 w-full rounded border px-2 py-1.5 text-sm dark:border-slate-600 dark:bg-slate-800"
          :placeholder="t('queryHistoryFilterPh')"
        />
      </label>
      <div v-if="historyItems.length" class="overflow-hidden rounded-lg border border-slate-100 dark:border-slate-800">
        <VirtualTable
          :items="historyItems"
          :row-height="HISTORY_ROW_HEIGHT"
          :height="Math.min(HISTORY_LIST_HEIGHT, Math.max(168, historyItems.length * HISTORY_ROW_HEIGHT))"
          data-testid="query-history-virtual-list"
        >
          <template #default="{ item: h, index }">
            <div
              class="flex h-full items-stretch border-b border-transparent px-2 py-1 hover:border-slate-200 hover:bg-slate-50 dark:hover:border-slate-700 dark:hover:bg-slate-800"
              :data-testid="`query-history-row-${index}`"
            >
              <div v-if="renamingId === h.id" class="flex w-full flex-wrap items-center gap-1 py-1">
                <input
                  v-model="renameDraft"
                  class="min-w-0 flex-1 rounded border px-2 py-1 text-xs dark:border-slate-600 dark:bg-slate-800"
                  data-testid="query-history-rename-input"
                  @keyup.enter="commitRename"
                  @keyup.escape="cancelRename"
                />
                <button type="button" class="rounded border px-2 py-1 text-xs dark:border-slate-600" @click="commitRename">{{ t('querySave') }}</button>
                <button type="button" class="rounded border px-2 py-1 text-xs dark:border-slate-600" @click="cancelRename">{{ t('cancel') }}</button>
              </div>
              <div v-else class="flex w-full items-start gap-2 py-1">
                <button
                  type="button"
                  class="mt-0.5 shrink-0 rounded p-0.5"
                  :class="h.pinned ? 'text-amber-500' : 'text-slate-300 hover:text-amber-400 dark:text-slate-600'"
                  :title="h.pinned ? t('queryUnpin') : t('queryPin')"
                  :data-testid="`query-history-pin-${index}`"
                  @click.stop="history.togglePin(h.id)"
                >
                  <Star class="h-3.5 w-3.5" :fill="h.pinned ? 'currentColor' : 'none'" />
                </button>
                <button type="button" class="min-w-0 flex-1 text-left" :data-testid="`query-history-apply-${index}`" @click="applyHistory(h.id)">
                  <div class="truncate text-xs font-medium text-slate-800 dark:text-slate-100">{{ history.titleOf(h) }}</div>
                  <div class="mt-0.5 flex flex-wrap gap-x-2 text-[11px] text-slate-400 dark:text-slate-500">
                    <span>{{ h.mode }}</span>
                    <span class="truncate">{{ h.form.database }}/{{ h.form.measurement || '*' }}</span>
                    <span>{{ new Date(h.at).toLocaleString() }}</span>
                  </div>
                </button>
                <div class="flex shrink-0 gap-0.5">
                  <button type="button" class="rounded p-1 text-slate-400 hover:bg-slate-100 hover:text-slate-700 dark:hover:bg-slate-700" :title="t('queryRename')" :data-testid="`query-history-rename-${index}`" @click.stop="startRename(h.id)">
                    <Pencil class="h-3.5 w-3.5" />
                  </button>
                  <button type="button" class="rounded p-1 text-slate-400 hover:bg-slate-100 hover:text-red-600 dark:hover:bg-slate-700" :title="t('delete')" :data-testid="`query-history-remove-${index}`" @click.stop="history.remove(h.id)">
                    <X class="h-3.5 w-3.5" />
                  </button>
                </div>
              </div>
            </div>
          </template>
        </VirtualTable>
        <p class="border-t border-slate-100 px-3 py-1.5 text-[11px] mts-muted dark:border-slate-800" data-testid="query-history-virtual-hint">
          {{ t('queryHistoryVirtualHint') }}
        </p>
      </div>
      <EmptyState
        v-else
        compact
        data-testid="query-history-empty"
        :title="historyFilter.trim() ? t('queryHistoryFilterEmpty') : t('queryHistoryEmpty')"
        :description="historyFilter.trim() ? t('queryHistoryFilterEmptyDesc') : t('queryHistoryEmptyDesc')"
      />
    </div>

    <div id="query-form" class="scroll-mt-20 grid grid-cols-1 gap-3 rounded-xl border border-slate-200 bg-white p-3 dark:border-slate-700 dark:bg-slate-900 sm:p-4 md:grid-cols-2 lg:grid-cols-3">
      <label class="text-xs text-slate-500 dark:text-slate-400 dark:text-slate-500">{{ t('database') }}
        <input v-model="queryForm.database" list="db-list" data-testid="query-database" class="mt-1 w-full rounded border px-2 py-1.5 text-sm dark:border-slate-600 dark:bg-slate-800" :placeholder="t('queryPlaceholderManual')" />
        <datalist id="db-list"><option v-for="db in databases" :key="db" :value="db" /></datalist>
      </label>
      <label class="text-xs text-slate-500 dark:text-slate-400 dark:text-slate-500">{{ t('measurement') }}
        <input v-model="queryForm.measurement" list="meas-list" data-testid="query-measurement" class="mt-1 w-full rounded border px-2 py-1.5 text-sm dark:border-slate-600 dark:bg-slate-800" :disabled="measurementsLoading" />
        <datalist id="meas-list"><option v-for="m in measurements" :key="m" :value="m" /></datalist>
      </label>
      <label class="text-xs text-slate-500 dark:text-slate-400 dark:text-slate-500">{{ t('retentionPolicy') }}
        <input v-model="queryForm.retention_policy" list="rp-list" class="mt-1 w-full rounded border px-2 py-1.5 text-sm dark:border-slate-600 dark:bg-slate-800" />
        <datalist id="rp-list">
          <option v-for="rp in (retentionPolicies.length ? retentionPolicies : ['autogen'])" :key="rp" :value="rp" />
        </datalist>
        <p v-if="metaSource === 'manual' || metaSource === 'partial'" class="mt-1 text-[11px] text-amber-700 dark:text-amber-200" data-testid="query-rp-meta-hint">{{ t('writeRpManualHint') }}</p>
      </label>
      <label class="text-xs text-slate-500 dark:text-slate-400 dark:text-slate-500">{{ t('queryStartMs') }}
        <div class="mt-1 flex gap-1">
          <input v-model="queryForm.start_time" data-testid="query-start-time" class="w-full rounded border px-2 py-1.5 text-sm dark:border-slate-600 dark:bg-slate-800" />
          <button class="rounded border px-2 text-xs" @click="fillNowMs('start')">{{ t('queryNow') }}</button>
        </div>
      </label>
      <label class="text-xs text-slate-500 dark:text-slate-400 dark:text-slate-500">{{ t('queryEndMs') }}
        <div class="mt-1 flex gap-1">
          <input v-model="queryForm.end_time" data-testid="query-end-time" class="w-full rounded border px-2 py-1.5 text-sm dark:border-slate-600 dark:bg-slate-800" />
          <button class="rounded border px-2 text-xs" @click="fillNowMs('end')">{{ t('queryNow') }}</button>
        </div>
      </label>
      <label class="text-xs text-slate-500 dark:text-slate-400 dark:text-slate-500">{{ t('fields') }}
        <input
          v-model="queryForm.fields"
          list="query-field-list"
          data-testid="query-fields"
          :placeholder="t('queryPhFields')"
          class="mt-1 w-full rounded border px-2 py-1.5 text-sm dark:border-slate-600 dark:bg-slate-800"
        />
        <datalist id="query-field-list">
          <option v-for="f in fieldOptions" :key="f" :value="f" />
        </datalist>
        <p v-if="fieldOptions.length" class="mt-1 text-[11px] mts-muted" data-testid="query-fields-meta-hint">
          {{ formatMessage(t('queryFieldsMetaHint'), { count: fieldOptions.length }) }}
        </p>
      </label>
      <label class="text-xs text-slate-500 dark:text-slate-400 dark:text-slate-500">{{ t('queryTagsExpr') }}
        <input v-model="queryForm.tags" data-testid="query-tags" :placeholder="t('queryPhTags')" class="mt-1 w-full rounded border px-2 py-1.5 text-sm dark:border-slate-600 dark:bg-slate-800" />
      </label>
      <div class="text-xs text-slate-500 dark:text-slate-400 md:col-span-2 lg:col-span-3" data-testid="query-series-meta">
        <div class="flex flex-wrap items-center justify-between gap-2">
          <span>{{ t('querySeriesPicker') }}</span>
          <span v-if="seriesLoading" class="text-[11px] mts-muted">{{ t('loading') }}</span>
          <span v-else-if="seriesTotal" class="text-[11px] mts-muted" data-testid="query-series-count">
            {{ formatMessage(t('querySeriesCountMeta'), { shown: filteredSeriesOptions.length, total: seriesTotal }) }}
          </span>
        </div>
        <input
          v-model="seriesFilter"
          class="mt-1 w-full rounded border px-2 py-1.5 text-xs dark:border-slate-600 dark:bg-slate-800"
          data-testid="query-series-filter"
          :placeholder="t('querySeriesFilterPh')"
          :disabled="seriesLoading || !seriesOptions.length"
        />
        <button
          type="button"
          class="mts-btn mt-1 text-xs"
          data-testid="query-series-refresh"
          :disabled="seriesLoading || !queryForm.measurement"
          @click="refreshSeriesWithTags"
        >{{ t('querySeriesRefreshByTags') }}</button>

        <select
          class="mt-1 w-full rounded border px-2 py-1.5 font-mono text-xs dark:border-slate-600 dark:bg-slate-800"
          data-testid="query-series-select"
          :disabled="seriesLoading || !filteredSeriesOptions.length"
          @change="onSeriesSelectFiltered(($event.target as HTMLSelectElement).value)"
        >
          <option value="">{{ filteredSeriesOptions.length ? t('querySeriesPickPlaceholder') : (seriesOptions.length ? t('querySeriesFilterEmpty') : t('querySeriesEmpty')) }}</option>
          <option v-for="(s, idx) in filteredSeriesOptions" :key="s.id ?? idx" :value="String(idx)">
            {{ seriesOptionLabel(s) }}
          </option>
        </select>
        <p v-if="seriesTruncated" class="mt-1 text-[11px] text-amber-700 dark:text-amber-200" data-testid="query-series-truncated">
          {{ formatMessage(t('querySeriesTruncated'), { max: 200, total: seriesTotal }) }}
        </p>
        <p v-if="seriesError" class="mt-1 text-[11px] text-rose-600" data-testid="query-series-error">{{ seriesError }}</p>
        <p class="mt-1 text-[11px] mts-muted">{{ t('querySeriesPickerHint') }}</p>
      </div>
      <label class="text-xs text-slate-500 dark:text-slate-400 dark:text-slate-500 md:col-span-2 lg:col-span-3">{{ t('queryPredicates') }}
        <textarea
          v-model="queryForm.predicates"
          data-testid="query-predicates"
          rows="2"
          class="mt-1 w-full rounded border px-2 py-1.5 font-mono text-xs dark:border-slate-600 dark:bg-slate-800"
          :placeholder="t('queryPhPredicates')"
        />
        <span class="mt-1 block text-[11px] mts-muted">{{ t('queryPredicatesHint') }}</span>
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

    <div id="query-actions" class="scroll-mt-20 flex flex-wrap gap-2">
      <button
        type="button"
        class="inline-flex items-center gap-1 rounded-lg bg-slate-800 px-4 py-2 text-sm text-white disabled:opacity-50 dark:bg-slate-100 dark:bg-slate-800 dark:text-slate-900"
        data-testid="query-run"
        :disabled="loading"
        @click="runQuery"
      >
        <Search class="h-4 w-4" /> {{ loading ? t('loading') : t('query') }}
      </button>
      <button
        type="button"
        class="inline-flex items-center gap-1 rounded-lg border px-3 py-2 text-sm dark:border-slate-700"
        data-testid="query-cancel"
        :disabled="!loading"
        @click="cancelQuery"
      ><Square class="h-4 w-4" />{{ t('queryCancel') }}</button>
      <button
        type="button"
        class="inline-flex items-center gap-1 rounded-lg border px-3 py-2 text-sm dark:border-slate-700"
        data-testid="query-engine-stats"
        :disabled="engineStatsLoading || loading"
        @click="loadEngineStats"
      >{{ engineStatsLoading ? t('loading') : t('queryEngineStatsBtn') }}</button>
      <button
        type="button"
        class="inline-flex items-center gap-1 rounded-lg border border-red-200 px-3 py-2 text-sm text-red-700 disabled:cursor-not-allowed disabled:opacity-50 dark:text-red-200"
        data-testid="query-range-delete"
        :disabled="writeBlocked"
        :title="writeBlocked ? t(blockedMessageKey('offlineDeleteBlocked')) : undefined"
        @click="openRangeDelete"
      ><Trash2 class="h-4 w-4" />{{ t('queryRangeDelete') }}</button>
      <button class="inline-flex items-center gap-1 rounded-lg border px-3 py-2 text-sm dark:border-slate-700" @click="copyResults">
        <component :is="copyState === 'ok' ? Check : Copy" class="h-4 w-4" />
        {{ streamMeta.previewOnly ? t('copyPreview') : t('copy') }}
      </button>
      <button type="button" class="mts-btn" data-testid="query-share-link" @click="copyShareLink">
        {{ t('queryShareLink') }}
      </button>
      <button class="mts-btn" data-testid="query-export-csv" :disabled="!rows.length || exportBusy" @click="exportCSV">
          <Download class="h-3.5 w-3.5" /> CSV
        </button>
    </div>

    <ExportJobBanner
      :job="exportJob"
      @cancel="cancelExport"
      @dismiss="resetExport"
    />
    <p v-if="actionError" class="mts-alert-error">{{ actionError }}</p>
    <p v-if="deleteResult" class="mts-alert-ok">{{ deleteResult }}</p>

    <div id="query-stats" class="scroll-mt-20 space-y-2" data-testid="query-stats-panel">
      <div class="flex flex-wrap items-center justify-between gap-2">
        <div class="flex flex-wrap items-center gap-2 text-xs">
          <span class="font-semibold text-slate-700 dark:text-slate-200">{{ t('queryStatsTitle') }}</span>
          <span
            v-if="engineStatsSource === 'engine'"
            class="rounded-full bg-slate-100 px-2 py-0.5 text-[11px] text-slate-600 dark:bg-slate-800 dark:text-slate-300"
            data-testid="query-stats-source-engine"
          >{{ t('queryStatsSourceEngine') }}</span>
          <span
            v-else-if="engineStatsSource === 'query' && queryStats"
            class="rounded-full bg-emerald-50 px-2 py-0.5 text-[11px] text-emerald-700 dark:bg-emerald-950/40 dark:text-emerald-200"
            data-testid="query-stats-source-query"
          >{{ t('queryStatsSourceQuery') }}</span>
          <span v-if="engineStatsSource === 'engine' && engineStatsAtLabel" class="text-[11px] mts-muted" data-testid="query-stats-at">
            {{ engineStatsAtLabel }}
          </span>
        </div>
        <button
          type="button"
          class="mts-btn text-xs"
          data-testid="query-engine-stats-inline"
          :disabled="engineStatsLoading || loading"
          @click="loadEngineStats"
        >{{ engineStatsLoading ? t('loading') : t('queryEngineStatsBtn') }}</button>
      </div>
      <p v-if="engineStatsError" class="mts-alert-error text-xs" data-testid="query-engine-stats-error">{{ engineStatsError }}</p>
      <template v-if="queryStats">
        <div class="grid grid-cols-2 gap-2 sm:grid-cols-5" data-testid="query-stats-primary">
          <div
            v-for="card in primaryStatsCards"
            :key="card.key"
            class="rounded-xl border bg-white p-3 dark:border-slate-700 dark:bg-slate-900"
          >
            <p class="text-[11px] text-slate-400 dark:text-slate-500">{{ statLabel(card.labelKey) }}</p>
            <p class="text-lg font-semibold" :class="toneClass(card.tone)">{{ card.value }}</p>
          </div>
        </div>
        <details class="mts-card p-3" data-testid="query-stats-details">
          <summary class="cursor-pointer text-xs font-medium text-slate-600 dark:text-slate-300">{{ t('queryStatsDetails') }}</summary>
          <div class="mt-2 grid grid-cols-2 gap-2 sm:grid-cols-3 lg:grid-cols-4">
            <div v-for="card in detailStatsCards" :key="card.key" class="rounded-lg border border-slate-100 px-2 py-1.5 dark:border-slate-800">
              <p class="text-[10px] mts-muted">{{ statLabel(card.labelKey) }}</p>
              <p class="font-mono text-sm font-semibold" :class="toneClass(card.tone)">{{ card.value }}</p>
            </div>
          </div>
        </details>
        <div v-if="latency" class="mts-card px-3 py-2">
          <div class="mb-1 flex flex-wrap items-center justify-between gap-2 text-xs">
            <span class="text-slate-600 dark:text-slate-300">{{ t('queryLatencyWaterline') }} · <span class="font-medium">{{ latency.label }}</span></span>
            <span class="font-mono text-slate-500 dark:text-slate-400">{{ latency.durationMs.toFixed(1) }} ms · {{ t('queryLatencyLegend') }}</span>
          </div>
          <div class="h-2 overflow-hidden rounded-full bg-slate-100 dark:bg-slate-800">
            <div class="h-full rounded-full transition-all" :class="latency.toneClass" :style="{ width: latency.barPercent + '%' }" />
          </div>
        </div>
      </template>
      <EmptyState
        v-else
        compact
        data-testid="query-stats-empty"
        :title="t('queryStatsEmptyTitle')"
        :description="t('queryStatsEmptyDesc')"
      />
    </div>

    <div id="query-chart" class="scroll-mt-20">
      <QueryChart v-if="showChart && rows.length" :rows="rows" />
    </div>

    <div
      v-if="queryAttempted && !loading && !actionError && !rows.length && !columnRows.length && !rawOutput"
      class="mts-card"
    >
      <EmptyState
        :title="t('queryNoMatchTitle')"
        :description="t('queryNoMatchDesc')"
      />
    </div>

    <div id="query-results" v-if="rows.length" class="scroll-mt-20 overflow-hidden rounded-2xl border bg-white dark:border-slate-700 dark:bg-slate-900" data-testid="query-results">
      <div class="flex flex-wrap items-center justify-between gap-2 border-b px-4 py-2 text-sm dark:border-slate-800">
        <span class="font-semibold">{{ t('queryRowResult') }}</span>
        <div class="flex flex-wrap items-center gap-2">
          <span class="text-xs text-slate-500 dark:text-slate-400" data-testid="query-row-count">{{ formatMessage(t('queryRowCountVirtual'), { count: rows.length }) }}</span>
          <div class="relative">
            <button
              type="button"
              class="mts-btn"
              data-testid="query-column-picker"
              :title="t('queryColVisibility')"
              @click="showColumnPicker = !showColumnPicker"
            >
              <Columns3 class="h-3.5 w-3.5" /> {{ t('queryColumns') }}
            </button>
            <div
              v-if="showColumnPicker"
              class="absolute right-0 z-20 mt-1 w-44 rounded-lg border border-slate-200 bg-white p-2 shadow-lg dark:border-slate-700 dark:bg-slate-900"
              data-testid="query-column-picker-menu"
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
          data-testid="query-results-header"
        >
          <span v-if="resultColumns.time">{{ t('queryColTime') }}</span>
          <span v-if="resultColumns.measurement">{{ t('queryColMeasurement') }}</span>
          <span v-if="resultColumns.tags">{{ t('queryColTags') }}</span>
          <span v-if="resultColumns.fields">{{ t('queryColFields') }}</span>
        </div>
        <VirtualTable
          :items="rows"
          :row-height="QUERY_ROW_HEIGHT"
          :height="QUERY_LIST_HEIGHT"
          data-testid="query-results-virtual-list"
        >
          <template #default="{ item: row, index }">
            <div
              class="grid min-w-[480px] items-center border-b px-4 text-xs dark:border-slate-800"
              :class="resultGridClass"
              :data-testid="`query-result-row-${index}`"
            >
              <span v-if="resultColumns.time" class="truncate font-mono" :title="formatTimestamp(row.timestamp)">{{ formatTimestamp(row.timestamp) }}</span>
              <span v-if="resultColumns.measurement" class="truncate" :title="row.measurement">{{ row.measurement }}</span>
              <span v-if="resultColumns.tags" class="truncate font-mono text-slate-500 dark:text-slate-400" :title="row.tags && Object.keys(row.tags).length ? JSON.stringify(row.tags) : ''">{{ row.tags && Object.keys(row.tags).length ? JSON.stringify(row.tags) : t('emptyValue') }}</span>
              <span v-if="resultColumns.fields" class="truncate font-mono" :title="showRawFields ? JSON.stringify(row.fields) : formatFieldsMap(row.fields as any)">{{ showRawFields ? JSON.stringify(row.fields) : formatFieldsMap(row.fields as any) }}</span>
            </div>
          </template>
        </VirtualTable>
        <p class="border-t border-slate-100 px-3 py-1.5 text-[11px] mts-muted dark:border-slate-800" data-testid="query-results-virtual-hint">
          {{ t('queryResultsVirtualHint') }}
        </p>
      </div>
    </div>

    <div
      v-if="columnRows.length"
      class="overflow-hidden rounded-2xl border bg-white dark:border-slate-700 dark:bg-slate-900"
      data-testid="query-columns-summary"
    >
      <div class="flex justify-between border-b px-4 py-2 text-sm dark:border-slate-800">
        <span class="font-semibold">{{ t('queryColumnSummary') }}</span>
        <span class="text-xs text-slate-500 dark:text-slate-400" data-testid="query-columns-count">{{ formatMessage(t('querySeriesCount'), { count: columnRows.length }) }}</span>
      </div>
      <div
        class="grid grid-cols-[minmax(7rem,1fr)_minmax(6rem,0.8fr)_minmax(8rem,1.2fr)_minmax(5rem,0.6fr)] border-b px-4 py-2 text-left text-[11px] uppercase text-slate-500 dark:border-slate-800 dark:text-slate-400"
        data-testid="query-columns-header"
      >
        <span>{{ t('queryColMeasurement') }}</span>
        <span>{{ t('field') }}</span>
        <span>{{ t('queryColTags') }}</span>
        <span>{{ t('queryColPoints') }}</span>
      </div>
      <VirtualTable
        :items="columnRows"
        :row-height="COLUMN_ROW_HEIGHT"
        :height="Math.min(COLUMN_LIST_HEIGHT, Math.max(160, columnRows.length * COLUMN_ROW_HEIGHT))"
        data-testid="query-columns-virtual-list"
      >
        <template #default="{ item: c, index }">
          <div
            class="grid h-full grid-cols-[minmax(7rem,1fr)_minmax(6rem,0.8fr)_minmax(8rem,1.2fr)_minmax(5rem,0.6fr)] items-center border-b px-4 text-xs dark:border-slate-800"
            :data-testid="`query-column-row-${index}`"
          >
            <span class="truncate" :title="c.measurement">{{ c.measurement }}</span>
            <span class="truncate" :title="c.field">{{ c.field }}</span>
            <span class="truncate font-mono text-slate-500 dark:text-slate-400" :title="c.tags">{{ c.tags }}</span>
            <span>{{ c.points }}</span>
          </div>
        </template>
      </VirtualTable>
      <p class="border-t border-slate-100 px-3 py-1.5 text-[11px] mts-muted dark:border-slate-800" data-testid="query-columns-virtual-hint">
        {{ t('queryColumnsVirtualHint') }}
      </p>
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
      :write-blocked="writeBlocked"
      :block-reason="blockReason"
      :offline-message-key="'offlineDeleteBlocked'"
      :title="t('queryDeleteTitle')"
      :message="`${t('queryDeleteMsg')}\n\n${deleteScopeMessage}`"
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
