<script setup lang="ts">
import { computed, ref, onMounted, watch } from 'vue'
import { useRoute } from 'vue-router'
import { useHashScroll } from '@/composables/useHashScroll'
import { useAuth } from '@/composables/useAuth'
import {
  ACCESS_LEVEL_LABEL,
  RBAC_CAPABILITY_MATRIX,
  countByLevel,
  levelForRole,
  matrixAreas,
  textForLocale,
  type AccessLevel,
  type LocaleCode,
  type RoleName,
} from '@/utils/rbacMatrix'
import { Download, Shield } from 'lucide-vue-next'
import ListSelectionToolbar from '@/components/ListSelectionToolbar.vue'
import VirtualTable from '@/components/VirtualTable.vue'
import { useI18n } from '@/composables/useI18n'
import { formatMessage } from '@/utils/formatMessage'
import { useNotify } from '@/composables/useNotify'
import { accessMatrixToCSV, buildAccessMatrixExport } from '@/utils/accessMatrixExport'
import { parseAccessPrefill, accessFormToPrefill } from '@/utils/routePrefill'
import { copyText } from '@/utils/clipboard'
import { stampFilename } from '@/utils/download'
import { useExportJob } from '@/composables/useExportJob'
import ExportJobBanner from '@/components/ExportJobBanner.vue'
import { filterAccessMatrixRows } from '@/utils/listFilter'
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

const route = useRoute()
useHashScroll()
const { currentRole } = useAuth()
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
const roleFilter = ref<'all' | RoleName>('all')
const areaFilter = ref('')
const textFilter = ref('')
const uiLocale = computed<LocaleCode>(() => (locale.value === 'en' ? 'en' : 'zh'))
const areas = computed(() =>
  matrixAreas().map((a) => ({ key: a.key, label: textForLocale(a.label, uiLocale.value) })),
)

const MATRIX_SORT_KEY = 'mts.dashboard.access-matrix-sort.prefs.v1'
const MATRIX_SORT_KEYS = ['area', 'capability', 'admin', 'user', 'route'] as const
type MatrixSortKey = (typeof MATRIX_SORT_KEYS)[number]
const storage = typeof localStorage !== 'undefined' ? localStorage : null
const matrixSort = ref<SortState<MatrixSortKey>>(
  loadSortState(storage, MATRIX_SORT_KEY, MATRIX_SORT_KEYS),
)

const displayRole = computed(() => (currentRole.value === 'admin' ? 'admin' : 'user') as RoleName)

const filteredRows = computed(() => {
  let list = RBAC_CAPABILITY_MATRIX.filter((r) => {
    if (areaFilter.value && r.areaKey !== areaFilter.value) return false
    if (roleFilter.value === 'all') return true
    return levelForRole(r, roleFilter.value) !== 'none'
  })
  list = filterAccessMatrixRows(list, textFilter.value, (v) => {
    if (v && typeof v === 'object' && v !== null && ('zh' in v || 'en' in v)) {
      const o = v as { zh?: string; en?: string }
      return uiLocale.value === 'en' ? String(o.en ?? o.zh ?? '') : String(o.zh ?? o.en ?? '')
    }
    return String(v ?? '')
  })
  return sortByAccessor(list, matrixSort.value, {
    area: (r) => textForLocale(r.area, uiLocale.value),
    capability: (r) => textForLocale(r.capability, uiLocale.value),
    admin: (r) => r.admin,
    user: (r) => r.user,
    route: (r) => r.route || '',
  })
})

const visibleIds = computed(() => filteredRows.value.map((r) => r.id))
const MATRIX_ROW_HEIGHT = 48
const MATRIX_LIST_HEIGHT = 448
const {
  selectedCount,
  allVisibleSelected,
  someVisibleSelected,
  exportIds,
  isSelected,
  toggle,
  toggleAllVisible,
  clear: clearSelection,
} = useListSelection(visibleIds)

const adminCounts = countByLevel('admin')
const userCounts = countByLevel('user')

function levelClass(level: AccessLevel): string {
  switch (level) {
    case 'full':
      return 'bg-emerald-50 text-emerald-800 dark:bg-emerald-950/50 dark:text-emerald-200'
    case 'self':
      return 'bg-sky-50 text-sky-800 dark:bg-sky-950/50 dark:text-sky-200'
    case 'data_scoped':
      return 'bg-amber-50 text-amber-900 dark:bg-amber-950/40 dark:text-amber-100'
    default:
      return 'bg-slate-100 text-slate-500 dark:bg-slate-800 dark:text-slate-400'
  }
}

function levelLabel(level: AccessLevel): string {
  return textForLocale(ACCESS_LEVEL_LABEL[level], uiLocale.value)
}
function roleLabel(role: string): string {
  if (role === 'admin') return t.value('roleAdmin')
  if (role === 'user') return t.value('roleUser')
  return role
}

