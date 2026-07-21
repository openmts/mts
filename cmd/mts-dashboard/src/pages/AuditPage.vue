<script setup lang="ts">
import { ref, onMounted, computed, watch } from 'vue'
import { useRoute } from 'vue-router'
import { useHashScroll } from '@/composables/useHashScroll'
import EmptyState from '@/components/EmptyState.vue'
import ListSelectionToolbar from '@/components/ListSelectionToolbar.vue'
import VirtualTable from '@/components/VirtualTable.vue'
import ActionResultBanner from '@/components/ActionResultBanner.vue'
import PartialErrorBanner from '@/components/PartialErrorBanner.vue'
import { apiGet } from '@/api/client'
import { useAuth } from '@/composables/useAuth'
import { useI18n } from '@/composables/useI18n'
import type { MessageKey } from '@/i18n/messages'
import { useNotify } from '@/composables/useNotify'
import { formatCaughtError } from '@/utils/apiError'
import {
  auditRangeToLocalInputs,
  filterAuditEvents,
  type AuditQuickRange,
} from '@/utils/commandPalette'
import { auditLimitOptions, buildAuditQueryString } from '@/utils/auditQuery'
import { parseAuditPrefill, auditFormToPrefill, type PrefillTimeRange, isPrefillTimeRange } from '@/utils/routePrefill'
import { copyText } from '@/utils/clipboard'
import { AUDIT_CSV_HEADER, auditEventToCSVLine, auditEventsToCSV } from '@/utils/auditExport'
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
import { auditRowId } from '@/utils/rowIds'
import { formatMessage } from '@/utils/formatMessage'
import { stampFilename } from '@/utils/download'
import { useExportJob } from '@/composables/useExportJob'
import ExportJobBanner from '@/components/ExportJobBanner.vue'
import { ScrollText, Download, RefreshCw, Eraser } from 'lucide-vue-next'

interface User { name: string; display_name?: string }
interface UsersResponse { users: User[] }
interface AuditEvent {
  time: string
  user_name: string
  action: string
  database?: string
  detail?: string
}
interface AuditResponse { events: AuditEvent[]; total?: number }

useHashScroll()
const route = useRoute()
const { isAdmin, currentUser } = useAuth()
const { t } = useI18n()
const { success, info, error: notifyError, warn } = useNotify()
const {
  exportJob,
  exportBusy,
  cancelExport,
  resetExport,
  runTextExport,
  runJSONExport,
} = useExportJob()
const users = ref<User[]>([])
const selectedUser = ref('')
const actionFilter = ref('')
const lastQuickRange = ref<PrefillTimeRange | ''>('')
const sinceLocal = ref('')
const untilLocal = ref('')
const clientQuery = ref('')
const limit = ref(500)
const serverTotal = ref<number | null>(null)
const auditEvents = ref<AuditEvent[]>([])
const loading = ref(false)
const loadError = ref('')
const usersLoadError = ref('')

const AUDIT_SORT_KEY = 'mts.dashboard.audit-sort.prefs.v1'
const AUDIT_SORT_KEYS = ['time', 'user', 'action', 'database'] as const
type AuditSortKey = (typeof AUDIT_SORT_KEYS)[number]
const storage = typeof localStorage !== 'undefined' ? localStorage : null
const auditSort = ref<SortState<AuditSortKey>>(
  loadSortState(storage, AUDIT_SORT_KEY, AUDIT_SORT_KEYS),
)

const filteredEvents = computed(() =>
  filterAuditEvents(auditEvents.value, clientQuery.value),
)

const displayedEvents = computed(() =>
  sortByAccessor(filteredEvents.value, auditSort.value, {
    time: (e) => e.time || '',
    user: (e) => e.user_name || '',
    action: (e) => e.action || '',
    database: (e) => e.database || '',
  }),
)

/** 带稳定序号，供虚拟列表与选择 id 复用 */
const displayedRows = computed(() =>
  displayedEvents.value.map((evt, idx) => ({
    evt,
    idx,
    id: auditRowId(evt, idx),
  })),
)

const AUDIT_ROW_HEIGHT = 44
const AUDIT_LIST_HEIGHT = 448

