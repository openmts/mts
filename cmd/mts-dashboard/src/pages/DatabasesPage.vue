<script setup lang="ts">
import { ref, onMounted, computed, inject, watch, onBeforeUnmount, type ComputedRef } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useHashScroll } from '@/composables/useHashScroll'
import { apiGet, apiPost, apiDelete } from '@/api/client'
import { useAdminOpBusy } from '@/composables/useAdminOpBusy'
import { parseAdminOpStatusPayload } from '@/utils/adminOpBusy'
import { useMutationGuard } from '@/composables/useMutationGuard'
import {
  anyRetentionPolicyDraftDirty,
  isDatabaseCreateDraftDirty,
} from '@/utils/adminFormDirty'
import { registerDirtyChecker } from '@/utils/routeDirty'
import { listDatabasesDetailed, listMeasurementsDetailed, listFieldsDetailed, listRetentionPoliciesDetailed, listSeriesDetailed } from '@/api/meta'
import { seriesLabel } from '@/utils/seriesMeta'
import { filterSeriesListLocal } from '@/utils/seriesFilter'
import { formatMessage } from '@/utils/formatMessage'
import {
  Plus, Trash2, ChevronDown, ChevronRight, Table2, Tag, Clock, Download,
} from 'lucide-vue-next'
import { useAuth } from '@/composables/useAuth'
import ConfirmDialog from '@/components/ConfirmDialog.vue'
import AdminOpLastChip from '@/components/AdminOpLastChip.vue'
import InFlightBanner from '@/components/InFlightBanner.vue'
import ActionResultBanner from '@/components/ActionResultBanner.vue'
import PartialErrorBanner from '@/components/PartialErrorBanner.vue'
import EmptyState from '@/components/EmptyState.vue'
import ListSelectionToolbar from '@/components/ListSelectionToolbar.vue'
import VirtualTable from '@/components/VirtualTable.vue'
import { makeActionResult } from '@/utils/actionResult'
import { useActionRetry } from '@/composables/useActionRetry'
import { useNotify } from '@/composables/useNotify'
import { useNotifyAdminBusy } from '@/composables/useNotifyAdminBusy'
import { actionResultAdminBusyAction } from '@/utils/adminOpBusy'
import { formatCaughtError, isCanceledError, isTimeoutError } from '@/utils/apiError'
import { createActionAbort } from '@/utils/actionAbort'
import { formatRPDuration, mapRPDurationError, parseRPDurationToNs } from '@/utils/rpDuration'
import { filterByName } from '@/utils/listFilter'
import { filterRowsByIds } from '@/utils/listSelection'
import { useListSelection } from '@/composables/useListSelection'
import {
  cycleSortState,
  loadSortState,
  saveSortState,
  sortByAccessor,
  type SortState,
  ariaSortValue,
} from '@/utils/listSort'
import { useI18n } from '@/composables/useI18n'
import type { MessageKey } from '@/i18n/messages'
import { buildDatabasesExport, databasesToCSV } from '@/utils/databasesExport'
import {
  buildQueryPrefillPath,
  buildWritePrefillPath,
  parseDatabasesPrefill,
  databasesFormToPrefill,
} from '@/utils/routePrefill'
import { copyText } from '@/utils/clipboard'
import { stampFilename } from '@/utils/download'
import { useExportJob } from '@/composables/useExportJob'
import ExportJobBanner from '@/components/ExportJobBanner.vue'
interface FieldSchema { measurement: string; name: string; type: number }
interface FieldsResponse {
  fields: FieldSchema[]
  admin_op_busy?: boolean
  op?: string
  started_at_unix?: number
  last?: unknown
}
interface Series { id: number; measurement: string; tags: Record<string, string> }
interface MeasurementEntry {
  name: string
  expanded: boolean
  loading: boolean
  loadError: string
  fields: FieldSchema[]
  series: Series[]
  seriesTotal: number
  seriesTruncated: boolean
  seriesOffset: number
  seriesHasMore: boolean
  seriesLoadingMore: boolean
  seriesQuery: string
  seriesLocalFilter: string
  seriesPath: string
}
interface DatabaseEntry {
  name: string
  expanded: boolean
  loading: boolean
  loaded: boolean
  detailError: string
  measurements: MeasurementEntry[]
  measurementsPath: string
  retentionPolicies: { name: string; duration: number }[]
  retentionPath: string
  newRpName: string
  newRpDuration: string
}
const { isAdmin } = useAuth()
const adminOpBusySummary = inject<ComputedRef<{ lastSummary?: string; lastError?: string; lastOk?: boolean | null }> | undefined>('adminOpBusySummary', undefined)
const databasesAdminLastLabel = computed(() => (adminOpBusySummary?.value?.lastSummary || '').trim())
const databasesAdminLastErrorDetail = computed(() => {
  if (adminOpBusySummary?.value?.lastOk !== false) return ''
  return (adminOpBusySummary?.value?.lastError || '').trim()
})
const { offline, writeBlocked, blockReason, blockedMessageKey } = useMutationGuard()
const route = useRoute()
const router = useRouter()
useHashScroll()
const SERIES_CAP = 200
const { t } = useI18n()
const { applyAdminOpStatus } = useAdminOpBusy()
const { success, info, error: notifyError, warn } = useNotify()
const { notifyMaybeAdminBusy } = useNotifyAdminBusy()

const {
  exportJob,
  exportBusy,
  cancelExport,
  resetExport,
  retryLastExport,
  canRetryExport,
  runTextExport,
  runJSONExport,
} = useExportJob()
const databases = ref<DatabaseEntry[]>([])
const dbFilter = ref('')
const measFilter = ref('')
const DB_SORT_KEY = 'mts.dashboard.databases-sort.prefs.v1'
const DB_SORT_KEYS = ['name'] as const
type DbSortKey = (typeof DB_SORT_KEYS)[number]
const dbStorage = typeof localStorage !== 'undefined' ? localStorage : null
const dbSort = ref<SortState<DbSortKey>>(loadSortState(dbStorage, DB_SORT_KEY, DB_SORT_KEYS))

