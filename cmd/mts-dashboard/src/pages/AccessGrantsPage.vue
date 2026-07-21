<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { useHashScroll } from '@/composables/useHashScroll'
import { apiGet } from '@/api/client'
import { formatCaughtError } from '@/utils/apiError'
import { useI18n } from '@/composables/useI18n'
import { formatMessage } from '@/utils/formatMessage'
import { permissionLabel } from '@/utils/permissionLabel'
import { useAuth } from '@/composables/useAuth'
import PermissionDenied from '@/components/PermissionDenied.vue'
import ActionResultBanner from '@/components/ActionResultBanner.vue'
import EmptyState from '@/components/EmptyState.vue'
import ListSelectionToolbar from '@/components/ListSelectionToolbar.vue'
import VirtualTable from '@/components/VirtualTable.vue'
import {
  filterGrantRows,
  flattenUserGrants,
  grantCoverage,
  type GrantRow,
  type UserGrantBundle,
} from '@/utils/grantsSummary'
import { RefreshCw, ShieldCheck, Download } from 'lucide-vue-next'
import { useNotify } from '@/composables/useNotify'
import { buildGrantsExport, grantsToCSV } from '@/utils/grantsExport'
import { parseAccessGrantsPrefill, accessGrantsFormToPrefill } from '@/utils/routePrefill'
import { copyText } from '@/utils/clipboard'
import { stampFilename } from '@/utils/download'
import { useExportJob } from '@/composables/useExportJob'
import ExportJobBanner from '@/components/ExportJobBanner.vue'
import { filterRowsByIds } from '@/utils/listSelection'
import { useListSelection } from '@/composables/useListSelection'
import {
  cycleSortState,
  loadSortState,
  saveSortState,
  sortByAccessor,
  type SortState,
} from '@/utils/listSort'
import { grantRowId } from '@/utils/rowIds'

interface User {
  name: string
  role?: string
  disabled?: boolean
}
interface UsersResponse { users: User[] }
interface PermissionsResponse { grants: Array<{ database: string; permission: string }> }

const route = useRoute()
useHashScroll()
const { isAdmin } = useAuth()
const { t, locale } = useI18n()
const { success, info, warn, error: notifyError } = useNotify()
const {
  exportJob,
  exportBusy,
  cancelExport,
  resetExport,
  runTextExport,
  runJSONExport,
} = useExportJob()
function roleLabel(role?: string): string {
  if (!role) return t.value('emptyValue')
  if (role === 'admin') return t.value('roleAdmin')
  if (role === 'user') return t.value('roleUser')
  return role
}
function permText(p: string): string {
  return permissionLabel(p, locale.value === 'en' ? 'en' : 'zh')
}
const loading = ref(false)
const loadError = ref('')
const rows = ref<GrantRow[]>([])
const userFilter = ref('')
const dbFilter = ref('')
const permFilter = ref('')
const q = ref('')
const partialErrors = ref<string[]>([])

const users = computed(() => Array.from(new Set(rows.value.map((r) => r.user))).sort())
const databases = computed(() => Array.from(new Set(rows.value.map((r) => r.database))).sort())
const permissions = computed(() => Array.from(new Set(rows.value.map((r) => r.permission))).sort())

const GRANTS_SORT_KEY = 'mts.dashboard.access-grants-sort.prefs.v1'
const GRANTS_SORT_KEYS = ['user', 'role', 'status', 'database', 'permission'] as const
type GrantSortKey = (typeof GRANTS_SORT_KEYS)[number]
const storage = typeof localStorage !== 'undefined' ? localStorage : null
const grantSort = ref<SortState<GrantSortKey>>(
  loadSortState(storage, GRANTS_SORT_KEY, GRANTS_SORT_KEYS),
)

const filteredBase = computed(() =>
  filterGrantRows(rows.value, {
    user: userFilter.value,
    database: dbFilter.value,
    permission: permFilter.value,
    q: q.value,
  }),
)

const filtered = computed(() =>
  sortByAccessor(filteredBase.value, grantSort.value, {
    user: (r) => r.user,
    role: (r) => r.role || '',
    status: (r) => Boolean(r.disabled),
    database: (r) => r.database,
    permission: (r) => r.permission,
  }),
)