const visibleAuditIds = computed(() =>
  displayedRows.value.map((r) => r.id),
)
const {
  selectedCount,
  allVisibleSelected,
  someVisibleSelected,
  exportIds,
  isSelected,
  toggle,
  toggleAllVisible,
  clear: clearSelection,
} = useListSelection(visibleAuditIds)

const filteredCount = computed(() => displayedEvents.value.length)

function cycleAuditSort(key: AuditSortKey) {
  auditSort.value = cycleSortState(auditSort.value, key)
  saveSortState(storage, AUDIT_SORT_KEY, auditSort.value)
}

function auditSortIndicator(key: AuditSortKey): string {
  if (auditSort.value.key !== key) return ''
  return auditSort.value.dir === 'asc' ? '↑' : '↓'
}

function rowsForExport(): AuditEvent[] {
  const withId = displayedRows.value.map((r) => ({ row: r.evt, id: r.id }))
  const picked = filterRowsByIds(withId, exportIds.value, (x) => x.id)
  return picked.map((x) => x.row)
}

const quickRanges: { id: AuditQuickRange; labelKey: MessageKey }[] = [
  { id: '1h', labelKey: 'auditRange1h' },
  { id: '24h', labelKey: 'auditRange24h' },
  { id: '7d', labelKey: 'auditRange7d' },
  { id: '30d', labelKey: 'auditRange30d' },
]

async function loadUsersForFilter() {
  if (!isAdmin.value) return
  usersLoadError.value = ''
  try {
    const data = await apiGet<UsersResponse>('/api/v1/users')
    users.value = data.users ?? []
  } catch (e) {
    users.value = []
    usersLoadError.value = formatCaughtError(e)
  }
}

onMounted(async () => {
  if (isAdmin.value) {
    await loadUsersForFilter()
  } else if (currentUser.value) {
    selectedUser.value = currentUser.value
  }
  applyAuditPrefillFromRoute({ reload: false })
  await loadAudit()
})

watch(limit, () => {
  void loadAudit()
})


function toUnix(local: string): number | undefined {
  if (!local) return undefined
  const ms = Date.parse(local)
  if (Number.isNaN(ms)) return undefined
  return Math.floor(ms / 1000)
}

async function loadAudit() {
  loading.value = true
  loadError.value = ''
  try {
    if (!isAdmin.value) {
      const name = (currentUser.value || selectedUser.value || '').trim()
      if (!name) {
        auditEvents.value = []
        serverTotal.value = null
        loadError.value = t.value('auditSelfNeedUser')
        return
      }
      selectedUser.value = name
      const since = toUnix(sinceLocal.value)
      const until = toUnix(untilLocal.value)
      const qs = buildAuditQueryString({
        action: actionFilter.value,
        sinceUnix: since,
        untilUnix: until,
        limit: limit.value,
      })
      // 自身审计：服务端 action/since/until/limit；user_name 由路径固定
      const data = await apiGet<AuditResponse>(`/api/v1/users/${encodeURIComponent(name)}/audit?${qs}`)
      auditEvents.value = data.events ?? []
      serverTotal.value = (data.events ?? []).length
      clearSelection()
      return
    }
    const since = toUnix(sinceLocal.value)
    const until = toUnix(untilLocal.value)
    const qs = buildAuditQueryString({
      userName: selectedUser.value || undefined,
      action: actionFilter.value,
      sinceUnix: since,
      untilUnix: until,
      limit: limit.value,
    })
    const data = await apiGet<AuditResponse>(`/api/v1/admin/audit?${qs}`)
    auditEvents.value = data.events ?? []
    serverTotal.value = typeof data.total === 'number' ? data.total : (data.events ?? []).length
    clearSelection()
  } catch (e) {
    loadError.value = formatCaughtError(e)
    auditEvents.value = []
    serverTotal.value = null
    notifyError(loadError.value)
  } finally {
    loading.value = false
  }
}

function applyQuickRange(range: AuditQuickRange) {
  const r = auditRangeToLocalInputs(range)
  sinceLocal.value = r.since
  untilLocal.value = r.until
  lastQuickRange.value = isPrefillTimeRange(range) ? range : ''
  void loadAudit()
}