const filteredDatabases = computed(() => {
  const base = filterByName(databases.value, dbFilter.value)
  return sortByAccessor(base, dbSort.value, { name: (d) => d.name })
})
const visibleDbIds = computed(() => filteredDatabases.value.map((d) => d.name))
const DB_ROW_HEIGHT = 52
const DB_LIST_HEIGHT = 416
const activeDatabase = computed(() => databases.value.find((d) => d.expanded) ?? null)
const filteredMeasurements = computed(() => {
  const db = activeDatabase.value
  if (!db) return []
  return filterByName(db.measurements, measFilter.value)
})

function cycleDbSort() {
  dbSort.value = cycleSortState(dbSort.value, 'name')
  saveSortState(dbStorage, DB_SORT_KEY, dbSort.value)
}

function dbSortIndicator(): string {
  if (dbSort.value.key !== 'name') return ''
  return dbSort.value.dir === 'asc' ? '↑' : '↓'
}
const {
  selectedIds,
  selectedCount,
  allVisibleSelected,
  someVisibleSelected,
  exportIds,
  isSelected,
  toggle,
  toggleAllVisible,
  clear: clearSelection,
  pruneTo,
} = useListSelection(visibleDbIds)
const newDbName = ref('')
const loadError = ref('')
type DbActionKey = 'create-db' | 'delete-db' | 'create-rp' | 'load-detail' | 'load-meas'
const {
  lastFailedAction,
  actionResult,
  actionContext,
  canRetryAction,
  clearActionResult,
  setActionOk,
  setActionError,
  setActionResult,
  reportActionError: reportRetryError,
} = useActionRetry<DbActionKey>()
const databasesAdminBusyAction = computed(() =>
  actionResultAdminBusyAction({
    message: actionResult.value?.message || '',
    openLabel: t.value('adminOpBusyOpenOps'),
  }),
)
const lastFailedDbName = ref('')
const lastFailedMeasName = ref('')
const confirmOpen = ref(false)
const confirmDbName = ref('')
const confirmLoading = ref(false)
const createDbLoading = ref(false)
const rpLoading = ref(false)
const dbActionStartedAt = ref<number | null>(null)
const dbActionAbort = createActionAbort()

async function loadDatabasesList() {
  try {
    const listed = await listDatabasesDetailed()
    if (listed.adminOp) applyAdminOpStatus(parseAdminOpStatusPayload(listed.adminOp))
    const names = listed.names
    if (listed.error && !names.length) throw new Error(listed.error)
    // 刷新时尽量保留已展开/已加载详情，避免列表闪空后丢失上下文
    const prev = new Map(databases.value.map((d) => [d.name, d]))
    databases.value = names.map((name) => {
      const oldDb = prev.get(name)
      if (oldDb) return oldDb
      return {
        name,
        expanded: false,
        loading: false,
        loaded: false,
        detailError: '',
        measurements: [],
        measurementsPath: '',
        retentionPolicies: [],
        retentionPath: '',
        newRpName: '',
        newRpDuration: '',
      }
    })
    pruneTo(names)
    loadError.value = ''
  } catch (e) {
    const msg = formatCaughtError(e)
    if (databases.value.length) loadError.value = msg
    else {
      databases.value = []
      loadError.value = msg
    }
  }
}

onMounted(async () => {
  unregisterDatabasesDirty = registerDirtyChecker('databases', () => databasesFormDirty.value)
  window.addEventListener('beforeunload', onDatabasesBeforeUnload)
  await loadDatabasesList()
  if (!loadError.value) await applyDatabasesPrefillFromRoute()
})

watch(
  () => route.fullPath,
  (path, prev) => {
    if (prev != null && path !== prev) void applyDatabasesPrefillFromRoute()
  },
)

async function applyDatabasesPrefillFromRoute() {
  const pre = parseDatabasesPrefill(route.query as Record<string, unknown>)
  let changed = false
  if (pre.q != null && dbFilter.value !== pre.q) {
    dbFilter.value = pre.q
    changed = true
  }
  if (pre.database) {
    const db = databases.value.find((d) => d.name === pre.database)
    if (db && !db.expanded) {
      await toggleExpand(db)
      changed = true
    }
  }
  if (changed) success(t.value('databasesPrefillApplied'))
}

async function copyDatabasesShareLink() {
  const path = databasesFormToPrefill({
    database: activeDatabase.value?.name,
    q: dbFilter.value,
  }, { hash: activeDatabase.value ? '#databases-detail' : '#databases-filter-bar' })
  const url = `${window.location.origin}${path}`
  const res = await copyText(url)
  if (res.ok) success(t.value('databasesShareCopied'))
  else notifyError(res.error || t.value('failed'))
}
async function loadDatabaseDetails(db: DatabaseEntry) {
  db.loading = true
  clearActionResult()
  try {
    const [meas, rps] = await Promise.all([
      listMeasurementsDetailed(db.name).then((r) => {
        if (r.adminOp) applyAdminOpStatus(parseAdminOpStatusPayload(r.adminOp))
        if (r.error && !r.names.length) throw new Error(r.error)
        return r
      }),
      listRetentionPoliciesDetailed(db.name).then((r) => {
        if (r.adminOp) applyAdminOpStatus(parseAdminOpStatusPayload(r.adminOp))
        if (r.error && !r.policies.length) throw new Error(r.error)
        return r
      }),
    ])
    db.measurementsPath = String(meas.path || '').trim()
    db.retentionPath = String(rps.path || '').trim()
    db.measurements = meas.names.map((m) => ({
      name: m,
      expanded: false,
      loading: false,
      loadError: '',
      fields: [],
      series: [],
      seriesTotal: 0,
      seriesTruncated: false,
      seriesOffset: 0,
      seriesHasMore: false,
      seriesLoadingMore: false,
      seriesQuery: '',
      seriesLocalFilter: '', seriesPath: '',
    }))
    db.retentionPolicies = rps.policies.map((p) => ({ name: p.name, duration: p.duration ?? 0 }))
    db.loaded = true
    db.detailError = ''
  } catch (e) {
    db.loaded = false
    db.detailError = formatCaughtError(e)
    reportActionError('load-detail', e, { db: db.name })
  } finally {
    db.loading = false
  }
}
async function toggleExpand(db: DatabaseEntry) {
  if (db.expanded) {
    db.expanded = false
    measFilter.value = ''
    return
  }
  measFilter.value = ''
  for (const item of databases.value) {
    if (item !== db) item.expanded = false
  }
  db.expanded = true
  if (!db.loaded) await loadDatabaseDetails(db)
}
function hasMeasurementSnapshot(meas: MeasurementEntry): boolean {
  return meas.fields.length > 0 || meas.series.length > 0 || meas.seriesTotal > 0
}