const visibleGrantIds = computed(() => filtered.value.map((r) => grantRowId(r)))
const GRANTS_ROW_HEIGHT = 48
const GRANTS_LIST_HEIGHT = 448
const {
  selectedCount,
  allVisibleSelected,
  someVisibleSelected,
  exportIds,
  isSelected,
  toggle,
  toggleAllVisible,
  clear: clearSelection,
  pruneTo,
} = useListSelection(visibleGrantIds)

const coverage = computed(() => grantCoverage(filtered.value))

function cycleGrantSort(key: GrantSortKey) {
  grantSort.value = cycleSortState(grantSort.value, key)
  saveSortState(storage, GRANTS_SORT_KEY, grantSort.value)
}

function grantSortIndicator(key: GrantSortKey): string {
  if (grantSort.value.key !== key) return ''
  return grantSort.value.dir === 'asc' ? '↑' : '↓'
}

function rowsForExport() {
  return filterRowsByIds(filtered.value, exportIds.value, (r) => grantRowId(r))
}

async function load() {
  if (!isAdmin.value) return
  loading.value = true
  loadError.value = ''
  partialErrors.value = []
  try {
    const list = await apiGet<UsersResponse>('/api/v1/users')
    const usersList = list.users ?? []
    const bundles: UserGrantBundle[] = []
    const errs: string[] = []
    // 并发拉取，限制并发数避免压垮 POC 单机
    const concurrency = 4
    let idx = 0
    async function worker() {
      while (idx < usersList.length) {
        const i = idx++
        const u = usersList[i]
        try {
          const data = await apiGet<PermissionsResponse>(
            `/api/v1/users/${encodeURIComponent(u.name)}/database-permissions`,
          )
          bundles.push({
            user: u.name,
            role: u.role,
            disabled: u.disabled,
            grants: data.grants ?? [],
          })
        } catch (e) {
          errs.push(`${u.name}: ${formatCaughtError(e)}`)
          bundles.push({ user: u.name, role: u.role, disabled: u.disabled, grants: [] })
        }
      }
    }
    await Promise.all(Array.from({ length: Math.min(concurrency, usersList.length || 1) }, () => worker()))
    rows.value = flattenUserGrants(bundles)
    partialErrors.value = errs
    pruneTo(rows.value.map((r) => grantRowId(r)))
  } catch (e) {
    loadError.value = formatCaughtError(e)
    rows.value = []
  } finally {
    loading.value = false
  }
}

async function exportJSON() {
  const list = rowsForExport()
  if (!list.length) {
    warn(t.value('accessExportEmpty'))
    return
  }
  if (exportBusy.value) return
  const outcome = await runJSONExport({
    label: 'JSON',
    filename: stampFilename('mts-access-grants', 'json'),
    total: list.length,
    build: async ({ isCancelled, progress }) => {
      progress(0, list.length)
      const chunk = 200
      for (let i = 0; i < list.length; i++) {
        if (isCancelled()) return null
        const done = i + 1
        if (done === list.length || done % chunk === 0) {
          progress(done, list.length)
          if (done < list.length) await new Promise((r) => setTimeout(r, 0))
        }
      }
      return buildGrantsExport(list)
    },
  })
  if (outcome === 'done') success(t.value('accessExported'))
  else if (outcome === 'cancelled') info(t.value('exportCancelledToast'))
  else if (outcome === 'error') notifyError(exportJob.value.error || t.value('failed'))
}

async function exportCSV() {
  const list = rowsForExport()
  if (!list.length) {
    warn(t.value('accessExportEmpty'))
    return
  }
  if (exportBusy.value) return
  const outcome = await runTextExport({
    label: 'CSV',
    filename: stampFilename('mts-access-grants', 'csv'),
    mime: 'text/csv;charset=utf-8',
    total: list.length,
    build: async ({ isCancelled, progress }) => {
      progress(0, list.length)
      const chunk = 200
      for (let i = 0; i < list.length; i++) {
        if (isCancelled()) return null
        const done = i + 1
        if (done === list.length || done % chunk === 0) {
          progress(done, list.length)
          if (done < list.length) await new Promise((r) => setTimeout(r, 0))
        }
      }
      return grantsToCSV(list)
    },
  })
  if (outcome === 'done') success(t.value('accessExported'))
  else if (outcome === 'cancelled') info(t.value('exportCancelledToast'))
  else if (outcome === 'error') notifyError(exportJob.value.error || t.value('failed'))
}

