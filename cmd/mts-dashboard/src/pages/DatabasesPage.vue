<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { apiGet, apiPost, apiDelete } from '@/api/client'
import { listDatabases, listMeasurements, listRetentionPolicies } from '@/api/meta'
import {
  Plus, Trash2, ChevronDown, ChevronRight, Table2, Tag, Clock, Download,
} from 'lucide-vue-next'
import { useAuth } from '@/composables/useAuth'
import PermissionDenied from '@/components/PermissionDenied.vue'
import ConfirmDialog from '@/components/ConfirmDialog.vue'
import ActionResultBanner from '@/components/ActionResultBanner.vue'
import EmptyState from '@/components/EmptyState.vue'
import ListSelectionToolbar from '@/components/ListSelectionToolbar.vue'
import { makeActionResult, type ActionResult } from '@/utils/actionResult'
import { useNotify } from '@/composables/useNotify'
import { formatCaughtError } from '@/utils/apiError'
import { formatRPDuration, mapRPDurationError, parseRPDurationToNs } from '@/utils/rpDuration'
import { formatMessage } from '@/utils/formatMessage'
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
interface SeriesResponse { series: Series[] }
interface MeasurementEntry {
  name: string
  expanded: boolean
  loading: boolean
  fields: FieldSchema[]
  series: Series[]
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
const { t } = useI18n()
const { success, error: notifyError, warn } = useNotify()
const databases = ref<DatabaseEntry[]>([])
const dbFilter = ref('')
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
  if (!isAdmin.value) return
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
    return
  }
  db.expanded = true
  if (!db.loaded) await loadDatabaseDetails(db)
}
async function toggleMeasurement(meas: MeasurementEntry, dbName: string) {
  meas.expanded = !meas.expanded
  if (meas.expanded && !meas.fields.length) {
    meas.loading = true
    try {
      const [fieldsData, seriesData] = await Promise.all([
        apiGet<FieldsResponse>(`/api/v1/data/databases/${encodeURIComponent(dbName)}/measurements/${encodeURIComponent(meas.name)}/fields`),
        apiGet<SeriesResponse>(`/api/v1/data/databases/${encodeURIComponent(dbName)}/measurements/${encodeURIComponent(meas.name)}/series`),
      ])
      meas.fields = fieldsData.fields ?? []
      meas.series = seriesData.series ?? []
    } catch (e) {
      const msg = formatCaughtError(e); actionResult.value = makeActionResult('error', msg)
    } finally {
      meas.loading = false
    }
  }
}
async function createDatabase() {
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
  <PermissionDenied v-if="!isAdmin" />
  <div v-else class="space-y-4" data-testid="databases-page">
    <ActionResultBanner v-if="loadError" kind="error" :message="loadError" @dismiss="loadError = ''" />
    <ActionResultBanner :result="actionResult" @dismiss="actionResult = null" />
    <div class="flex flex-wrap items-center justify-between gap-3">
      <div>
        <h1 class="text-lg font-semibold text-slate-800 dark:text-slate-100">{{ t('databases') }}</h1>
        <p class="text-xs mts-muted">{{ t('databasesDesc') }}</p>
      </div>
      <div class="flex flex-wrap gap-2">
        <button type="button" class="mts-btn" data-testid="databases-export-json" :disabled="!filteredDatabases.length" @click="exportJSON">
          <Download class="h-3.5 w-3.5" /> {{ t('inventoryExportJSON') }}
        </button>
        <button type="button" class="mts-btn" data-testid="databases-export-csv" :disabled="!filteredDatabases.length" @click="exportCSV">
          <Download class="h-3.5 w-3.5" /> {{ t('inventoryExportCSV') }}
        </button>
        <input v-model="newDbName" type="text" :placeholder="t('databasesCreatePlaceholder')" class="w-56 rounded-lg border border-slate-300 px-3 py-2 text-sm focus:border-slate-500 focus:outline-none dark:border-slate-600 dark:bg-slate-800" @keyup.enter="createDatabase" />
        <button class="inline-flex items-center gap-1 rounded-lg bg-slate-800 px-4 py-2 text-sm font-medium text-white hover:bg-slate-700" @click="createDatabase">
          <Plus class="h-4 w-4" /> {{ t('databasesCreate') }}
        </button>
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
        <template v-if="!databases.length" #action>
          <button type="button" class="mts-btn-primary" @click="createDatabase">{{ t('databasesCreate') }}</button>
        </template>
      </EmptyState>
    </div>

    <div v-else class="space-y-2">
<div v-for="db in filteredDatabases" :key="db.name" class="overflow-hidden rounded-xl border border-slate-200 bg-white dark:border-slate-700 dark:bg-slate-900">
        <div class="flex items-center justify-between px-4 py-3 hover:bg-slate-50 dark:hover:bg-slate-800">
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
          <button class="flex items-center gap-2 text-left" @click="toggleExpand(db)">
            <component :is="db.expanded ? ChevronDown : ChevronRight" class="h-4 w-4 text-slate-400 dark:text-slate-500" />
            <span class="text-sm font-medium text-slate-800 dark:text-slate-100">{{ db.name }}</span>
            <span v-if="db.loading" class="text-xs text-slate-400 dark:text-slate-500">{{ t('databasesLoading') }}</span>
          </button>
          </div>
          <button class="rounded p-1 text-slate-400 dark:text-slate-500 hover:text-red-600 dark:text-red-300" :title="t('databasesDeleteDbBtnTitle')" @click="requestDeleteDatabase(db.name)">
            <Trash2 class="h-4 w-4" />
          </button>
        </div>
        <div v-if="db.expanded && db.loaded" class="border-t border-slate-100">
          <div class="px-6 py-3">
            <p class="mb-2 text-xs font-semibold uppercase tracking-wide text-slate-500 dark:text-slate-400 dark:text-slate-500">{{ t('databasesMeasurements') }}</p>
            <EmptyState v-if="!db.measurements.length" compact :title="t('databasesNoMeasurement')" :description="t('databasesNoMeasurementDesc')" />
            <div v-for="meas in db.measurements" :key="meas.name" class="mb-2 rounded border border-slate-100">
              <button class="flex w-full items-center gap-2 px-3 py-2 text-left text-sm text-slate-700 dark:text-slate-200 hover:bg-slate-50 dark:hover:bg-slate-800" @click="toggleMeasurement(meas, db.name)">
                <component :is="meas.expanded ? ChevronDown : ChevronRight" class="h-3.5 w-3.5 text-slate-400 dark:text-slate-500" />
                <Table2 class="h-3.5 w-3.5 text-slate-400 dark:text-slate-500" />
                {{ meas.name }}
              </button>
              <div v-if="meas.expanded" class="border-t border-slate-50 px-4 py-2 text-xs text-slate-600 dark:text-slate-300">
                <div v-if="meas.loading" class="text-slate-400 dark:text-slate-500">{{ t('databasesLoading') }}</div>
                <template v-else>
                  <p class="mb-1 font-medium text-slate-500 dark:text-slate-400 dark:text-slate-500">{{ t('databasesFields') }}</p>
                  <div class="mb-2 flex flex-wrap gap-1">
                    <span v-for="f in meas.fields" :key="f.name" class="rounded bg-slate-100 dark:bg-slate-800 px-2 py-0.5">{{ f.name }}:{{ fieldTypeName(f.type) }}</span>
                    <span v-if="!meas.fields.length" class="text-slate-400 dark:text-slate-500">{{ t('databasesNone') }}</span>
                  </div>
                  <p class="mb-1 font-medium text-slate-500 dark:text-slate-400 dark:text-slate-500">{{ t('databasesSeries') }}</p>
                  <div class="space-y-1">
                    <div v-for="s in meas.series" :key="s.id" class="flex items-center gap-1">
                      <Tag class="h-3 w-3 text-slate-400 dark:text-slate-500" />
                      <span class="font-mono">{{ s.tags && Object.keys(s.tags).length ? JSON.stringify(s.tags) : '{}' }}</span>
                    </div>
                    <span v-if="!meas.series.length" class="text-slate-400 dark:text-slate-500">{{ t('databasesNone') }}</span>
                  </div>
                </template>
              </div>
            </div>
          </div>
          <div class="border-t border-slate-200 dark:border-slate-700 px-6 py-3">
            <p class="mb-2 text-xs font-semibold uppercase tracking-wide text-slate-500 dark:text-slate-400 dark:text-slate-500">{{ t('databasesRetention') }}</p>
            <div v-if="db.retentionPolicies.length" class="mb-3 space-y-1">
              <div v-for="rp in db.retentionPolicies" :key="rp.name" class="flex items-center gap-2 rounded border border-slate-200 bg-white dark:border-slate-700 dark:bg-slate-900 px-3 py-1.5">
                <Clock class="h-3.5 w-3.5 text-slate-400 dark:text-slate-500" />
                <span class="text-sm font-medium text-slate-700 dark:text-slate-200">{{ rp.name }}</span>
                <span class="text-xs text-slate-500 dark:text-slate-400 dark:text-slate-500">{{ formatDuration(rp.duration) }}</span>
              </div>
            </div>
            <div class="flex flex-wrap items-center gap-2">
              <input v-model="db.newRpName" type="text" :placeholder="t('databasesRpNamePlaceholder')" class="w-28 rounded border border-slate-300 dark:border-slate-600 px-2 py-1 text-xs" />
              <input v-model="db.newRpDuration" type="text" :placeholder="t('databasesRpDurationPlaceholder')" class="w-24 rounded border border-slate-300 dark:border-slate-600 px-2 py-1 text-xs" />
              <button class="inline-flex items-center gap-1 rounded bg-slate-800 px-3 py-1 text-xs font-medium text-white" @click="createRetentionPolicy(db)">
                <Plus class="h-3.5 w-3.5" /> {{ t('databasesAdd') }}
              </button>
            </div>
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