function cycleMatrixSort(key: MatrixSortKey) {
  matrixSort.value = cycleSortState(matrixSort.value, key)
  saveSortState(storage, MATRIX_SORT_KEY, matrixSort.value)
}

function sortIndicator(key: MatrixSortKey): string {
  if (matrixSort.value.key !== key) return ''
  return matrixSort.value.dir === 'asc' ? '↑' : '↓'
}

function rowsForExport() {
  return filterRowsByIds(filteredRows.value, exportIds.value, (r) => r.id)
}

function applyAccessPrefillFromRoute() {
  const pre = parseAccessPrefill(route.query as Record<string, unknown>)
  let changed = false
  if (pre.role && roleFilter.value !== pre.role) {
    roleFilter.value = pre.role as 'all' | RoleName
    changed = true
  }
  if (pre.area != null && areaFilter.value !== pre.area) {
    areaFilter.value = pre.area
    changed = true
  }
  if (pre.q != null && textFilter.value !== pre.q) {
    textFilter.value = pre.q
    changed = true
  }
  if (changed) success(t.value('accessPrefillApplied'))
}

async function copyAccessShareLink() {
  const path = accessFormToPrefill({
    role: roleFilter.value === 'all' ? undefined : roleFilter.value,
    area: areaFilter.value,
    q: textFilter.value,
  }, { hash: '#access-matrix-filter-bar' })
  const url = `${window.location.origin}${path}`
  const res = await copyText(url)
  if (res.ok) success(t.value('accessShareCopied'))
  else notifyError(res.error || t.value('failed'))
}

onMounted(() => {
  applyAccessPrefillFromRoute()
})

watch(
  () => route.fullPath,
  (path, prev) => {
    if (prev != null && path !== prev) applyAccessPrefillFromRoute()
  },
)

async function exportMatrix() {
  const list = rowsForExport()
  if (!list.length) {
    warn(t.value('accessExportEmpty'))
    return
  }
  if (exportBusy.value) return
  const outcome = await runJSONExport({
    label: 'JSON',
    filename: stampFilename('mts-access-matrix', 'json'),
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
      return buildAccessMatrixExport(list, uiLocale.value)
    },
  })
  if (outcome === 'done') success(t.value('accessMatrixExported'))
  else if (outcome === 'cancelled') info(t.value('exportCancelledToast'))
  else if (outcome === 'error') notifyError(exportJob.value.error || t.value('failed'))
}

async function exportMatrixCSV() {
  const list = rowsForExport()
  if (!list.length) {
    warn(t.value('accessExportEmpty'))
    return
  }
  if (exportBusy.value) return
  const outcome = await runTextExport({
    label: 'CSV',
    filename: stampFilename('mts-access-matrix', 'csv'),
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
      return accessMatrixToCSV(list, uiLocale.value)
    },
  })
  if (outcome === 'done') success(t.value('accessMatrixExported'))
  else if (outcome === 'cancelled') info(t.value('exportCancelledToast'))
  else if (outcome === 'error') notifyError(exportJob.value.error || t.value('failed'))
}
</script>

