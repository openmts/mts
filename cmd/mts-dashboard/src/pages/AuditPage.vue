<script setup lang="ts">
import { ref, onMounted, computed, watch } from 'vue'
import { useRoute } from 'vue-router'
import { useHashScroll } from '@/composables/useHashScroll'
import EmptyState from '@/components/EmptyState.vue'
import ListSelectionToolbar from '@/components/ListSelectionToolbar.vue'
import ActionResultBanner from '@/components/ActionResultBanner.vue'
import { apiGet } from '@/api/client'
import { useAuth } from '@/composables/useAuth'
import PermissionDenied from '@/components/PermissionDenied.vue'
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
import { parseAuditPrefill } from '@/utils/routePrefill'
import { auditEventsToCSV } from '@/utils/auditExport'
import { filterRowsByIds } from '@/utils/listSelection'
import { useListSelection } from '@/composables/useListSelection'
import {
  cycleSortState,
  loadSortState,
  saveSortState,
  sortByAccessor,
  type SortState,
} from '@/utils/listSort'
import { auditRowId } from '@/utils/rowIds'
import { formatMessage } from '@/utils/formatMessage'
import { downloadJSON, downloadText, stampFilename } from '@/utils/download'
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
const { isAdmin } = useAuth()
const { t } = useI18n()
const { success, error: notifyError, warn } = useNotify()
const users = ref<User[]>([])
const selectedUser = ref('')
const actionFilter = ref('')
const sinceLocal = ref('')
const untilLocal = ref('')
const clientQuery = ref('')
const limit = ref(500)
const serverTotal = ref<number | null>(null)
const auditEvents = ref<AuditEvent[]>([])
const loading = ref(false)
const loadError = ref('')

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

const visibleAuditIds = computed(() =>
  displayedEvents.value.map((e, i) => auditRowId(e, i)),
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
  const withId = displayedEvents.value.map((e, i) => ({ row: e, id: auditRowId(e, i) }))
  const picked = filterRowsByIds(withId, exportIds.value, (r) => r.id)
  return picked.map((r) => r.row)
}

const quickRanges: { id: AuditQuickRange; labelKey: MessageKey }[] = [
  { id: '1h', labelKey: 'auditRange1h' },
  { id: '24h', labelKey: 'auditRange24h' },
  { id: '7d', labelKey: 'auditRange7d' },
  { id: '30d', labelKey: 'auditRange30d' },
]

onMounted(async () => {
  if (!isAdmin.value) return
  try {
    const data = await apiGet<UsersResponse>('/api/v1/users')
    users.value = data.users ?? []
  } catch (e) {
    loadError.value = formatCaughtError(e)
  }
  applyAuditPrefillFromRoute({ reload: false })
  await loadAudit()
})

watch(limit, () => {
  if (!isAdmin.value) return
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
    if (selectedUser.value) {
      try {
        const data = await apiGet<AuditResponse>(`/api/v1/users/${encodeURIComponent(selectedUser.value)}/audit`)
        auditEvents.value = data.events ?? []
        clearSelection()
        loadError.value = ''
        return
      } catch (e2) {
        loadError.value = formatCaughtError(e2)
      }
    } else {
      loadError.value = formatCaughtError(e)
    }
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
  }
  if (pre.action != null && actionFilter.value !== pre.action) {
    actionFilter.value = pre.action
    changed = true
  }
  if (pre.q != null && clientQuery.value !== pre.q) {
    clientQuery.value = pre.q
    changed = true
  }
  if (pre.user != null && selectedUser.value !== pre.user) {
    selectedUser.value = pre.user
    changed = true
  }
  if (changed) {
    success(t.value('auditPrefillApplied'))
    if (opts?.reload !== false && isAdmin.value) void loadAudit()
  }
}

function clearFilters() {
  selectedUser.value = ''
  actionFilter.value = ''
  sinceLocal.value = ''
  untilLocal.value = ''
  clientQuery.value = ''
  clearSelection()
  void loadAudit()
}

function exportJSON() {
  const rows = rowsForExport()
  if (!rows.length) {
    warn(t.value('auditExportEmpty'))
    return
  }
  downloadJSON(stampFilename('mts-audit', 'json'), rows)
  success(t.value('auditExportJSONOk'))
}