function applyAccessGrantsPrefillFromRoute() {
  if (!isAdmin.value) return
  const pre = parseAccessGrantsPrefill(route.query as Record<string, unknown>)
  let changed = false
  if (pre.user != null && userFilter.value !== pre.user) {
    userFilter.value = pre.user
    changed = true
  }
  if (pre.database != null && dbFilter.value !== pre.database) {
    dbFilter.value = pre.database
    changed = true
  }
  if (pre.permission != null && permFilter.value !== pre.permission) {
    permFilter.value = pre.permission
    changed = true
  }
  if (pre.q != null && q.value !== pre.q) {
    q.value = pre.q
    changed = true
  }
  if (changed) success(t.value('accessGrantsPrefillApplied'))
}

async function copyAccessGrantsShareLink() {
  const path = accessGrantsFormToPrefill({
    user: userFilter.value,
    database: dbFilter.value,
    permission: permFilter.value,
    q: q.value,
  }, { hash: '#access-grants-filters' })
  const url = `${window.location.origin}${path}`
  const res = await copyText(url)
  if (res.ok) success(t.value('accessGrantsShareCopied'))
  else notifyError(res.error || t.value('failed'))
}

onMounted(async () => {
  await load()
  applyAccessGrantsPrefillFromRoute()
})

watch(
  () => route.fullPath,
  (path, prev) => {
    if (!isAdmin.value) return
    if (prev != null && path !== prev) applyAccessGrantsPrefillFromRoute()
  },
)
</script>