<template>
  <div class="space-y-4" data-testid="access-matrix-page">
    <div class="flex flex-wrap items-start justify-between gap-3">
      <div>
        <h1 class="mts-title flex items-center gap-2">
          <Shield class="h-5 w-5" />
          {{ t('accessMatrixTitle') }}
        </h1>
        <p class="text-xs mts-muted">
          {{ t('accessMatrixDesc') }}
          <span class="font-medium text-slate-800 dark:text-slate-100">{{ roleLabel(displayRole) }}</span>
        </p>
      </div>
      <div class="flex flex-wrap gap-2">
        <ExportJobBanner class="w-full basis-full" :job="exportJob" @cancel="cancelExport" @dismiss="resetExport" />
        <button
          type="button"
          class="mts-btn"
          data-testid="access-matrix-export"
          :disabled="exportBusy || (!filteredRows.length)"
          @click="exportMatrix"
        >
          <Download class="h-3.5 w-3.5" /> {{ t('accessMatrixExport') }}
        </button>
        <button
          type="button"
          class="mts-btn"
          data-testid="access-matrix-export-csv"
          :disabled="exportBusy || (!filteredRows.length)"
          @click="exportMatrixCSV"
        >
          <Download class="h-3.5 w-3.5" /> {{ t('accessMatrixExportCSV') }}
        </button>
        <button type="button" class="mts-btn" data-testid="access-matrix-share-link" @click="copyAccessShareLink">
          {{ t('accessShareLink') }}
        </button>
      </div>
    </div>

    <div class="grid gap-3 sm:grid-cols-2">
      <div class="mts-card p-3">
        <p class="text-xs font-medium text-slate-700 dark:text-slate-200">{{ formatMessage(t('accessMatrixAdminDist'), { role: t('roleAdmin') }) }}</p>
        <p class="mt-1 text-xs mts-muted">
          {{ formatMessage(t('accessMatrixDistLine'), { full: adminCounts.full, self: adminCounts.self, data: adminCounts.data_scoped, none: adminCounts.none }) }}
        </p>
      </div>
      <div class="mts-card p-3">
        <p class="text-xs font-medium text-slate-700 dark:text-slate-200">{{ formatMessage(t('accessMatrixUserDist'), { role: t('roleUser') }) }}</p>
        <p class="mt-1 text-xs mts-muted">
          {{ formatMessage(t('accessMatrixDistLine'), { full: userCounts.full, self: userCounts.self, data: userCounts.data_scoped, none: userCounts.none }) }}
        </p>
      </div>
    </div>

    <div id="access-matrix-filter-bar" class="scroll-mt-20 flex flex-wrap items-end gap-2" data-testid="access-matrix-filter-bar">
      <label class="text-xs mts-muted">{{ t('accessMatrixRoleFilter') }}
        <select v-model="roleFilter" class="mts-input mt-1 w-auto text-sm" data-testid="access-matrix-role-filter">
          <option value="all">{{ t('accessMatrixAllRows') }}</option>
          <option value="admin">{{ formatMessage(t('accessMatrixAdminOnly'), { role: t('roleAdmin') }) }}</option>
          <option value="user">{{ formatMessage(t('accessMatrixUserOnly'), { role: t('roleUser') }) }}</option>
        </select>
      </label>
      <label class="text-xs mts-muted">{{ t('accessMatrixArea') }}
        <select v-model="areaFilter" class="mts-input mt-1 w-auto text-sm" data-testid="access-matrix-area-filter">
          <option value="">{{ t('accessMatrixAllAreas') }}</option>
          <option v-for="a in areas" :key="a.key" :value="a.key">{{ a.label }}</option>
        </select>
      </label>
      <label class="text-xs mts-muted grow">{{ t('accessMatrixSearch') }}
        <input
          v-model="textFilter"
          type="search"
          class="mts-input mt-1 text-sm"
          data-testid="access-matrix-search"
          :placeholder="t('accessMatrixSearchPlaceholder')"
        />
      </label>
      <span class="text-xs mts-muted" data-testid="access-matrix-filter-count">{{ filteredRows.length }} / {{ RBAC_CAPABILITY_MATRIX.length }}</span>
      <ListSelectionToolbar
        prefix="access-matrix"
        :selected-count="selectedCount"
        :has-visible="!!filteredRows.length"
        @select-all="toggleAllVisible(true)"
        @clear="clearSelection"
      >
        <template #actions>
          <button type="button" class="mts-btn" data-testid="access-matrix-sort-area" :title="t('listSortBy')" @click="cycleMatrixSort('area')" :aria-sort="ariaSortValue(matrixSort, 'area')">{{ t('accessMatrixColArea') }} {{ sortIndicator('area') }}</button>
          <button type="button" class="mts-btn" data-testid="access-matrix-sort-capability" :title="t('listSortBy')" @click="cycleMatrixSort('capability')" :aria-sort="ariaSortValue(matrixSort, 'capability')">{{ t('accessMatrixColCapability') }} {{ sortIndicator('capability') }}</button>
        </template>
      </ListSelectionToolbar>
    </div>

        <div class="mts-card overflow-hidden p-0" data-testid="access-matrix-table-wrap">
      <div
        id="access-matrix-table"
        class="min-w-[52rem] overflow-hidden"
        data-testid="access-matrix-table"
      >
        <div
          class="grid grid-cols-[2.5rem_minmax(5rem,0.8fr)_minmax(8rem,1.2fr)_minmax(5rem,0.7fr)_minmax(5rem,0.7fr)_minmax(7rem,1fr)_minmax(7rem,1.1fr)] border-b border-slate-200 bg-slate-50 text-left text-xs font-medium uppercase tracking-wide text-slate-500 dark:border-slate-700 dark:bg-slate-900/60 dark:text-slate-400"
          data-testid="access-matrix-table-header"
        >
          <div class="px-3 py-2">
            <input
              type="checkbox"
              class="h-3.5 w-3.5"
              data-testid="access-matrix-select-all-checkbox"
              :checked="allVisibleSelected"
              :indeterminate.prop="someVisibleSelected"
              :aria-label="t('listSelectAll')"
              @change="toggleAllVisible(($event.target as HTMLInputElement).checked)"
            />
          </div>
          <div class="px-3 py-2">
            <button type="button" class="mts-focus-ring inline-flex items-center gap-1 uppercase" data-testid="access-matrix-sort-area-col" @click="cycleMatrixSort('area')" :aria-sort="ariaSortValue(matrixSort, 'area')">
              {{ t('accessMatrixColArea') }} <span aria-hidden="true">{{ sortIndicator('area') }}</span>
            </button>
          </div>
          <div class="px-3 py-2">
            <button type="button" class="mts-focus-ring inline-flex items-center gap-1 uppercase" data-testid="access-matrix-sort-capability-col" @click="cycleMatrixSort('capability')" :aria-sort="ariaSortValue(matrixSort, 'capability')">
              {{ t('accessMatrixColCapability') }} <span aria-hidden="true">{{ sortIndicator('capability') }}</span>
            </button>
          </div>
          <div class="px-3 py-2">
            <button type="button" class="mts-focus-ring inline-flex items-center gap-1 uppercase" data-testid="access-matrix-sort-admin-col" @click="cycleMatrixSort('admin')" :aria-sort="ariaSortValue(matrixSort, 'admin')">
              {{ t('roleAdmin') }} <span aria-hidden="true">{{ sortIndicator('admin') }}</span>
            </button>
          </div>
          <div class="px-3 py-2">
            <button type="button" class="mts-focus-ring inline-flex items-center gap-1 uppercase" data-testid="access-matrix-sort-user-col" @click="cycleMatrixSort('user')" :aria-sort="ariaSortValue(matrixSort, 'user')">
              {{ t('roleUser') }} <span aria-hidden="true">{{ sortIndicator('user') }}</span>
            </button>
          </div>
          <div class="px-3 py-2">
            <button type="button" class="mts-focus-ring inline-flex items-center gap-1 uppercase" data-testid="access-matrix-sort-route-col" @click="cycleMatrixSort('route')" :aria-sort="ariaSortValue(matrixSort, 'route')">
              {{ t('accessMatrixColRoute') }} <span aria-hidden="true">{{ sortIndicator('route') }}</span>
            </button>
          </div>
          <div class="px-3 py-2">{{ t('accessMatrixColNote') }}</div>
        </div>
        <div v-if="!filteredRows.length" class="px-3 py-8 text-center text-sm mts-muted" data-testid="access-matrix-empty">
          {{ t('accessMatrixFilterEmpty') }}
        </div>
        <VirtualTable
          v-else
          :items="filteredRows"
          :row-height="MATRIX_ROW_HEIGHT"
          :height="MATRIX_LIST_HEIGHT"
          data-testid="access-matrix-virtual-list"
        >
          <template #default="{ item: row }">
            <div
              class="grid h-full grid-cols-[2.5rem_minmax(5rem,0.8fr)_minmax(8rem,1.2fr)_minmax(5rem,0.7fr)_minmax(5rem,0.7fr)_minmax(7rem,1fr)_minmax(7rem,1.1fr)] items-center border-b border-slate-100 dark:border-slate-800"
              :data-testid="`access-matrix-row-${row.id}`"
            >
              <div class="px-3">
                <input
                  type="checkbox"
                  class="h-3.5 w-3.5"
                  :data-testid="`access-matrix-select-${row.id}`"
                  :checked="isSelected(row.id)"
                  :aria-label="t('listSelectCol')"
                  @change="toggle(row.id, ($event.target as HTMLInputElement).checked)"
                />
              </div>
              <div class="truncate px-3 text-slate-700 dark:text-slate-200">{{ textForLocale(row.area, uiLocale) }}</div>
              <div class="truncate px-3 text-slate-800 dark:text-slate-100">{{ textForLocale(row.capability, uiLocale) }}</div>
              <div class="px-3">
                <span class="inline-flex rounded-full px-2 py-0.5 text-xs" :class="levelClass(row.admin)">{{ levelLabel(row.admin) }}</span>
              </div>
              <div class="px-3">
                <span class="inline-flex rounded-full px-2 py-0.5 text-xs" :class="levelClass(row.user)">{{ levelLabel(row.user) }}</span>
              </div>
              <div class="truncate px-3 font-mono text-xs mts-muted">{{ row.route || t('emptyValue') }}</div>
              <div class="truncate px-3 text-xs mts-muted" :title="row.notes ? textForLocale(row.notes, uiLocale) : ''">
                {{ row.notes ? textForLocale(row.notes, uiLocale) : t('emptyValue') }}
              </div>
            </div>
          </template>
        </VirtualTable>
        <p
          v-if="filteredRows.length"
          class="border-t border-slate-100 px-3 py-1.5 text-[11px] mts-muted dark:border-slate-800"
          data-testid="access-matrix-virtual-hint"
        >
          {{ t('accessMatrixVirtualHint') }}
        </p>
      </div>
    </div>
  </div>
</template>