function exportCSV() {
  const rows = rowsForExport()
  if (!rows.length) {
    warn(t.value('auditExportEmpty'))
    return
  }
  downloadText(stampFilename('mts-audit', 'csv'), auditEventsToCSV(rows), 'text/csv;charset=utf-8')
  success(t.value('auditExportCSVOk'))
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
  <PermissionDenied v-if="!isAdmin" />
  <div v-else class="space-y-6">
    <div>
      <h1 class="mts-title flex items-center gap-2">
        <ScrollText class="h-5 w-5" />
        {{ t('auditTitle') }}
      </h1>
      <p class="text-xs mts-muted">{{ t('auditDesc') }}</p>
    </div>

    <ActionResultBanner v-if="loadError" kind="error" :message="loadError" @dismiss="loadError = ''" />
    <p class="mts-alert-warn">{{ t('auditHint') }}</p>

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
        <select v-model="selectedUser" class="mts-input mt-1" data-testid="audit-user">
          <option value="">{{ t('auditAllUsers') }}</option>
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
      <div class="flex flex-wrap items-end gap-2">
        <button type="button" :disabled="loading" class="mts-btn-primary" data-testid="audit-reload" @click="loadAudit">
          <RefreshCw class="h-3.5 w-3.5" />
          {{ loading ? t('loading') : t('filter') }}
        </button>
        <button type="button" class="mts-btn" data-testid="audit-export-json" @click="exportJSON">
          <Download class="h-3.5 w-3.5" />
          {{ t('exportJSON') }}
        </button>
        <button type="button" class="mts-btn" data-testid="audit-export-csv" @click="exportCSV">
          <Download class="h-3.5 w-3.5" />
          {{ t('exportCSV') }}
        </button>
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
            <button type="button" class="mts-btn" data-testid="audit-sort-time" :title="t('listSortBy')" @click="cycleAuditSort('time')">{{ t('auditColTime') }} {{ auditSortIndicator('time') }}</button>
            <button type="button" class="mts-btn" data-testid="audit-sort-user" :title="t('listSortBy')" @click="cycleAuditSort('user')">{{ t('user') }} {{ auditSortIndicator('user') }}</button>
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
      <div v-if="!displayedEvents.length">
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
      <div v-else class="overflow-x-auto">
        <div class="mts-table-wrap max-h-[28rem] overflow-auto">
        <table id="audit-table" class="scroll-mt-20 w-full text-sm" data-testid="audit-table">
          <thead>
            <tr class="border-b border-slate-200 text-left dark:border-slate-700">
              <th class="sticky top-0 z-[1] w-10 bg-white px-3 py-3 dark:bg-slate-900">
                <input
                  type="checkbox"
                  class="h-3.5 w-3.5"
                  data-testid="audit-select-all-checkbox"
                  :checked="allVisibleSelected"
                  :indeterminate.prop="someVisibleSelected"
                  :aria-label="t('listSelectAll')"
                  @change="toggleAllVisible(($event.target as HTMLInputElement).checked)"
                />
              </th>
              <th class="sticky top-0 z-[1] bg-white px-4 py-3 text-xs font-medium mts-muted dark:bg-slate-900">
                <button type="button" class="mts-focus-ring inline-flex items-center gap-1" data-testid="audit-sort-time-col" @click="cycleAuditSort('time')">
                  {{ t('auditColTime') }} <span aria-hidden="true">{{ auditSortIndicator('time') }}</span>
                </button>
              </th>
              <th class="sticky top-0 z-[1] bg-white px-4 py-3 text-xs font-medium mts-muted dark:bg-slate-900">
                <button type="button" class="mts-focus-ring inline-flex items-center gap-1" data-testid="audit-sort-user-col" @click="cycleAuditSort('user')">
                  {{ t('user') }} <span aria-hidden="true">{{ auditSortIndicator('user') }}</span>
                </button>
              </th>
              <th class="sticky top-0 z-[1] bg-white px-4 py-3 text-xs font-medium mts-muted dark:bg-slate-900">
                <button type="button" class="mts-focus-ring inline-flex items-center gap-1" data-testid="audit-sort-action-col" @click="cycleAuditSort('action')">
                  {{ t('action') }} <span aria-hidden="true">{{ auditSortIndicator('action') }}</span>
                </button>
              </th>
              <th class="sticky top-0 z-[1] bg-white px-4 py-3 text-xs font-medium mts-muted dark:bg-slate-900">
                <button type="button" class="mts-focus-ring inline-flex items-center gap-1" data-testid="audit-sort-database-col" @click="cycleAuditSort('database')">
                  {{ t('database') }} <span aria-hidden="true">{{ auditSortIndicator('database') }}</span>
                </button>
              </th>
              <th class="sticky top-0 z-[1] bg-white px-4 py-3 text-xs font-medium mts-muted dark:bg-slate-900">{{ t('auditColDetail') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr
              v-for="(evt, idx) in displayedEvents"
              :key="auditRowId(evt, idx)"
              class="border-b border-slate-100 last:border-b-0 dark:border-slate-800"
            >
              <td class="px-3 py-3">
                <input
                  type="checkbox"
                  class="h-3.5 w-3.5"
                  :data-testid="`audit-select-${idx}`"
                  :checked="isSelected(auditRowId(evt, idx))"
                  :aria-label="t('listSelectCol')"
                  @change="toggle(auditRowId(evt, idx), ($event.target as HTMLInputElement).checked)"
                />
              </td>
              <td class="px-4 py-3 text-xs text-slate-600 dark:text-slate-300">{{ evt.time }}</td>
              <td class="px-4 py-3 text-xs text-slate-700 dark:text-slate-200">{{ evt.user_name }}</td>
              <td class="px-4 py-3 text-xs font-medium text-slate-700 dark:text-slate-200">{{ evt.action }}</td>
              <td class="px-4 py-3 text-xs mts-muted">{{ evt.database || t('emptyValue') }}</td>
              <td class="px-4 py-3 text-xs mts-muted">{{ evt.detail || t('emptyValue') }}</td>
            </tr>
          </tbody>
        </table>
        </div>
      </div>
    </div>
  </div>
</template>