<template>
  <PermissionDenied v-if="!isAdmin" />
  <div v-else class="space-y-4" data-testid="access-grants-page">
    <div class="flex flex-wrap items-start justify-between gap-3">
      <div>
        <h1 class="mts-title flex items-center gap-2">
          <ShieldCheck class="h-5 w-5" />
          {{ t('accessGrantsTitle') }}
        </h1>
        <p class="text-xs mts-muted">
          {{ t('accessGrantsDesc') }}
        </p>
      </div>
      <div class="flex flex-wrap gap-2">
        <ExportJobBanner class="w-full basis-full" :job="exportJob" @cancel="cancelExport" @dismiss="resetExport" />
        <button type="button" class="mts-btn" data-testid="access-grants-export-json" :disabled="exportBusy || !filtered.length" @click="exportJSON">
          <Download class="h-3.5 w-3.5" /> {{ t('accessExportJSON') }}
        </button>
        <button type="button" class="mts-btn" data-testid="access-grants-export-csv" :disabled="exportBusy || !filtered.length" @click="exportCSV">
          <Download class="h-3.5 w-3.5" /> {{ t('accessExportCSV') }}
        </button>
        <button type="button" class="mts-btn" data-testid="access-grants-refresh" :disabled="loading" :aria-busy="loading ? 'true' : undefined" @click="load">
          <RefreshCw class="h-3.5 w-3.5" :class="loading ? 'animate-spin' : ''" /> {{ t('refresh') }}
        </button>
        <button type="button" class="mts-btn" data-testid="access-grants-share-link" @click="copyAccessGrantsShareLink">
          {{ t('accessGrantsShareLink') }}
        </button>
      </div>
    </div>

    <ActionResultBanner v-if="loadError" kind="error" :message="loadError" @dismiss="loadError = ''" />
    <ActionResultBanner
      v-else-if="partialErrors.length"
      kind="warn"
      :message="formatMessage(t('accessGrantsPartialFail'), { summary: partialErrors.slice(0, 3).join('; ') + (partialErrors.length > 3 ? '…' : '') })"
      :dismissible="false"
    />

    <div class="grid gap-3 sm:grid-cols-3">
      <div class="mts-card p-3">
        <p class="text-xs mts-muted">{{ t('accessGrantsUsersFiltered') }}</p>
        <p class="text-xl font-semibold text-slate-800 dark:text-slate-100">{{ coverage.users }}</p>
      </div>
      <div class="mts-card p-3">
        <p class="text-xs mts-muted">{{ t('accessGrantsDatabases') }}</p>
        <p class="text-xl font-semibold text-slate-800 dark:text-slate-100">{{ coverage.databases }}</p>
      </div>
      <div class="mts-card p-3">
        <p class="text-xs mts-muted">{{ t('accessGrantsCount') }}</p>
        <p class="text-xl font-semibold text-slate-800 dark:text-slate-100">{{ coverage.grants }}</p>
      </div>
    </div>

    <div id="access-grants-filters" class="scroll-mt-20 flex flex-wrap items-end gap-2" data-testid="access-grants-filter-bar">
      <label class="text-xs mts-muted">{{ t('accessGrantsUser') }}
        <select v-model="userFilter" class="mts-input mt-1 w-auto min-w-[8rem] text-sm" data-testid="access-grants-user-filter">
          <option value="">{{ t('accessGrantsAll') }}</option>
          <option v-for="u in users" :key="u" :value="u">{{ u }}</option>
        </select>
      </label>
      <label class="text-xs mts-muted">{{ t('accessGrantsDatabase') }}
        <select v-model="dbFilter" class="mts-input mt-1 w-auto min-w-[8rem] text-sm" data-testid="access-grants-db-filter">
          <option value="">{{ t('accessGrantsAll') }}</option>
          <option v-for="d in databases" :key="d" :value="d">{{ d }}</option>
        </select>
      </label>
      <label class="text-xs mts-muted">{{ t('accessGrantsPermission') }}
        <select v-model="permFilter" class="mts-input mt-1 w-auto min-w-[8rem] text-sm" data-testid="access-grants-perm-filter">
          <option value="">{{ t('accessGrantsAll') }}</option>
          <option v-for="p in permissions" :key="p" :value="p">{{ permText(p) }}</option>
        </select>
      </label>
      <label class="text-xs mts-muted grow">{{ t('accessGrantsSearch') }}
        <input v-model="q" class="mts-input mt-1 text-sm" :placeholder="t('accessGrantsFilterPlaceholder')" data-testid="access-grants-search" />
      </label>
      <ListSelectionToolbar
        prefix="access-grants"
        :selected-count="selectedCount"
        :has-visible="!!filtered.length"
        @select-all="toggleAllVisible(true)"
        @clear="clearSelection"
      >
        <template #actions>
          <button type="button" class="mts-btn" data-testid="access-grants-sort-user" :title="t('listSortBy')" @click="cycleGrantSort('user')">{{ t('accessGrantsColUser') }} {{ grantSortIndicator('user') }}</button>
          <button type="button" class="mts-btn" data-testid="access-grants-sort-database" :title="t('listSortBy')" @click="cycleGrantSort('database')">{{ t('accessGrantsColDatabase') }} {{ grantSortIndicator('database') }}</button>
        </template>
      </ListSelectionToolbar>
    </div>

    <div v-if="!loading && !filtered.length" class="mts-card">
      <EmptyState
        :title="t('accessGrantsEmpty')"
        :description="t('accessGrantsEmptyDesc')"
      />
    </div>
        <div v-else class="mts-card overflow-hidden p-0" data-testid="access-grants-table-wrap">
      <div
        id="access-grants-table"
        class="min-w-[44rem] overflow-hidden"
        data-testid="access-grants-table"
      >
        <div
          class="grid grid-cols-[2.5rem_minmax(7rem,1fr)_minmax(5rem,0.7fr)_minmax(5rem,0.7fr)_minmax(7rem,1fr)_minmax(6rem,0.9fr)] border-b border-slate-200 bg-slate-50 text-left text-xs font-medium uppercase tracking-wide text-slate-500 dark:border-slate-700 dark:bg-slate-900/60 dark:text-slate-400"
          data-testid="access-grants-table-header"
        >
          <div class="px-3 py-2">
            <input
              type="checkbox"
              class="h-3.5 w-3.5"
              data-testid="access-grants-select-all-checkbox"
              :checked="allVisibleSelected"
              :indeterminate.prop="someVisibleSelected"
              :aria-label="t('listSelectAll')"
              @change="toggleAllVisible(($event.target as HTMLInputElement).checked)"
            />
          </div>
          <div class="px-3 py-2">
            <button type="button" class="mts-focus-ring inline-flex items-center gap-1 uppercase" data-testid="access-grants-sort-user-col" @click="cycleGrantSort('user')">
              {{ t('accessGrantsColUser') }} <span aria-hidden="true">{{ grantSortIndicator('user') }}</span>
            </button>
          </div>
          <div class="px-3 py-2">
            <button type="button" class="mts-focus-ring inline-flex items-center gap-1 uppercase" data-testid="access-grants-sort-role-col" @click="cycleGrantSort('role')">
              {{ t('accessGrantsColRole') }} <span aria-hidden="true">{{ grantSortIndicator('role') }}</span>
            </button>
          </div>
          <div class="px-3 py-2">
            <button type="button" class="mts-focus-ring inline-flex items-center gap-1 uppercase" data-testid="access-grants-sort-status-col" @click="cycleGrantSort('status')">
              {{ t('accessGrantsColStatus') }} <span aria-hidden="true">{{ grantSortIndicator('status') }}</span>
            </button>
          </div>
          <div class="px-3 py-2">
            <button type="button" class="mts-focus-ring inline-flex items-center gap-1 uppercase" data-testid="access-grants-sort-database-col" @click="cycleGrantSort('database')">
              {{ t('accessGrantsColDatabase') }} <span aria-hidden="true">{{ grantSortIndicator('database') }}</span>
            </button>
          </div>
          <div class="px-3 py-2">
            <button type="button" class="mts-focus-ring inline-flex items-center gap-1 uppercase" data-testid="access-grants-sort-permission-col" @click="cycleGrantSort('permission')">
              {{ t('accessGrantsColPermission') }} <span aria-hidden="true">{{ grantSortIndicator('permission') }}</span>
            </button>
          </div>
        </div>
        <VirtualTable
          :items="filtered"
          :row-height="GRANTS_ROW_HEIGHT"
          :height="GRANTS_LIST_HEIGHT"
          data-testid="access-grants-virtual-list"
        >
          <template #default="{ item: row }">
            <div
              class="grid h-full grid-cols-[2.5rem_minmax(7rem,1fr)_minmax(5rem,0.7fr)_minmax(5rem,0.7fr)_minmax(7rem,1fr)_minmax(6rem,0.9fr)] items-center border-b border-slate-100 dark:border-slate-800"
              :data-testid="`access-grants-row-${row.user}-${row.database}-${row.permission}`"
            >
              <div class="px-3">
                <input
                  type="checkbox"
                  class="h-3.5 w-3.5"
                  :data-testid="`access-grants-select-${row.user}-${row.database}-${row.permission}`"
                  :checked="isSelected(grantRowId(row))"
                  :aria-label="t('listSelectCol')"
                  @change="toggle(grantRowId(row), ($event.target as HTMLInputElement).checked)"
                />
              </div>
              <div class="truncate px-3 font-medium text-slate-800 dark:text-slate-100">{{ row.user }}</div>
              <div class="truncate px-3 text-slate-600 dark:text-slate-300">{{ roleLabel(row.role) }}</div>
              <div class="px-3">
                <span
                  class="inline-flex rounded-full px-2 py-0.5 text-xs"
                  :class="row.disabled
                    ? 'bg-rose-50 text-rose-800 dark:bg-rose-950/40 dark:text-rose-100'
                    : 'bg-emerald-50 text-emerald-800 dark:bg-emerald-950/40 dark:text-emerald-100'"
                >
                  {{ row.disabled ? t('accessGrantsDisabled') : t('accessGrantsEnabled') }}
                </span>
              </div>
              <div class="truncate px-3 font-mono text-xs">{{ row.database }}</div>
              <div class="px-3">
                <span class="inline-flex rounded-full bg-slate-100 px-2 py-0.5 text-xs text-slate-700 dark:bg-slate-800 dark:text-slate-200">
                  {{ permText(row.permission) }}
                </span>
              </div>
            </div>
          </template>
        </VirtualTable>
        <p class="border-t border-slate-100 px-3 py-1.5 text-[11px] mts-muted dark:border-slate-800" data-testid="access-grants-virtual-hint">
          {{ t('accessGrantsVirtualHint') }}
        </p>
      </div>
    </div>
  </div>
</template>