/** 深链只读预填：筛选条件，不自动导出 */
function applyAuditPrefillFromRoute(opts?: { reload?: boolean }) {
  const pre = parseAuditPrefill(route.query as Record<string, unknown>)
  let changed = false
  if (pre.range) {
    const r = auditRangeToLocalInputs(pre.range)
    if (sinceLocal.value !== r.since || untilLocal.value !== r.until) {
      sinceLocal.value = r.since
      untilLocal.value = r.until
      changed = true
    }
    lastQuickRange.value = pre.range
  }
  if (pre.action != null && actionFilter.value !== pre.action) {
    actionFilter.value = pre.action
    changed = true
  }
  if (pre.q != null && clientQuery.value !== pre.q) {
    clientQuery.value = pre.q
    changed = true
  }
  if (pre.user != null) {
    const nextUser = isAdmin.value ? pre.user : (currentUser.value || selectedUser.value || pre.user)
    if (selectedUser.value !== nextUser) {
      selectedUser.value = nextUser
      changed = true
    }
  }
  if (changed) {
    success(t.value('auditPrefillApplied'))
    if (opts?.reload !== false) void loadAudit()
  }
}

function clearFilters() {
  selectedUser.value = isAdmin.value ? '' : (currentUser.value || selectedUser.value || '')
  actionFilter.value = ''
  sinceLocal.value = ''
  untilLocal.value = ''
  clientQuery.value = ''
  lastQuickRange.value = ''
  clearSelection()
  void loadAudit()
}

async function copyAuditShareLink() {
  // 相对时间快捷优先；无 since/until 时仅 action/user/q
  const range = lastQuickRange.value && isPrefillTimeRange(lastQuickRange.value) ? lastQuickRange.value : undefined
  const path = auditFormToPrefill({
    range,
    action: actionFilter.value,
    q: clientQuery.value,
    user: isAdmin.value ? selectedUser.value : (currentUser.value || selectedUser.value),
  }, { hash: '#audit-filters' })
  const url = `${window.location.origin}${path}`
  const res = await copyText(url)
  if (res.ok) success(t.value('auditShareCopied'))
  else notifyError(res.error || t.value('failed'))
}

async function exportJSON() {
  const rows = rowsForExport()
  if (!rows.length) {
    warn(t.value('auditExportEmpty'))
    return
  }
  if (exportBusy.value) return
  const outcome = await runJSONExport({
    label: 'JSON',
    filename: stampFilename('mts-audit', 'json'),
    total: rows.length,
    build: async ({ isCancelled, progress }) => {
      progress(0, rows.length)
      const chunk = 400
      const out: typeof rows = []
      for (let i = 0; i < rows.length; i++) {
        if (isCancelled()) return null
        out.push(rows[i])
        const done = i + 1
        if (done === rows.length || done % chunk === 0) {
          progress(done, rows.length)
          if (done < rows.length) await new Promise((r) => setTimeout(r, 0))
        }
      }
      return out
    },
  })
  if (outcome === 'done') success(t.value('auditExportJSONOk'))
  else if (outcome === 'cancelled') info(t.value('exportCancelledToast'))
  else if (outcome === 'error') notifyError(exportJob.value.error || t.value('failed'))
}

async function exportCSV() {
  const rows = rowsForExport()
  if (!rows.length) {
    warn(t.value('auditExportEmpty'))
    return
  }
  if (exportBusy.value) return
  const outcome = await runTextExport({
    label: 'CSV',
    filename: stampFilename('mts-audit', 'csv'),
    mime: 'text/csv;charset=utf-8',
    total: rows.length,
    build: async ({ isCancelled, progress }) => {
      const lines = [AUDIT_CSV_HEADER]
      progress(0, rows.length)
      const chunk = 400
      for (let i = 0; i < rows.length; i++) {
        if (isCancelled()) return null
        lines.push(auditEventToCSVLine(rows[i]))
        const done = i + 1
        if (done === rows.length || done % chunk === 0) {
          progress(done, rows.length)
          if (done < rows.length) await new Promise((r) => setTimeout(r, 0))
        }
      }
      return lines.join('\n')
    },
  })
  if (outcome === 'done') success(t.value('auditExportCSVOk'))
  else if (outcome === 'cancelled') info(t.value('exportCancelledToast'))
  else if (outcome === 'error') notifyError(exportJob.value.error || t.value('failed'))
}