async function loadMeasurementDetails(meas: MeasurementEntry, dbName: string) {
  meas.loading = true
  try {
    const [fieldsData, seriesResult] = await Promise.all([
      listFieldsDetailed(dbName, meas.name).then((r) => {
        if (r.adminOp) applyAdminOpStatus(parseAdminOpStatusPayload(r.adminOp))
        if (r.error && !r.fields.length) throw new Error(r.error)
        return { fields: r.fields as FieldSchema[] }
      }),
      listSeriesDetailed(dbName, meas.name, { limit: SERIES_CAP, offset: 0, q: meas.seriesQuery || undefined }).then((r) => {
        if (r.adminOp) applyAdminOpStatus(parseAdminOpStatusPayload(r.adminOp))
        return r
      }),
    ])
    meas.fields = fieldsData.fields ?? []
    meas.series = (seriesResult.series as Series[]).map((s) => ({
      id: s.id ?? 0,
      measurement: s.measurement ?? meas.name,
      tags: s.tags ?? {},
    }))
    meas.seriesTotal = seriesResult.total
    meas.seriesTruncated = seriesResult.truncated
    meas.seriesOffset = seriesResult.series.length
    meas.seriesHasMore = seriesResult.series.length < seriesResult.total
    meas.seriesPath = String(seriesResult.path || '').trim()
    meas.loadError = ''
  } catch (e) {
    // 就地 soft-fail，避免与顶层 action banner 双重 toast
    meas.loadError = formatCaughtError(e)
    lastFailedDbName.value = dbName
    lastFailedMeasName.value = meas.name
  } finally {
    meas.loading = false
  }
}


async function loadMoreSeries(meas: MeasurementEntry, dbName: string) {
  if (!meas.seriesHasMore || meas.seriesLoadingMore) return
  meas.seriesLoadingMore = true
  try {
    const seriesResult = await listSeriesDetailed(dbName, meas.name, {
      limit: SERIES_CAP,
      offset: meas.seriesOffset,
      q: meas.seriesQuery || undefined,
    })
    if (seriesResult.adminOp) applyAdminOpStatus(parseAdminOpStatusPayload(seriesResult.adminOp))
    const mapped = (seriesResult.series as Series[]).map((s) => ({
      id: s.id ?? 0,
      measurement: s.measurement ?? meas.name,
      tags: s.tags ?? {},
    }))
    const seen = new Set(meas.series.map((s) => s.id))
    for (const s of mapped) {
      if (!seen.has(s.id)) {
        meas.series.push(s)
        seen.add(s.id)
      }
    }
    meas.seriesTotal = seriesResult.total
    meas.seriesOffset += seriesResult.series.length
    meas.seriesHasMore = meas.seriesOffset < seriesResult.total
    meas.seriesTruncated = meas.seriesHasMore
    meas.loadError = ''
  } catch (e) {
    meas.loadError = formatCaughtError(e)
  } finally {
    meas.seriesLoadingMore = false
  }
}


function filteredSeriesOf(meas: MeasurementEntry): Series[] {
  return filterSeriesListLocal(meas.series, meas.seriesLocalFilter)
}

async function applySeriesServerFilter(meas: MeasurementEntry, dbName: string) {
  meas.seriesQuery = (meas.seriesLocalFilter || '').trim()
  meas.seriesOffset = 0
  meas.seriesHasMore = false
  await loadMeasurementDetails(meas, dbName)
}

async function toggleMeasurement(meas: MeasurementEntry, dbName: string) {
  meas.expanded = !meas.expanded
  if (!meas.expanded) return
  if (hasMeasurementSnapshot(meas) && !meas.loadError) return
  await loadMeasurementDetails(meas, dbName)
}

const databasesFormDirty = computed(() => {
  if (isDatabaseCreateDraftDirty({ name: newDbName.value })) return true
  return anyRetentionPolicyDraftDirty(
    databases.value.map((db) => ({ name: db.newRpName, duration: db.newRpDuration })),
  )
})

function onDatabasesBeforeUnload(e: BeforeUnloadEvent) {
  if (!databasesFormDirty.value) return
  e.preventDefault()
  e.returnValue = ''
}

let unregisterDatabasesDirty: (() => void) | null = null


function reportActionError(key: DbActionKey, e: unknown, ctx?: { db?: string; meas?: string }) {
  if (ctx?.db) lastFailedDbName.value = ctx.db
  if (ctx?.meas) lastFailedMeasName.value = ctx.meas
  reportRetryError(key, e, {
    ...(ctx?.db ? { db: ctx.db } : {}),
    ...(ctx?.meas ? { meas: ctx.meas } : {}),
  })
  const msg = actionResult.value?.message || formatCaughtError(e)
  notifyMaybeAdminBusy(msg, e)
}

async function retryLastDatabaseAction() {
  const key = lastFailedAction.value as DbActionKey | null
  if (!key) return
  if (key === 'create-db') return createDatabase()
  if (key === 'delete-db' && confirmDbName.value) {
    confirmOpen.value = true
    return confirmDeleteDatabase()
  }
  if (key === 'create-rp' && lastFailedDbName.value) {
    const db = databases.value.find((d) => d.name === lastFailedDbName.value)
    if (db) return createRetentionPolicy(db)
  }
  if (key === 'load-detail' && lastFailedDbName.value) {
    const db = databases.value.find((d) => d.name === lastFailedDbName.value)
    if (db) return loadDatabaseDetails(db)
  }
  if (key === 'load-meas' && lastFailedDbName.value && lastFailedMeasName.value) {
    const db = databases.value.find((d) => d.name === lastFailedDbName.value)
    const meas = db?.measurements.find((m) => m.name === lastFailedMeasName.value)
    if (db && meas) {
      meas.expanded = true
      return loadMeasurementDetails(meas, db.name)
    }
  }
}


function cancelDbAction() {
  dbActionAbort.cancel()
}

