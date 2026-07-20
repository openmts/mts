<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { apiGet, apiPost, apiDelete } from '@/api/client'
import { listDatabases, listMeasurements, listRetentionPolicies, listSeriesDetailed } from '@/api/meta'
import { seriesLabel } from '@/utils/seriesMeta'
import { formatMessage } from '@/utils/formatMessage'
import {
  Plus, Trash2, ChevronDown, ChevronRight, Table2, Tag, Clock, Download,
} from 'lucide-vue-next'
import { useAuth } from '@/composables/useAuth'
import ConfirmDialog from '@/components/ConfirmDialog.vue'
import ActionResultBanner from '@/components/ActionResultBanner.vue'
import EmptyState from '@/components/EmptyState.vue'
import ListSelectionToolbar from '@/components/ListSelectionToolbar.vue'
import VirtualTable from '@/components/VirtualTable.vue'
import { makeActionResult, type ActionResult } from '@/utils/actionResult'
import { useNotify } from '@/composables/useNotify'
import { formatCaughtError } from '@/utils/apiError'
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
} from '@/utils/listSort'
import { useI18n } from '@/composables/useI18n'
import type { MessageKey } from '@/i18n/messages'
import { buildDatabasesExport, databasesToCSV } from '@/utils/databasesExport'
import { downloadJSON, downloadText, stampFilename } from '@/utils/download'
interface FieldSchema { measurement: string; name: string; type: number }
interface FieldsResponse { fields: FieldSchema[] }
interface Series { id: number; measurement: string; tags: Record<string, string> }
interface MeasurementEntry {
  name: string
  expanded: boolean
  loading: boolean
  fields: FieldSchema[]
  series: Series[]
  seriesTotal: number
  seriesTruncated: boolean
}
interface DatabaseEntry {
  name: string
  expanded: boolean
  loading: boolean
  loaded: boolean
  measurements: MeasurementEntry[]
  retentionPolicies: { name: string; duration: number }[]
  newRpName: string
  newRpDuration: string
}
const { isAdmin } = useAuth()
const SERIES_CAP = 200
const { t } = useI18n()
const { success, error: notifyError, warn } = useNotify()
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
const actionResult = ref<ActionResult | null>(null)
const confirmOpen = ref(false)
const confirmDbName = ref('')
const confirmLoading = ref(false)
onMounted(async () => {
  try {
    const names = await listDatabases()
    databases.value = names.map((name) => ({
      name,
      expanded: false,
      loading: false,
      loaded: false,
      measurements: [],
      retentionPolicies: [],
      newRpName: '',
      newRpDuration: '',
    }))
    pruneTo(names)
  } catch (e) {
    loadError.value = formatCaughtError(e)
  }
})
async function loadDatabaseDetails(db: DatabaseEntry) {
  db.loading = true
  actionResult.value = null
  try {
    const [meas, rps] = await Promise.all([
      listMeasurements(db.name),
      listRetentionPolicies(db.name),
    ])
    db.measurements = meas.map((m) => ({
      name: m,
      expanded: false,
      loading: false,
      fields: [],
      series: [],
      seriesTotal: 0,
      seriesTruncated: false,
    }))
    db.retentionPolicies = rps.map((p) => ({ name: p.name, duration: p.duration ?? 0 }))
    db.loaded = true
  } catch (e) {
    const msg = formatCaughtError(e); actionResult.value = makeActionResult('error', msg)
    db.loaded = false
    db.expanded = false
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
async function toggleMeasurement(meas: MeasurementEntry, dbName: string) {
  meas.expanded = !meas.expanded
  if (meas.expanded && !meas.fields.length && !meas.series.length && !meas.seriesTotal) {
    meas.loading = true
    try {
      const [fieldsData, seriesResult] = await Promise.all([
        apiGet<FieldsResponse>(`/api/v1/data/databases/${encodeURIComponent(dbName)}/measurements/${encodeURIComponent(meas.name)}/fields`),
        listSeriesDetailed(dbName, meas.name, { limit: SERIES_CAP }),
      ])
      meas.fields = fieldsData.fields ?? []
      meas.series = (seriesResult.series as Series[]).map((s) => ({
        id: s.id ?? 0,
        measurement: s.measurement ?? meas.name,
        tags: s.tags ?? {},
      }))
      meas.seriesTotal = seriesResult.total
      meas.seriesTruncated = seriesResult.truncated
    } catch (e) {
      const msg = formatCaughtError(e); actionResult.value = makeActionResult('error', msg)
    } finally {
      meas.loading = false
    }
  }
}
async function createDatabase() {
  if (!isAdmin.value) return
  if (!newDbName.value.trim()) return
  actionResult.value = null
  try {
    await apiPost('/api/v1/admin/databases', { name: newDbName.value.trim() })
    databases.value.push({
      name: newDbName.value.trim(),
      measurements: [],
      retentionPolicies: [],
      expanded: false,
      loading: false,
      loaded: false,
      newRpName: '',
      newRpDuration: '',
    })
    newDbName.value = ''
    actionResult.value = makeActionResult('ok', t.value('databasesCreated'))
    success(t.value('databasesCreated'))
  } catch (e) {
    const msg = formatCaughtError(e)
    actionResult.value = makeActionResult('error', msg)
    notifyError(msg)
  }
}
function requestDeleteDatabase(name: string) {
  if (!isAdmin.value) return
  confirmDbName.value = name
  confirmOpen.value = true
}
async function confirmDeleteDatabase() {
  const name = confirmDbName.value
  if (!name) return
  confirmLoading.value = true
  actionResult.value = null
  try {
    await apiDelete(`/api/v1/admin/databases/${encodeURIComponent(name)}`)
    databases.value = databases.value.filter((d) => d.name !== name)
    pruneTo(databases.value.map((d) => d.name))
    confirmOpen.value = false
    success(formatMessage(t.value('databasesDeleted'), { name }))
  } catch (e) {
    const msg = formatCaughtError(e)
    actionResult.value = makeActionResult('error', msg)
    notifyError(msg)
  } finally {
    confirmLoading.value = false
  }
}
async function createRetentionPolicy(db: DatabaseEntry) {
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
    actionResult.value = makeActionResult('error', msg)
    notifyError(msg)
    return
  }
  actionResult.value = null
  try {
    await apiPost(`/api/v1/admin/databases/${encodeURIComponent(db.name)}/retention-policies`, {
      policy: { name, duration: durationNs },
    })
    db.retentionPolicies.push({ name, duration: durationNs })
    db.newRpName = ''
    db.newRpDuration = ''
    actionResult.value = makeActionResult('ok', t.value('databasesRpCreated'))
    success(t.value('databasesRpCreated'))
  } catch (e) {
    const msg = formatCaughtError(e)
    actionResult.value = makeActionResult('error', msg)
    notifyError(msg)
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

function exportJSON() {
  const list = rowsForExport()
  if (!list.length) {
    warn(t.value('inventoryExportEmpty'))
    return
  }
  downloadJSON(
    stampFilename('mts-databases', 'json'),
    buildDatabasesExport(
      list.map((db) => ({
        name: db.name,
        measurement_count: db.loaded ? db.measurements.length : undefined,
        retention_policy_count: db.loaded ? db.retentionPolicies.length : undefined,
        loaded: db.loaded,
      })),
    ),
  )
  success(t.value('inventoryExported'))
}

function exportCSV() {
  const list = rowsForExport()
  if (!list.length) {
    warn(t.value('inventoryExportEmpty'))
    return
  }
  const rows = list.map((db) => ({
    name: db.name,
    measurement_count: db.loaded ? db.measurements.length : undefined,
    retention_policy_count: db.loaded ? db.retentionPolicies.length : undefined,
    loaded: db.loaded,
  }))
  downloadText(stampFilename('mts-databases', 'csv'), databasesToCSV(rows), 'text/csv;charset=utf-8')
  success(t.value('inventoryExported'))
}
</script>
<template>
  <div class="space-y-4" data-testid="databases-page">
    <ActionResultBanner v-if="loadError" kind="error" :message="loadError" @dismiss="loadError = ''" />
    <ActionResultBanner :result="actionResult" @dismiss="actionResult = null" />
    <p
      v-if="!isAdmin"
      class="rounded-lg border border-amber-200 bg-amber-50 px-3 py-2 text-xs text-amber-900 dark:border-amber-900 dark:bg-amber-950/40 dark:text-amber-100"
      data-testid="databases-readonly-hint"
    >{{ t('databasesReadOnlyHint') }}</p>
    <div class="flex flex-wrap items-center justify-between gap-3">
      <div>
        <h1 class="text-lg font-semibold text-slate-800 dark:text-slate-100">{{ t('databases') }}</h1>
        <p class="text-xs mts-muted">{{ isAdmin ? t('databasesDesc') : t('databasesReadOnlyDesc') }}</p>
      </div>
      <div class="flex flex-wrap gap-2">
        <button type="button" class="mts-btn" data-testid="databases-export-json" :disabled="!filteredDatabases.length" @click="exportJSON">
          <Download class="h-3.5 w-3.5" /> {{ t('inventoryExportJSON') }}
        </button>
        <button type="button" class="mts-btn" data-testid="databases-export-csv" :disabled="!filteredDatabases.length" @click="exportCSV">
          <Download class="h-3.5 w-3.5" /> {{ t('inventoryExportCSV') }}
        </button>
        <template v-if="isAdmin">
          <input v-model="newDbName" type="text" :placeholder="t('databasesCreatePlaceholder')" class="w-56 rounded-lg border border-slate-300 px-3 py-2 text-sm focus:border-slate-500 focus:outline-none dark:border-slate-600 dark:bg-slate-800" data-testid="databases-create-input" @keyup.enter="createDatabase" />
          <button type="button" class="inline-flex items-center gap-1 rounded-lg bg-slate-800 px-4 py-2 text-sm font-medium text-white hover:bg-slate-700" data-testid="databases-create-btn" @click="createDatabase">
            <Plus class="h-4 w-4" /> {{ t('databasesCreate') }}
          </button>
        </template>
      </div>
    </div>

    <div class="flex flex-wrap items-end gap-3" data-testid="databases-filter-bar">
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
          <button type="button" class="mts-btn" data-testid="databases-sort-name" :title="t('listSortBy')" @click="cycleDbSort">{{ t('listSortBy') }} {{ dbSortIndicator() }}</button>
        </template>
      </ListSelectionToolbar>
    </div>

    <div v-if="!filteredDatabases.length" class="mts-card">
      <EmptyState
        data-testid="databases-empty-filter"
        :title="databases.length ? t('databasesFilterEmpty') : t('databasesEmpty')"
        :description="databases.length ? t('databasesFilterEmptyDesc') : t('databasesEmptyDesc')"
      >
        <template v-if="!databases.length && isAdmin" #action>
          <button type="button" class="mts-btn-primary" data-testid="databases-empty-create" @click="createDatabase">{{ t('databasesCreate') }}</button>
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
                class="rounded p-1 text-slate-400 hover:text-red-600 dark:text-slate-500 dark:hover:text-red-300"
                :title="t('databasesDeleteDbBtnTitle')"
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
        v-if="activeDatabase && activeDatabase.expanded && activeDatabase.loaded"
        class="overflow-hidden rounded-xl border border-slate-200 bg-white dark:border-slate-700 dark:bg-slate-900"
        data-testid="databases-detail-panel"
      >
        <div class="flex items-center justify-between border-b border-slate-100 px-4 py-2 dark:border-slate-800">
          <p class="text-sm font-medium text-slate-800 dark:text-slate-100">{{ activeDatabase.name }}</p>
          <button type="button" class="mts-btn" data-testid="databases-detail-collapse" @click="activeDatabase.expanded = false">
            {{ t('collapse') }}
          </button>
        </div>
        <div class="px-6 py-3">
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
          />
          <div v-for="meas in filteredMeasurements" :key="meas.name" class="mb-2 rounded border border-slate-100 dark:border-slate-800">
            <button
              type="button"
              class="flex w-full items-center gap-2 px-3 py-2 text-left text-sm text-slate-700 hover:bg-slate-50 dark:text-slate-200 dark:hover:bg-slate-800"
              :data-testid="`databases-meas-${meas.name}`"
              @click="toggleMeasurement(meas, activeDatabase.name)"
            >
              <component :is="meas.expanded ? ChevronDown : ChevronRight" class="h-3.5 w-3.5 text-slate-400 dark:text-slate-500" />
              <Table2 class="h-3.5 w-3.5 text-slate-400 dark:text-slate-500" />
              {{ meas.name }}
            </button>
            <div v-if="meas.expanded" class="border-t border-slate-50 px-4 py-2 text-xs text-slate-600 dark:border-slate-800 dark:text-slate-300">
              <div v-if="meas.loading" class="text-slate-400 dark:text-slate-500">{{ t('databasesLoading') }}</div>
              <template v-else>
                <p class="mb-1 font-medium text-slate-500 dark:text-slate-400">{{ t('databasesFields') }}</p>
                <div class="mb-2 flex flex-wrap gap-1">
                  <span v-for="f in meas.fields" :key="f.name" class="rounded bg-slate-100 px-2 py-0.5 dark:bg-slate-800">{{ f.name }}:{{ fieldTypeName(f.type) }}</span>
                  <span v-if="!meas.fields.length" class="text-slate-400 dark:text-slate-500">{{ t('databasesNone') }}</span>
                </div>
                <p class="mb-1 font-medium text-slate-500 dark:text-slate-400">{{ t('databasesSeries') }}</p>
                <div class="space-y-1" data-testid="databases-series-list">
                  <div v-for="s in meas.series" :key="s.id" class="flex items-center gap-1">
                    <Tag class="h-3 w-3 text-slate-400 dark:text-slate-500" />
                    <span class="font-mono">{{ seriesLabel(s) }}</span>
                  </div>
                  <span v-if="!meas.series.length" class="text-slate-400 dark:text-slate-500">{{ t('databasesNone') }}</span>
                  <p
                    v-if="meas.seriesTruncated"
                    class="text-[11px] text-amber-700 dark:text-amber-200"
                    data-testid="databases-series-truncated"
                  >{{ formatMessage(t('databasesSeriesTruncated'), { max: SERIES_CAP, total: meas.seriesTotal }) }}</p>
                </div>
              </template>
            </div>
          </div>
        </div>
        <div class="border-t border-slate-200 px-6 py-3 dark:border-slate-700">
          <p class="mb-2 text-xs font-semibold uppercase tracking-wide text-slate-500 dark:text-slate-400">{{ t('databasesRetention') }}</p>
          <div v-if="activeDatabase.retentionPolicies.length" class="mb-3 space-y-1">
            <div
              v-for="rp in activeDatabase.retentionPolicies"
              :key="rp.name"
              class="flex items-center gap-2 rounded border border-slate-200 bg-white px-3 py-1.5 dark:border-slate-700 dark:bg-slate-900"
            >
              <Clock class="h-3.5 w-3.5 text-slate-400 dark:text-slate-500" />
              <span class="text-sm font-medium text-slate-700 dark:text-slate-200">{{ rp.name }}</span>
              <span class="text-xs text-slate-500 dark:text-slate-400">{{ formatDuration(rp.duration) }}</span>
            </div>
          </div>
          <div v-if="isAdmin" class="flex flex-wrap items-center gap-2" data-testid="databases-rp-create">
            <input v-model="activeDatabase.newRpName" type="text" :placeholder="t('databasesRpNamePlaceholder')" class="w-28 rounded border border-slate-300 px-2 py-1 text-xs dark:border-slate-600" data-testid="databases-rp-name" />
            <input v-model="activeDatabase.newRpDuration" type="text" :placeholder="t('databasesRpDurationPlaceholder')" class="w-24 rounded border border-slate-300 px-2 py-1 text-xs dark:border-slate-600" data-testid="databases-rp-duration" />
            <button type="button" class="inline-flex items-center gap-1 rounded bg-slate-800 px-3 py-1 text-xs font-medium text-white" data-testid="databases-rp-add" @click="createRetentionPolicy(activeDatabase)">
              <Plus class="h-3.5 w-3.5" /> {{ t('databasesAdd') }}
            </button>
          </div>
        </div>
      </div>
    </div>
    <ConfirmDialog
      v-model:open="confirmOpen"
      :title="t('databasesDeleteDbBtnTitle')"
      :message="t('databasesDeleteDbMsg')"
      :require-text="confirmDbName"
      :confirm-label="t('delete')"
      danger
      :loading="confirmLoading"
      @confirm="confirmDeleteDatabase"
    />
  </div>
</template>