watch(
  () => route.fullPath,
  (path, prev) => {
    if (path === prev) return
    // 仅 query/hash 变化时应用预填；首次由 onMounted 处理
    if (prev != null) applyAuditPrefillFromRoute()
  },
)
</script>

<template>
  <div class="space-y-6" data-testid="audit-page">
    <div>
      <h1 class="mts-title flex items-center gap-2">
        <ScrollText class="h-5 w-5" />
        {{ t('auditTitle') }}
      </h1>
      <p class="text-xs mts-muted">{{ isAdmin ? t('auditDesc') : t('auditSelfDesc') }}</p>
    </div>
    <p
      v-if="!isAdmin"
      class="rounded-lg border border-amber-200 bg-amber-50 px-3 py-2 text-xs text-amber-900 dark:border-amber-900 dark:bg-amber-950/40 dark:text-amber-100"
      data-testid="audit-self-hint"
    >{{ t('auditSelfHint') }}</p>

    <ActionResultBanner v-if="loadError" kind="error" :message="loadError" retryable data-testid="audit-load-error" @retry="loadAudit" @dismiss="loadError = ''" />
    <PartialErrorBanner
      v-else-if="usersLoadError"
      :message="`${t('auditUsersLoadFailed')}：${usersLoadError}`"
      test-id="audit-users-load-error"
      @retry="loadUsersForFilter"
      @dismiss="usersLoadError = ''"
    />
    <p class="mts-alert-warn" role="note">{{ t('auditHint') }}</p>

    <div id="audit-filters" class="scroll-mt-20 flex flex-wrap gap-2" data-testid="audit-quick-ranges">
      <button
        v-for="r in quickRanges"
        :key="r.id"
        type="button"
        class="mts-btn"
        :data-testid="`audit-range-${r.id}`"
        @click="applyQuickRange(r.id)"
      >
        {{ t(r.labelKey as MessageKey) }}
      </button>
      <button type="button" class="mts-btn" data-testid="audit-clear-filters" @click="clearFilters">
        <Eraser class="h-3.5 w-3.5" />
        {{ t('auditClearFilters') }}
      </button>
    </div>

    <div class="grid gap-3 rounded-xl border border-slate-200 bg-white p-4 dark:border-slate-700 dark:bg-slate-900 md:grid-cols-6">
      <label class="text-xs mts-muted">{{ t('user') }}
        <select v-model="selectedUser" class="mts-input mt-1" data-testid="audit-user" :disabled="!isAdmin">
          <option v-if="isAdmin" value="">{{ t('auditAllUsers') }}</option>
          <option v-if="!isAdmin && currentUser" :value="currentUser">{{ currentUser }}</option>
          <option v-for="user in users" :key="user.name" :value="user.name">{{ user.display_name || user.name }}</option>
        </select>
      </label>
      <label class="text-xs mts-muted">{{ t('action') }}
        <input
          v-model="actionFilter"
          :placeholder="t('auditActionPlaceholder')"
          class="mts-input mt-1"
          data-testid="audit-action"
        />
      </label>
      <label class="text-xs mts-muted">{{ t('since') }}
        <input v-model="sinceLocal" type="datetime-local" class="mts-input mt-1" data-testid="audit-since" />
      </label>
      <label class="text-xs mts-muted">{{ t('until') }}
        <input v-model="untilLocal" type="datetime-local" class="mts-input mt-1" data-testid="audit-until" />
      </label>
      <label class="text-xs mts-muted">{{ t('auditClientFilter') }}
        <input
          v-model="clientQuery"
          :placeholder="t('auditClientFilterPlaceholder')"
          class="mts-input mt-1"
          data-testid="audit-client-filter"
        />
      </label>
      <label class="text-xs mts-muted">{{ t('auditLimit') }}
        <select v-model.number="limit" class="mts-input mt-1" data-testid="audit-limit">
          <option v-for="n in auditLimitOptions()" :key="n" :value="n">{{ n }}</option>
        </select>
      </label>
      <div class="md:col-span-6 space-y-2">
        <ExportJobBanner :job="exportJob" @cancel="cancelExport" @dismiss="resetExport" />
        <div class="flex flex-wrap items-end gap-2">
          <button type="button" :disabled="loading" class="mts-btn-primary" data-testid="audit-reload" @click="loadAudit">
            <RefreshCw class="h-3.5 w-3.5" />
            {{ loading ? t('loading') : t('filter') }}
          </button>
          <button type="button" class="mts-btn" data-testid="audit-export-json" :disabled="exportBusy" @click="exportJSON">
            <Download class="h-3.5 w-3.5" />
            {{ t('exportJSON') }}
          </button>
          <button type="button" class="mts-btn" data-testid="audit-export-csv" :disabled="exportBusy" @click="exportCSV">
            <Download class="h-3.5 w-3.5" />
            {{ t('exportCSV') }}
          </button>
          <button type="button" class="mts-btn" data-testid="audit-share-link" @click="copyAuditShareLink">
            {{ t('auditShareLink') }}
          </button>
        </div>
      </div>
    </div>

    <div class="rounded-xl border border-slate-200 bg-white dark:border-slate-700 dark:bg-slate-900">
      <div class="border-b border-slate-100 px-4 py-2 dark:border-slate-800">
        <ListSelectionToolbar
          prefix="audit"
          :selected-count="selectedCount"
          :has-visible="!!displayedEvents.length"
          @select-all="toggleAllVisible(true)"
          @clear="clearSelection"
        >
          <template #actions>
            <button type="button" class="mts-btn" data-testid="audit-sort-time" :title="t('listSortBy')" @click="cycleAuditSort('time')" :aria-sort="ariaSortValue(auditSort, 'time')">{{ t('auditColTime') }} {{ auditSortIndicator('time') }}</button>
            <button type="button" class="mts-btn" data-testid="audit-sort-user" :title="t('listSortBy')" @click="cycleAuditSort('user')" :aria-sort="ariaSortValue(auditSort, 'user')">{{ t('user') }} {{ auditSortIndicator('user') }}</button>
          </template>
        </ListSelectionToolbar>
      </div>
      <div class="flex items-center justify-between border-b border-slate-100 px-4 py-2 text-xs mts-muted dark:border-slate-800">
        <span class="inline-flex items-center gap-1"><ScrollText class="h-3.5 w-3.5" /> {{ t('auditEvents') }}</span>
        <span class="inline-flex flex-wrap items-center gap-2">
          <span data-testid="audit-count">{{ filteredCount }} / {{ auditEvents.length }}</span>
          <span v-if="serverTotal != null" class="text-[11px]" data-testid="audit-total">{{ t('auditTotal') }}: {{ serverTotal }}</span>
          <span class="text-[11px]" data-testid="audit-merged-hint">{{ t('auditMergedHint') }}</span>
        </span>
      </div>
      <div class="overflow-x-auto px-2 pb-2 pt-1 sm:px-3">
        <div
          id="audit-table"
          class="scroll-mt-20 overflow-hidden rounded-lg border border-slate-100 dark:border-slate-800"
          data-testid="audit-table"
        >
          <div
            class="grid grid-cols-[2.5rem_minmax(9rem,1.1fr)_minmax(6rem,0.8fr)_minmax(6rem,0.8fr)_minmax(6rem,0.7fr)_minmax(8rem,1.4fr)] gap-0 border-b border-slate-100 bg-white text-left dark:border-slate-800 dark:bg-slate-900"
            data-testid="audit-table-header"
          >
            <div class="sticky top-0 z-[1] px-3 py-3">
              <input
                type="checkbox"
                class="h-3.5 w-3.5"
                data-testid="audit-select-all-checkbox"
                :checked="allVisibleSelected"
                :indeterminate.prop="someVisibleSelected && !allVisibleSelected"
                :aria-label="t('listSelectAll')"
                @change="toggleAllVisible(($event.target as HTMLInputElement).checked)"
              />
            </div>
            <div class="sticky top-0 z-[1] px-4 py-3 text-xs font-medium mts-muted">
              <button type="button" class="mts-focus-ring inline-flex items-center gap-1" data-testid="audit-sort-time-col" @click="cycleAuditSort('time')" :aria-sort="ariaSortValue(auditSort, 'time')">
                {{ t('auditColTime') }} <span aria-hidden="true">{{ auditSortIndicator('time') }}</span>
              </button>
            </div>
            <div class="sticky top-0 z-[1] px-4 py-3 text-xs font-medium mts-muted">
              <button type="button" class="mts-focus-ring inline-flex items-center gap-1" data-testid="audit-sort-user-col" @click="cycleAuditSort('user')" :aria-sort="ariaSortValue(auditSort, 'user')">
                {{ t('user') }} <span aria-hidden="true">{{ auditSortIndicator('user') }}</span>
              </button>
            </div>
            <div class="sticky top-0 z-[1] px-4 py-3 text-xs font-medium mts-muted">
              <button type="button" class="mts-focus-ring inline-flex items-center gap-1" data-testid="audit-sort-action-col" @click="cycleAuditSort('action')" :aria-sort="ariaSortValue(auditSort, 'action')">
                {{ t('action') }} <span aria-hidden="true">{{ auditSortIndicator('action') }}</span>
              </button>
            </div>
            <div class="sticky top-0 z-[1] px-4 py-3 text-xs font-medium mts-muted">
              <button type="button" class="mts-focus-ring inline-flex items-center gap-1" data-testid="audit-sort-database-col" @click="cycleAuditSort('database')" :aria-sort="ariaSortValue(auditSort, 'database')">
                {{ t('database') }} <span aria-hidden="true">{{ auditSortIndicator('database') }}</span>
              </button>
            </div>
            <div class="sticky top-0 z-[1] px-4 py-3 text-xs font-medium mts-muted">{{ t('auditColDetail') }}</div>
          </div>
          <div v-if="!displayedEvents.length" data-testid="audit-empty-body">
            <EmptyState
              v-if="loading"
              compact
              :title="t('loading')"
              :description="t('auditLoadingDesc')"
            />
            <EmptyState
              v-else
              :title="t('auditEmptyTitle')"
              :description="t('auditEmptyDesc')"
            >
              <template #action>
                <button type="button" class="mts-btn-primary" :disabled="loading" @click="loadAudit">{{ t('refresh') }}</button>
              </template>
            </EmptyState>
          </div>
          <VirtualTable
            v-else
            :items="displayedRows"
            :row-height="AUDIT_ROW_HEIGHT"
            :height="AUDIT_LIST_HEIGHT"
            data-testid="audit-virtual-list"
          >
            <template #default="{ item: row }">
              <div
                class="grid h-full grid-cols-[2.5rem_minmax(9rem,1.1fr)_minmax(6rem,0.8fr)_minmax(6rem,0.8fr)_minmax(6rem,0.7fr)_minmax(8rem,1.4fr)] items-center border-b border-slate-100 last:border-b-0 dark:border-slate-800"
                :data-testid="`audit-row-${row.idx}`"
              >
                <div class="px-3">
                  <input
                    type="checkbox"
                    class="h-3.5 w-3.5"
                    :data-testid="`audit-select-${row.idx}`"
                    :checked="isSelected(row.id)"
                    :aria-label="t('listSelectCol')"
                    @change="toggle(row.id, ($event.target as HTMLInputElement).checked)"
                  />
                </div>
                <div class="truncate px-4 text-xs text-slate-600 dark:text-slate-300">{{ row.evt.time }}</div>
                <div class="truncate px-4 text-xs text-slate-700 dark:text-slate-200">{{ row.evt.user_name }}</div>
                <div class="truncate px-4 text-xs font-medium text-slate-700 dark:text-slate-200">{{ row.evt.action }}</div>
                <div class="truncate px-4 text-xs mts-muted">{{ row.evt.database || t('emptyValue') }}</div>
                <div class="truncate px-4 text-xs mts-muted" :title="row.evt.detail || ''">{{ row.evt.detail || t('emptyValue') }}</div>
              </div>
            </template>
          </VirtualTable>
          <p
            v-if="displayedEvents.length"
            class="border-t border-slate-100 px-3 py-1.5 text-[11px] mts-muted dark:border-slate-800"
            data-testid="audit-virtual-hint"
          >
            {{ t('auditVirtualHint') }}
          </p>
        </div>
      </div>
    </div>
  </div>
</template>