function reportDbCatch(key: DbActionKey, e: unknown, ctx?: Record<string, string>) {
  if (isCanceledError(e)) {
    const msg = t.value('adminActionCancelled')
    setActionResult(makeActionResult('info', msg))
    info(msg)
    return
  }
  if (isTimeoutError(e)) {
    const msg = t.value('adminActionTimedOut')
    setActionResult(makeActionResult('error', msg))
    notifyError(msg)
    return
  }
  reportActionError(key, e, ctx)
}

async function createDatabase() {
  if (createDbLoading.value) return
  if (writeBlocked.value) {
    const msg = t.value(blockedMessageKey('offlineAdminBlocked'))
    setActionError(msg)
    notifyError(msg)
    return
  }
  if (!isAdmin.value) return
  if (!newDbName.value.trim()) return
  clearActionResult()
  createDbLoading.value = true
  dbActionStartedAt.value = Date.now()
  const signal = dbActionAbort.begin()
  try {
    const name = newDbName.value.trim()
    const created = await apiPost<{ admin_op_busy?: boolean; op?: string; started_at_unix?: number; last?: unknown }>('/api/v1/admin/databases', { name }, { signal })
    applyAdminOpStatus(parseAdminOpStatusPayload(created))
    databases.value.push({
      name,
      expanded: false,
      loading: false,
      loaded: false,
      detailError: '',
      measurements: [],
      measurementsPath: '',
      retentionPolicies: [],
      retentionPath: '',
      newRpName: '',
      newRpDuration: '',
    })
    newDbName.value = ''
    setActionOk(t.value('databasesCreated'))
    success(t.value('databasesCreated'))
  } catch (e) {
    reportDbCatch('create-db', e)
  } finally {
    dbActionAbort.end()
    createDbLoading.value = false
    dbActionStartedAt.value = null
  }
}
function requestDeleteDatabase(name: string) {
  if (!isAdmin.value) return
  if (writeBlocked.value) {
    notifyError(t.value(blockedMessageKey('offlineAdminBlocked')))
    return
  }
  confirmDbName.value = name
  confirmOpen.value = true
}
async function confirmDeleteDatabase() {
  if (confirmLoading.value) return
  if (writeBlocked.value) {
    const msg = t.value(blockedMessageKey('offlineAdminBlocked'))
    setActionError(msg)
    notifyError(msg)
    return
  }
  const name = confirmDbName.value
  if (!name) return
  confirmLoading.value = true
  clearActionResult()
  dbActionStartedAt.value = Date.now()
  const signal = dbActionAbort.begin()
  try {
    const del = await apiDelete<{ admin_op_busy?: boolean; op?: string; started_at_unix?: number; last?: unknown }>(`/api/v1/admin/databases/${encodeURIComponent(name)}`, { signal })
    applyAdminOpStatus(parseAdminOpStatusPayload(del))
    databases.value = databases.value.filter((d) => d.name !== name)
    pruneTo(databases.value.map((d) => d.name))
    confirmOpen.value = false
    lastFailedAction.value = null
    const okMsg = formatMessage(t.value('databasesDeleted'), { name })
    setActionOk(okMsg)
    success(okMsg)
  } catch (e) {
    reportDbCatch('delete-db', e, { db: name })
  } finally {
    dbActionAbort.end()
    confirmLoading.value = false
    dbActionStartedAt.value = null
  }
}
async function createRetentionPolicy(db: DatabaseEntry) {
  if (rpLoading.value) return
  if (writeBlocked.value) {
    const msg = t.value(blockedMessageKey('offlineAdminBlocked'))
    setActionError(msg)
    notifyError(msg)
    return
  }
  if (!isAdmin.value) return
  const name = db.newRpName.trim()
  if (!name) return
  const dur = db.newRpDuration.trim()
  if (!dur) return
  let durationNs: number
  try {
    durationNs = parseDuration(dur)
  } catch {
    const msg = t.value('databasesInvalidDuration')
    setActionError(msg)
    notifyError(msg)
    return
  }
  clearActionResult()
  lastFailedDbName.value = db.name
  rpLoading.value = true
  dbActionStartedAt.value = Date.now()
  const signal = dbActionAbort.begin()
  try {
    const rp = await apiPost<{ admin_op_busy?: boolean; op?: string; started_at_unix?: number; last?: unknown }>(`/api/v1/admin/databases/${encodeURIComponent(db.name)}/retention-policies`, {
      policy: { name, duration: durationNs },
    }, { signal })
    applyAdminOpStatus(parseAdminOpStatusPayload(rp))
    db.retentionPolicies.push({ name, duration: durationNs })
    db.newRpName = ''
    db.newRpDuration = ''
    setActionOk(t.value('databasesRpCreated'))
    success(t.value('databasesRpCreated'))
  } catch (e) {
    reportDbCatch('create-rp', e, { db: db.name })
  } finally {
    dbActionAbort.end()
    rpLoading.value = false
    dbActionStartedAt.value = null
  }
}
function parseDuration(s: string): number {
  try {
    return parseRPDurationToNs(s)
  } catch (e) {
    throw new Error(mapRPDurationError(e, (k) => t.value(k as MessageKey)))
  }
}
function formatDuration(ns: number): string {
  return formatRPDuration(ns)
}

function openQueryFor(db: string, measurement?: string, tags?: Record<string, string> | string) {
  let tagsExpr: string | undefined
  if (typeof tags === 'string') tagsExpr = tags.trim() || undefined
  else if (tags && Object.keys(tags).length) tagsExpr = seriesLabel({ tags })
  void router.push(
    buildQueryPrefillPath({
      database: db,
      measurement: measurement || undefined,
      tags: tagsExpr,
      range: '1h',
      hash: '#query-form',
    }),
  )
}

function openWriteFor(db: string, measurement?: string) {
  void router.push(
    buildWritePrefillPath({
      database: db,
      measurement: measurement || undefined,
      hash: '#write-mode-typed',
    }),
  )
}

function fieldTypeName(type: number): string {
  switch (type) {
    case 1:
      return t.value('typeFloat')
    case 2:
      return t.value('typeInt')
    case 3:
      return t.value('typeString')
    case 4:
      return t.value('typeBool')
    default:
      return String(type)
  }
}

function rowsForExport() {
  return filterRowsByIds(filteredDatabases.value, exportIds.value, (d) => d.name)
}

