<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import EmptyState from '@/components/EmptyState.vue'
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

const { isAdmin } = useAuth()
const { t } = useI18n()
const { success, error: notifyError, warn } = useNotify()
const users = ref<User[]>([])
const selectedUser = ref('')
const actionFilter = ref('')
const sinceLocal = ref('')
const untilLocal = ref('')
const clientQuery = ref('')
const auditEvents = ref<AuditEvent[]>([])
const loading = ref(false)
const loadError = ref('')

const displayedEvents = computed(() => filterAuditEvents(auditEvents.value, clientQuery.value))
const filteredCount = computed(() => displayedEvents.value.length)

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
  await loadAudit()
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
    const params = new URLSearchParams()
    if (selectedUser.value) params.set('user_name', selectedUser.value)
    if (actionFilter.value.trim()) params.set('action', actionFilter.value.trim())
    const since = toUnix(sinceLocal.value)
    const until = toUnix(untilLocal.value)
    if (since) params.set('since_unix', String(since))
    if (until) params.set('until_unix', String(until))
    params.set('limit', '500')
    const qs = params.toString()
    const data = await apiGet<AuditResponse>(`/api/v1/admin/audit${qs ? `?${qs}` : ''}`)
    auditEvents.value = data.events ?? []
  } catch (e) {
    if (selectedUser.value) {
      try {
        const data = await apiGet<AuditResponse>(`/api/v1/users/${encodeURIComponent(selectedUser.value)}/audit`)
        auditEvents.value = data.events ?? []
        loadError.value = ''
        return
      } catch (e2) {
        loadError.value = formatCaughtError(e2)
      }
    } else {
      loadError.value = formatCaughtError(e)
    }
    auditEvents.value = []
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

function clearFilters() {
  selectedUser.value = ''
  actionFilter.value = ''
  sinceLocal.value = ''
  untilLocal.value = ''
  clientQuery.value = ''
  void loadAudit()
}

function downloadBlob(filename: string, blob: Blob) {
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = filename
  a.rel = 'noopener'
  document.body.appendChild(a)
  a.click()
  a.remove()
  URL.revokeObjectURL(url)
}

function exportJSON() {
  if (!displayedEvents.value.length) {
    warn(t.value('auditExportEmpty'))
    return
  }
  const blob = new Blob([JSON.stringify(displayedEvents.value, null, 2)], { type: 'application/json' })
  downloadBlob(`mts-audit-${Date.now()}.json`, blob)
  success(t.value('auditExportJSONOk'))
}

function exportCSV() {
  if (!displayedEvents.value.length) {
    warn(t.value('auditExportEmpty'))
    return
  }
  const header = ['time', 'user_name', 'action', 'database', 'detail']
  const lines = [header.join(',')]
  for (const e of displayedEvents.value) {
    const cols = [e.time, e.user_name, e.action, e.database || '', e.detail || ''].map((v) => {
      const s = String(v ?? '')
      return /[",\n]/.test(s) ? `"${s.replace(/"/g, '""')}"` : s
    })
    lines.push(cols.join(','))
  }
  const blob = new Blob([lines.join('\n')], { type: 'text/csv;charset=utf-8' })
  downloadBlob(`mts-audit-${Date.now()}.csv`, blob)
  success(t.value('auditExportCSVOk'))
}
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

    <div class="flex flex-wrap gap-2" data-testid="audit-quick-ranges">
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
      <div class="flex flex-wrap items-end gap-2">
        <button type="button" :disabled="loading" class="mts-btn-primary" data-testid="audit-reload" @click="loadAudit">
          <RefreshCw class="h-3.5 w-3.5" />
          {{ loading ? t('loading') : t('filter') }}
        </button>
        <button type="button" class="mts-btn" data-testid="audit-export-json" @click="exportJSON">
          <Download class="h-3.5 w-3.5" />
          {{ t('export') }}
        </button>
        <button type="button" class="mts-btn" data-testid="audit-export-csv" @click="exportCSV">CSV</button>
      </div>
    </div>

    <div class="rounded-xl border border-slate-200 bg-white dark:border-slate-700 dark:bg-slate-900">
      <div class="flex items-center justify-between border-b border-slate-100 px-4 py-2 text-xs mts-muted dark:border-slate-800">
        <span class="inline-flex items-center gap-1"><ScrollText class="h-3.5 w-3.5" /> {{ t('auditEvents') }}</span>
        <span data-testid="audit-count">{{ filteredCount }} / {{ auditEvents.length }}</span>
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
        <table class="w-full text-sm" data-testid="audit-table">
          <thead>
            <tr class="border-b border-slate-200 text-left dark:border-slate-700">
              <th class="px-4 py-3 text-xs font-medium mts-muted">{{ t('auditColTime') }}</th>
              <th class="px-4 py-3 text-xs font-medium mts-muted">{{ t('user') }}</th>
              <th class="px-4 py-3 text-xs font-medium mts-muted">{{ t('action') }}</th>
              <th class="px-4 py-3 text-xs font-medium mts-muted">{{ t('database') }}</th>
              <th class="px-4 py-3 text-xs font-medium mts-muted">{{ t('auditColDetail') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="(evt, idx) in displayedEvents" :key="idx" class="border-b border-slate-100 last:border-b-0 dark:border-slate-800">
              <td class="px-4 py-3 text-xs text-slate-600 dark:text-slate-300">{{ evt.time }}</td>
              <td class="px-4 py-3 text-xs text-slate-700 dark:text-slate-200">{{ evt.user_name }}</td>
              <td class="px-4 py-3 text-xs font-medium text-slate-700 dark:text-slate-200">{{ evt.action }}</td>
              <td class="px-4 py-3 text-xs mts-muted">{{ evt.database || '—' }}</td>
              <td class="px-4 py-3 text-xs mts-muted">{{ evt.detail || '—' }}</td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>
  </div>
</template>