async function exportJSON() {
  const list = rowsForExport()
  if (!list.length) {
    warn(t.value('inventoryExportEmpty'))
    return
  }
  if (exportBusy.value) return
  const rows = list.map((db) => ({
    name: db.name,
    measurement_count: db.loaded ? db.measurements.length : undefined,
    retention_policy_count: db.loaded ? db.retentionPolicies.length : undefined,
    loaded: db.loaded,
  }))
  const outcome = await runJSONExport({
    label: 'JSON',
    filename: stampFilename('mts-databases', 'json'),
    total: rows.length,
    build: async ({ isCancelled, progress }) => {
      progress(0, rows.length)
      const chunk = 200
      for (let i = 0; i < rows.length; i++) {
        if (isCancelled()) return null
        const done = i + 1
        if (done === rows.length || done % chunk === 0) {
          progress(done, rows.length)
          if (done < rows.length) await new Promise((r) => setTimeout(r, 0))
        }
      }
      return buildDatabasesExport(rows)
    },
  })
  if (outcome === 'done') success(t.value('inventoryExported'))
  else if (outcome === 'cancelled') info(t.value('exportCancelledToast'))
  else if (outcome === 'error') notifyError(exportJob.value.error || t.value('failed'))
}

async function exportCSV() {
  const list = rowsForExport()
  if (!list.length) {
    warn(t.value('inventoryExportEmpty'))
    return
  }
  if (exportBusy.value) return
  const rows = list.map((db) => ({
    name: db.name,
    measurement_count: db.loaded ? db.measurements.length : undefined,
    retention_policy_count: db.loaded ? db.retentionPolicies.length : undefined,
    loaded: db.loaded,
  }))
  const outcome = await runTextExport({
    label: 'CSV',
    filename: stampFilename('mts-databases', 'csv'),
    mime: 'text/csv;charset=utf-8',
    total: rows.length,
    build: async ({ isCancelled, progress }) => {
      progress(0, rows.length)
      const chunk = 200
      for (let i = 0; i < rows.length; i++) {
        if (isCancelled()) return null
        const done = i + 1
        if (done === rows.length || done % chunk === 0) {
          progress(done, rows.length)
          if (done < rows.length) await new Promise((r) => setTimeout(r, 0))
        }
      }
      return databasesToCSV(rows)
    },
  })
  if (outcome === 'done') success(t.value('inventoryExported'))
  else if (outcome === 'cancelled') info(t.value('exportCancelledToast'))
  else if (outcome === 'error') notifyError(exportJob.value.error || t.value('failed'))
}

onBeforeUnmount(() => {
  cancelDbAction()
  unregisterDatabasesDirty?.()
  unregisterDatabasesDirty = null
  window.removeEventListener('beforeunload', onDatabasesBeforeUnload)
})
</script>
<template>
  <div class="space-y-4" data-testid="databases-page">
    <ActionResultBanner
      v-if="loadError && !databases.length"
      kind="error"
      :message="loadError"
      retryable
      data-testid="databases-load-error"
      @retry="loadDatabasesList"
      @dismiss="loadError = ''"
    />
    <PartialErrorBanner
      v-else-if="loadError && databases.length"
      :message="`${t('databasesRefreshFailed')}：${loadError}`"
      test-id="databases-refresh-error"
      @retry="loadDatabasesList"
      @dismiss="loadError = ''"
    />
    <ActionResultBanner
      :result="actionResult"
      :retryable="canRetryAction"
      :action-label="databasesAdminBusyAction?.label || ''"
      :action-path="databasesAdminBusyAction?.path || ''"
      data-testid="databases-action-result"
      @retry="retryLastDatabaseAction"
      @dismiss="clearActionResult"
    />
    <InFlightBanner
      :active="confirmLoading || createDbLoading || rpLoading"
      :started-at-ms="dbActionStartedAt"
      kind="admin"
      @cancel="cancelDbAction"
    />
    <p
      v-if="!isAdmin"
      class="rounded-lg border border-amber-200 bg-amber-50 px-3 py-2 text-xs text-amber-900 dark:border-amber-900 dark:bg-amber-950/40 dark:text-amber-100"
      data-testid="databases-readonly-hint"
    >{{ t('databasesReadOnlyHint') }}</p>
    <div class="flex flex-wrap items-center justify-between gap-3">
      <div>
        <h1 class="text-lg font-semibold text-slate-800 dark:text-slate-100">{{ t('databases') }}</h1>
        <div class="mt-1 flex flex-wrap items-center gap-2">
          <p class="text-xs mts-muted">{{ isAdmin ? t('databasesDesc') : t('databasesReadOnlyDesc') }}</p>
          <AdminOpLastChip
            v-if="isAdmin && databasesAdminLastLabel"
            :label="databasesAdminLastLabel"
            :last-ok="adminOpBusySummary?.lastOk"
            :last-error="databasesAdminLastErrorDetail"
            test-id="databases-admin-last"
            show-copy
            copy-test-id="databases-admin-last-copy"
            error-test-id="databases-admin-last-error"
          />

        </div>
      </div>
      <div class="flex flex-wrap gap-2">
        <ExportJobBanner class="w-full basis-full" :job="exportJob" :retryable="canRetryExport" @cancel="cancelExport" @retry="retryLastExport" @dismiss="resetExport" />
        <button type="button" class="mts-btn" data-testid="databases-export-json" :disabled="exportBusy || !filteredDatabases.length" @click="exportJSON">
          <Download class="h-3.5 w-3.5" /> {{ t('inventoryExportJSON') }}
        </button>
        <button type="button" class="mts-btn" data-testid="databases-export-csv" :disabled="exportBusy || !filteredDatabases.length" @click="exportCSV">
          <Download class="h-3.5 w-3.5" /> {{ t('inventoryExportCSV') }}
        </button>
        <button type="button" class="mts-btn" data-testid="databases-share-link" @click="copyDatabasesShareLink">
          {{ t('databasesShareLink') }}
        </button>
        <template v-if="isAdmin">
          <input v-model="newDbName" type="text" :placeholder="t('databasesCreatePlaceholder')" class="w-56 rounded-lg border border-slate-300 px-3 py-2 text-sm focus:border-slate-500 focus:outline-none dark:border-slate-600 dark:bg-slate-800" data-testid="databases-create-input" @keyup.enter="createDatabase" />
          <button type="button" class="inline-flex items-center gap-1 rounded-lg bg-slate-800 px-4 py-2 text-sm font-medium text-white hover:bg-slate-700" data-testid="databases-create-btn" :disabled="writeBlocked || createDbLoading" :title="writeBlocked ? t(blockedMessageKey('offlineAdminBlocked')) : undefined" @click="createDatabase">
            <Plus class="h-4 w-4" /> {{ t('databasesCreate') }}
          </button>
          <span
            v-if="databasesFormDirty"
            data-testid="databases-dirty-badge"
            class="rounded-full bg-amber-100 px-2 py-0.5 text-[11px] font-medium text-amber-800 dark:bg-amber-950/50 dark:text-amber-200"
          >{{ t('adminDirtyBadge') }}</span>
        </template>
      </div>
    </div>

    <div id="databases-filter-bar" class="scroll-mt-20 flex flex-wrap items-end gap-3" data-testid="databases-filter-bar">
      <label class="text-xs mts-muted">{{ t('filter') }}
        <input
          v-model="dbFilter"
          type="search"
          class="mts-input mt-1 min-w-[12rem]"
          data-testid="databases-filter"
          :placeholder="t('databasesFilterPlaceholder')"
        />
      </label>
      <span class="text-xs mts-muted" data-testid="databases-filter-count">{{ filteredDatabases.length }} / {{ databases.length }}</span>
      <ListSelectionToolbar
        prefix="databases"
        :selected-count="selectedCount"
        :has-visible="!!filteredDatabases.length"
        @select-all="toggleAllVisible(true)"
        @clear="clearSelection"
      >
        <template #actions>
          <button type="button" class="mts-btn" data-testid="databases-sort-name" :title="t('listSortBy')" @click="cycleDbSort" :aria-sort="ariaSortValue(dbSort, 'name')">{{ t('listSortBy') }} {{ dbSortIndicator() }}</button>
        </template>
      </ListSelectionToolbar>
    </div>

    <div v-if="!filteredDatabases.length" class="mts-card">
      <EmptyState
        data-testid="databases-empty-filter"
        :title="databases.length ? t('databasesFilterEmpty') : t('databasesEmpty')"
        :description="databases.length ? t('databasesFilterEmptyDesc') : t('databasesEmptyDesc')"
      >
        <template v-if="databases.length" #action>
          <button type="button" class="mts-btn-primary" data-testid="databases-clear-filters" @click="dbFilter = ''">{{ t('clearFilters') }}</button>
        </template>
        <template v-else-if="isAdmin" #action>
          <button type="button" class="mts-btn-primary" data-testid="databases-empty-create" :disabled="writeBlocked" :title="writeBlocked ? t(blockedMessageKey('offlineAdminBlocked')) : undefined" @click="createDatabase">{{ t('databasesCreate') }}</button>
        </template>
      </EmptyState>
    </div>

    <div v-else class="space-y-3" data-testid="databases-list-panel">
      <div class="overflow-hidden rounded-xl border border-slate-200 bg-white dark:border-slate-700 dark:bg-slate-900">
        <VirtualTable
          :items="filteredDatabases"
          :row-height="DB_ROW_HEIGHT"
          :height="DB_LIST_HEIGHT"
          data-testid="databases-virtual-list"
        >
          <template #default="{ item: db }">
            <div
              class="flex h-full items-center justify-between border-b border-slate-100 px-4 hover:bg-slate-50 dark:border-slate-800 dark:hover:bg-slate-800"
              :data-testid="`databases-row-${db.name}`"
            >
              <div class="flex min-w-0 items-center gap-2">
                <input
                  type="checkbox"
                  class="h-3.5 w-3.5 shrink-0"
                  :data-testid="`databases-select-${db.name}`"
                  :checked="isSelected(db.name)"
                  :aria-label="t('listSelectCol') + ' ' + db.name"
                  @change="toggle(db.name, ($event.target as HTMLInputElement).checked)"
                  @click.stop
                />
                <button type="button" class="flex min-w-0 items-center gap-2 text-left" :data-testid="`databases-expand-${db.name}`" @click="toggleExpand(db)">
                  <component :is="db.expanded ? ChevronDown : ChevronRight" class="h-4 w-4 shrink-0 text-slate-400 dark:text-slate-500" />
                  <span class="truncate text-sm font-medium text-slate-800 dark:text-slate-100">{{ db.name }}</span>
                  <span v-if="db.loading" class="text-xs text-slate-400 dark:text-slate-500">{{ t('databasesLoading') }}</span>
                  <span
                    v-else-if="db.loaded"
                    class="truncate text-[11px] mts-muted"
                  >{{ db.measurements.length }} {{ t('databasesMeasurements') }} · {{ db.retentionPolicies.length }} {{ t('databasesRetention') }}</span>
                </button>
              </div>
              <button
                v-if="isAdmin"
                type="button"
                class="rounded p-1 text-slate-400 hover:text-red-600 dark:text-slate-500 dark:hover:text-red-300 disabled:cursor-not-allowed disabled:opacity-40"
                :title="writeBlocked ? t(blockedMessageKey('offlineAdminBlocked')) : t('databasesDeleteDbBtnTitle')"
                :disabled="writeBlocked"
                :data-testid="`databases-delete-${db.name}`"
                @click="requestDeleteDatabase(db.name)"
              >
                <Trash2 class="h-4 w-4" />
              </button>
            </div>
          </template>
        </VirtualTable>
        <p class="border-t border-slate-100 px-3 py-1.5 text-[11px] mts-muted dark:border-slate-800" data-testid="databases-virtual-hint">
          {{ t('databasesVirtualHint') }}
        </p>
      </div>

      <div
        v-if="activeDatabase && activeDatabase.expanded"
        id="databases-detail"
        class="scroll-mt-20 overflow-hidden rounded-xl border border-slate-200 bg-white dark:border-slate-700 dark:bg-slate-900"
        data-testid="databases-detail-panel"
      >
        <div class="flex flex-wrap items-center justify-between gap-2 border-b border-slate-100 px-4 py-2 dark:border-slate-800">
          <p class="text-sm font-medium text-slate-800 dark:text-slate-100">{{ activeDatabase.name }}</p>
          <div class="flex flex-wrap items-center gap-2">
            <button
              type="button"
              class="mts-btn text-xs"
              data-testid="databases-open-query"
              @click="openQueryFor(activeDatabase.name)"
            >{{ t('databasesOpenQuery') }}</button>
            <button
              type="button"
              class="mts-btn text-xs"
              data-testid="databases-open-write"
              @click="openWriteFor(activeDatabase.name)"
            >{{ t('databasesOpenWrite') }}</button>
            <button type="button" class="mts-btn" data-testid="databases-detail-collapse" @click="activeDatabase.expanded = false">
              {{ t('collapse') }}
            </button>
          </div>
        </div>
        <div v-if="activeDatabase.loading" class="px-6 py-4 text-sm mts-muted" data-testid="databases-detail-loading">
          {{ t('databasesLoading') }}
        </div>
        <PartialErrorBanner
          v-else-if="activeDatabase.detailError"
          class="mx-4 my-3"
          :message="`${t('databasesDetailFailed')}：${activeDatabase.detailError}`"
          test-id="databases-detail-error"
          @retry="loadDatabaseDetails(activeDatabase)"
          @dismiss="activeDatabase.detailError = ''"
        />
        <div v-else-if="activeDatabase.loaded" class="px-6 py-3">
          <div class="mb-2 flex flex-wrap items-end justify-between gap-2">
            <p class="text-xs font-semibold uppercase tracking-wide text-slate-500 dark:text-slate-400">{{ t('databasesMeasurements') }}</p>
            <label v-if="activeDatabase.measurements.length" class="text-[11px] mts-muted">
              {{ t('filter') }}
              <input
                v-model="measFilter"
                type="search"
                class="mts-input mt-0.5 min-w-[10rem] text-xs"
                data-testid="databases-meas-filter"
                :placeholder="t('databasesMeasFilterPh')"
              />
            </label>
            <span
              v-if="activeDatabase.measurements.length"
              class="text-[11px] mts-muted"
              data-testid="databases-meas-count"
            >{{ filteredMeasurements.length }} / {{ activeDatabase.measurements.length }}</span>
          </div>
          <EmptyState v-if="!activeDatabase.measurements.length" compact :title="t('databasesNoMeasurement')" :description="t('databasesNoMeasurementDesc')" />
          <EmptyState
            v-else-if="!filteredMeasurements.length"
            compact
            data-testid="databases-meas-empty-filter"
            :title="t('databasesMeasFilterEmpty')"
            :description="t('databasesMeasFilterEmptyDesc')"
          >
            <template #action>
              <button type="button" class="mts-btn-primary" data-testid="databases-meas-clear-filters" @click="measFilter = ''">{{ t('clearFilters') }}</button>
            </template>
          </EmptyState>
          <div v-for="meas in filteredMeasurements" :key="meas.name" class="mb-2 rounded border border-slate-100 dark:border-slate-800">
            <div class="flex flex-wrap items-center justify-between gap-1">
              <button
                type="button"
                class="flex min-w-0 flex-1 items-center gap-2 px-3 py-2 text-left text-sm text-slate-700 hover:bg-slate-50 dark:text-slate-200 dark:hover:bg-slate-800"
                :data-testid="`databases-meas-${meas.name}`"
                @click="toggleMeasurement(meas, activeDatabase.name)"
              >
                <component :is="meas.expanded ? ChevronDown : ChevronRight" class="h-3.5 w-3.5 shrink-0 text-slate-400 dark:text-slate-500" />
                <Table2 class="h-3.5 w-3.5 shrink-0 text-slate-400 dark:text-slate-500" />
                <span class="truncate">{{ meas.name }}</span>
              </button>
              <div class="flex shrink-0 items-center gap-1 px-2 py-1">
                <button
                  type="button"
                  class="mts-btn px-2 py-0.5 text-[11px]"
                  :data-testid="`databases-meas-query-${meas.name}`"
                  @click="openQueryFor(activeDatabase.name, meas.name)"
                >{{ t('query') }}</button>
                <button
                  type="button"
                  class="mts-btn px-2 py-0.5 text-[11px]"
                  :data-testid="`databases-meas-write-${meas.name}`"
                  @click="openWriteFor(activeDatabase.name, meas.name)"
                >{{ t('write') }}</button>
              </div>
            </div>
            <div v-if="meas.expanded" class="border-t border-slate-50 px-4 py-2 text-xs text-slate-600 dark:border-slate-800 dark:text-slate-300">
              <div v-if="meas.loading" class="text-slate-400 dark:text-slate-500" data-testid="databases-meas-loading">{{ t('databasesLoading') }}</div>
              <PartialErrorBanner
                v-else-if="meas.loadError"
                class="my-1"
                :message="`${t('databasesMeasFailed')}：${meas.loadError}`"
                :test-id="`databases-meas-error-${meas.name}`"
                @retry="loadMeasurementDetails(meas, activeDatabase.name)"
                @dismiss="meas.loadError = ''"
              />
              <template v-if="!meas.loading && (!meas.loadError || meas.fields.length || meas.series.length || meas.seriesTotal)">
                <p class="mb-1 font-medium text-slate-500 dark:text-slate-400">{{ t('databasesFields') }}</p>
                <div class="mb-2 flex flex-wrap gap-1">
                  <span v-for="f in meas.fields" :key="f.name" class="rounded bg-slate-100 px-2 py-0.5 dark:bg-slate-800">{{ f.name }}:{{ fieldTypeName(f.type) }}</span>
                  <span v-if="!meas.fields.length" class="text-slate-400 dark:text-slate-500">{{ t('databasesNone') }}</span>
                </div>
                <p class="mb-1 font-medium text-slate-500 dark:text-slate-400">{{ t('databasesSeries') }}</p>
                <div class="mb-1 flex flex-wrap items-center gap-1">
                  <input
                    v-model="meas.seriesLocalFilter"
                    type="search"
                    class="mts-input min-w-[10rem] flex-1 text-[11px]"
                    :data-testid="`databases-series-filter-${meas.name}`"
                    :placeholder="t('databasesSeriesFilterPh')"
                    :disabled="meas.loading || meas.seriesLoadingMore"
                    @keydown.enter.prevent="applySeriesServerFilter(meas, activeDatabase.name)"
                  />
                  <button
                    type="button"
                    class="mts-btn text-[11px]"
                    :data-testid="`databases-series-server-filter-${meas.name}`"
                    :disabled="meas.loading || meas.seriesLoadingMore"
                    @click="applySeriesServerFilter(meas, activeDatabase.name)"
                  >{{ t('databasesSeriesServerFilter') }}</button>
                </div>
                <div class="space-y-1" data-testid="databases-series-list">
                  <div v-for="s in filteredSeriesOf(meas)" :key="s.id" class="flex flex-wrap items-center gap-1">
                    <Tag class="h-3 w-3 shrink-0 text-slate-400 dark:text-slate-500" />
                    <span class="min-w-0 flex-1 font-mono">{{ seriesLabel(s) }}</span>
                    <button
                      type="button"
                      class="mts-btn px-1.5 py-0.5 text-[11px]"
                      :data-testid="`databases-series-query-${s.id}`"
                      @click="openQueryFor(activeDatabase.name, meas.name, s.tags)"
                    >{{ t('query') }}</button>
                  </div>
                  <span v-if="!meas.series.length" class="text-slate-400 dark:text-slate-500">{{ t('databasesNone') }}</span>
                  <div
                    v-else-if="!filteredSeriesOf(meas).length"
                    class="space-y-1"
                    :data-testid="`databases-series-filter-empty-${meas.name}`"
                  >
                    <span class="text-slate-400 dark:text-slate-500">{{ t('databasesSeriesFilterEmpty') }}</span>
                    <button
                      type="button"
                      class="mts-btn block text-[11px]"
                      :data-testid="`databases-series-clear-filters-${meas.name}`"
                      @click="meas.seriesLocalFilter = ''; meas.seriesQuery = ''; loadMeasurementDetails(meas, activeDatabase.name)"
                    >{{ t('clearFilters') }}</button>
                  </div>
                  <p
                    v-if="meas.seriesTruncated || meas.seriesHasMore"
                    class="text-[11px] text-amber-700 dark:text-amber-200"
                    data-testid="databases-series-truncated"
                  >{{ formatMessage(t('databasesSeriesTruncated'), { max: SERIES_CAP, total: meas.seriesTotal }) }}</p>
                  <p
                    v-if="meas.seriesPath"
                    class="max-w-full truncate font-mono text-[10px] text-slate-500 dark:text-slate-400"
                    data-testid="databases-series-path"
                    :title="meas.seriesPath"
                  >{{ meas.seriesPath }}</p>
                  <button
                    v-if="meas.seriesHasMore"
                    type="button"
                    class="mts-btn mt-1 text-[11px]"
                    :data-testid="`databases-series-load-more-${meas.name}`"
                    :disabled="meas.seriesLoadingMore"
                    :aria-busy="meas.seriesLoadingMore ? 'true' : undefined"
                    @click="loadMoreSeries(meas, activeDatabase.name)"
                  >{{ meas.seriesLoadingMore ? t('loading') : t('databasesSeriesLoadMore') }}</button>
                </div>
              </template>
            </div>
          </div>
        <div class="border-t border-slate-200 px-6 py-3 dark:border-slate-700">
          <p class="mb-2 text-xs font-semibold uppercase tracking-wide text-slate-500 dark:text-slate-400">{{ t('databasesRetention') }}</p>
          <p
            v-if="activeDatabase.retentionPath"
            class="mb-2 max-w-full truncate font-mono text-[10px] text-slate-500 dark:text-slate-400"
            data-testid="databases-rp-path"
            :title="activeDatabase.retentionPath"
          >{{ activeDatabase.retentionPath }}</p>
          <div v-if="activeDatabase.retentionPolicies.length" class="mb-3 space-y-1" data-testid="databases-rp-list">
            <div
              v-for="rp in activeDatabase.retentionPolicies"
              :key="rp.name"
              class="flex items-center gap-2 rounded border border-slate-200 bg-white px-3 py-1.5 dark:border-slate-700 dark:bg-slate-900"
              :data-testid="`databases-rp-row-${rp.name}`"
            >
              <Clock class="h-3.5 w-3.5 text-slate-400 dark:text-slate-500" />
              <span class="text-sm font-medium text-slate-700 dark:text-slate-200">{{ rp.name }}</span>
              <span class="text-xs text-slate-500 dark:text-slate-400">{{ formatDuration(rp.duration) }}</span>
            </div>
          </div>
          <EmptyState
            v-else
            compact
            data-testid="databases-rp-empty"
            :title="t('databasesRpEmpty')"
            :description="isAdmin ? t('databasesRpEmptyAdminDesc') : t('databasesRpEmptyReadOnlyDesc')"
          />
          <div v-if="isAdmin" class="mt-2 flex flex-wrap items-center gap-2" data-testid="databases-rp-create">
            <input v-model="activeDatabase.newRpName" type="text" :placeholder="t('databasesRpNamePlaceholder')" class="w-28 rounded border border-slate-300 px-2 py-1 text-xs dark:border-slate-600" data-testid="databases-rp-name" />
            <input v-model="activeDatabase.newRpDuration" type="text" :placeholder="t('databasesRpDurationPlaceholder')" class="w-24 rounded border border-slate-300 px-2 py-1 text-xs dark:border-slate-600" data-testid="databases-rp-duration" />
            <button type="button" class="disabled:cursor-not-allowed disabled:opacity-50 inline-flex items-center gap-1 rounded bg-slate-800 px-3 py-1 text-xs font-medium text-white" data-testid="databases-rp-add" @click="createRetentionPolicy(activeDatabase)" :disabled="writeBlocked || rpLoading" :title="writeBlocked ? t(blockedMessageKey('offlineAdminBlocked')) : undefined">
              <Plus class="h-3.5 w-3.5" /> {{ t('databasesAdd') }}
            </button>
          </div>
        </div>
        </div>
      </div>
    </div>
    <ConfirmDialog
      v-model:open="confirmOpen"
      :write-blocked="writeBlocked"
      :block-reason="blockReason"
      :offline-message-key="'offlineAdminBlocked'"
      :title="t('databasesDeleteDbBtnTitle')"
      :message="t('databasesDeleteDbMsg')"
      :require-text="confirmDbName"
      :confirm-label="t('delete')"
      danger
      :loading="confirmLoading"
      allow-cancel-while-loading
      @confirm="confirmDeleteDatabase"
      @cancel="cancelDbAction"
    />
  </div>
</template>
