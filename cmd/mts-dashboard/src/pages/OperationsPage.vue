<script setup lang="ts">
import { ref, computed, inject, onMounted, onBeforeUnmount, watch, type ComputedRef } from 'vue'
import { useRoute } from 'vue-router'
import { apiGet, apiPost } from '@/api/client'
import { useMutationGuard } from '@/composables/useMutationGuard'
import { useAuth } from '@/composables/useAuth'
import { useI18n } from '@/composables/useI18n'
import PermissionDenied from '@/components/PermissionDenied.vue'
import ConfirmDialog from '@/components/ConfirmDialog.vue'
import ActionResultBanner from '@/components/ActionResultBanner.vue'
import PartialErrorBanner from '@/components/PartialErrorBanner.vue'
import EmptyState from '@/components/EmptyState.vue'
import InFlightBanner from '@/components/InFlightBanner.vue'
import VirtualTable from '@/components/VirtualTable.vue'
import { useNotify } from '@/composables/useNotify'
import { useNotifyAdminBusy } from '@/composables/useNotifyAdminBusy'
import { formatCaughtError, isCanceledError, isTimeoutError } from '@/utils/apiError'
import { createActionAbort } from '@/utils/actionAbort'
import { makeActionResult } from '@/utils/actionResult'
import { useActionRetry } from '@/composables/useActionRetry'
import {
  appendOpsAction,
  buildOpsActionExport,
  clearOpsActionLog,
  loadOpsActionLog,
  saveOpsActionLog,
  type OpsActionEntry,
  type OpsActionKind,
} from '@/utils/opsActionLog'
import { stampFilename } from '@/utils/download'
import { useExportJob } from '@/composables/useExportJob'
import ExportJobBanner from '@/components/ExportJobBanner.vue'
import { copyText } from '@/utils/clipboard'
import { buildMaintenanceErrorsExport, buildOpsStatsExport, formatOpsStatsPretty, maintenanceErrorsToText } from '@/utils/opsExport'
import { RefreshCw, DatabaseBackup, Layers, Timer, AlertTriangle, Download, Eraser, Copy } from 'lucide-vue-next'
import type { CompactionStats, MaintenanceStats, MaintenanceStatsResponse, StorageMemorySnapshot } from '@/api/types'
import { useHashScroll } from '@/composables/useHashScroll'
import { useServerReachability } from '@/composables/useServerReachability'
import { useAdminOpBusy } from '@/composables/useAdminOpBusy'
import { adminOpKindLabelKey, adminHeavyBusyOpFromError, adminOpLastToneClass, formatAdminHeavyLastSummary, formatAdminHeavyLastCopyText, isAdminHeavyBusyError, joinAdminOpChip, parseAdminOpStatusPayload } from '@/utils/adminOpBusy'
import { formatMessage } from '@/utils/formatMessage'
import { parseOperationsPrefill, operationsFormToPrefill } from '@/utils/routePrefill'

interface CompactionStatsResponse { stats: CompactionStats }
interface MaintenanceErrorsResponse { errors: string[] }
interface StorageMemoryResponse { snapshot: StorageMemorySnapshot }

const { isAdmin } = useAuth()
const route = useRoute()
useHashScroll()
const { t } = useI18n()
const { offline, writeBlocked, blockReason, blockedMessageKey } = useMutationGuard()
const { success, error: notifyError, warn, info } = useNotify()
const { notifyAdminBusy, notifyMaybeAdminBusy } = useNotifyAdminBusy({ busyMessageKey: 'opsAdminBusy' })
const {
  exportJob,
  exportBusy,
  cancelExport,
  resetExport,
  retryLastExport,
  canRetryExport,
  runJSONExport,
} = useExportJob()
const { kind: connectivityKind, checking: reachChecking, checkOnce: retryReadyz } = useServerReachability()
const {
  adminOpBusy,
  adminOpKind,
  adminOpBusyChecking,
  adminOpLast,
  setAdminOpBusy,
  refreshAdminOpBusy,
  applyAdminOpStatus,
} = useAdminOpBusy()
const opsStatusRefreshing = ref(false)
async function refreshOpsBusyOnly() {
  if (opsStatusRefreshing.value || adminOpBusyChecking.value) return
  opsStatusRefreshing.value = true
  try {
    await refreshAdminOpBusy()
  } finally {
    opsStatusRefreshing.value = false
  }
}
const adminOpBusySummary = inject<ComputedRef<{ busy: boolean; opLabel: string; elapsed: string; detail: string; lastSummary?: string; lastOk?: boolean | null }> | undefined>('adminOpBusySummary', undefined)
const ackAdminOpLastFail = inject<(() => void) | undefined>('ackAdminOpLastFail', undefined)
const adminOpBusyDetailTitle = computed(() => adminOpBusySummary?.value?.detail || t.value('opsAdminBusy'))
const adminOpBusyChipLabel = computed(() => {
  if (confirmLoading.value) return t.value('opsAdminBusyChip')
  if (!adminOpBusy.value) return t.value('opsAdminBusyChip')
  const key = adminOpKindLabelKey(adminOpKind.value) as import('@/i18n/messages').MessageKey
  const kind = adminOpBusySummary?.value?.opLabel || t.value(key) || t.value('adminOpKindGeneric')
  const base = joinAdminOpChip(t.value('opsAdminBusyChip'), kind)
  const elapsed = adminOpBusySummary?.value?.elapsed
  return elapsed ? `${base} · ${elapsed}` : base
})
const statsLoadedAt = ref<number | null>(null)
const loadError = ref('')
const partialStatsError = ref('')

const adminOpLastLabel = computed(() => {
  const last = adminOpLast.value
  if (!last || !last.op) return ''
  const key = adminOpKindLabelKey(last.op) as import('@/i18n/messages').MessageKey
  const kind = t.value(key) || last.op
  return formatAdminHeavyLastSummary(last, kind)
})

async function copyAdminOpLast() {
  const last = adminOpLast.value
  if (!last || !last.op) {
    notifyError(t.value('opsStatusLastEmpty'))
    return
  }
  const key = adminOpKindLabelKey(last.op) as import('@/i18n/messages').MessageKey
  const kind = t.value(key) || last.op
  const textToCopy = formatAdminHeavyLastCopyText(last, kind)
  if (!textToCopy) {
    notifyError(t.value('opsStatusLastEmpty'))
    return
  }
  const res = await copyText(textToCopy)
  if (res.ok) success(t.value('opsStatusLastCopied'))
  else notifyError(res.error || t.value('failed'))
}

type OpsActionKey = 'flush' | 'compact' | 'retention'
const {
  lastFailedAction,
  actionResult,
  canRetryAction,
  clearActionResult,
  setActionOk,
  setActionResult,
  reportActionError: reportRetryError,
} = useActionRetry<OpsActionKey>()
const loading = ref(false)
const maintenanceStats = ref<MaintenanceStats | null>(null)
const compactionStats = ref<CompactionStats | null>(null)
const maintenanceErrors = ref<string[]>([])
const memorySnapshot = ref<StorageMemorySnapshot | null>(null)
type OpsSectionKey = 'maintenance' | 'compaction' | 'maintErrors' | 'memory'
const sectionRetrying = ref<Partial<Record<OpsSectionKey, boolean>>>({})
const sectionErrors = ref<Partial<Record<OpsSectionKey, string>>>({})
const maintErrorFilter = ref('')
const MAINT_ROW_HEIGHT = 36
const MAINT_LIST_HEIGHT = 160
const confirmKind = ref<'flush' | 'compact' | 'retention' | null>(null)
const confirmLoading = ref(false)
const opsActionStartedAt = ref<number | null>(null)
const opsActionAbort = createActionAbort()
const clearLogOpen = ref(false)
const clearLogLoading = ref(false)
const actionLog = ref<OpsActionEntry[]>(loadOpsActionLog())
const actionKindFilter = ref<'all' | OpsActionKind>('all')
const actionStatusFilter = ref<'all' | 'ok' | 'error'>('all')
const actionTextFilter = ref('')
const ACTION_ROW_HEIGHT = 48
const ACTION_LIST_HEIGHT = 288

const confirmTitle = computed(() => ({
  flush: t.value('opsConfirmFlush'),
  compact: t.value('opsConfirmCompact'),
  retention: t.value('opsConfirmRetention'),
}))

const confirmMessage = computed(() => ({
  flush: t.value('opsMsgFlush'),
  compact: t.value('opsMsgCompact'),
  retention: t.value('opsMsgRetention') + '\n' + t.value('opsRetentionRequireHint'),
}))

const confirmRequireText = computed(() => {
  if (confirmKind.value === 'retention') return t.value('opsRetentionRequire')
  return ''
})

function persistLog() {
  saveOpsActionLog(actionLog.value)
}

function recordAction(kind: OpsActionKind, status: 'ok' | 'error', message: string) {
  actionLog.value = appendOpsAction(actionLog.value, {
    kind,
    status,
    message,
    at: Date.now(),
  })
  persistLog()
}

function sectionLabel(key: OpsSectionKey): string {
  switch (key) {
    case 'maintenance': return t.value('overviewPartialMaintenance')
    case 'compaction': return t.value('overviewPartialCompaction')
    case 'maintErrors': return t.value('overviewPartialMaintErrors')
    case 'memory': return t.value('overviewPartialMemory')
  }
}

function clearSectionError(key: OpsSectionKey) {
  if (!sectionErrors.value[key]) return
  const next = { ...sectionErrors.value }
  delete next[key]
  sectionErrors.value = next
}

function setSectionError(key: OpsSectionKey, err: unknown) {
  sectionErrors.value = { ...sectionErrors.value, [key]: formatCaughtError(err) }
}

async function loadOpsSection(key: OpsSectionKey): Promise<void> {
  switch (key) {
    case 'maintenance': {
      const v = await apiGet<MaintenanceStatsResponse>('/api/v1/admin/stats/maintenance')
      maintenanceStats.value = v.stats ?? null
      applyAdminOpStatus(parseAdminOpStatusPayload(v))
      return
    }
    case 'compaction': {
      const v = await apiGet<CompactionStatsResponse>('/api/v1/admin/stats/compaction')
      compactionStats.value = v.stats
      return
    }
    case 'maintErrors': {
      const v = await apiGet<MaintenanceErrorsResponse>('/api/v1/admin/maintenance/errors')
      maintenanceErrors.value = v.errors ?? []
      return
    }
    case 'memory': {
      const v = await apiGet<StorageMemoryResponse>('/api/v1/admin/stats/storage-memory')
      memorySnapshot.value = v.snapshot ?? null
      return
    }
  }
}

async function retryOpsSection(key: OpsSectionKey) {
  if (!isAdmin.value || sectionRetrying.value[key]) return
  sectionRetrying.value = { ...sectionRetrying.value, [key]: true }
  try {
    await loadOpsSection(key)
    clearSectionError(key)
  } catch (e) {
    setSectionError(key, e)
  } finally {
    const next = { ...sectionRetrying.value }
    delete next[key]
    sectionRetrying.value = next
    rebuildPartialFromSections()
  }
}

function rebuildPartialFromSections() {
  const order: OpsSectionKey[] = ['maintenance', 'compaction', 'maintErrors', 'memory']
  const partials = order
    .filter((k) => sectionErrors.value[k])
    .map((k) => `${sectionLabel(k)}: ${sectionErrors.value[k]}`)
  const hasAnySnapshot = !!(
    maintenanceStats.value
    || compactionStats.value
    || (maintenanceErrors.value && maintenanceErrors.value.length)
    || memorySnapshot.value
  )
  if (partials.length === 4 && !hasAnySnapshot) {
    loadError.value = partials.join('；')
    partialStatsError.value = ''
    return
  }
  loadError.value = ''
  partialStatsError.value = partials.join('；')
  if (!partials.length || hasAnySnapshot || partials.length < 4) {
    statsLoadedAt.value = Date.now()
  }
}

const opsFailedSections = computed(() => {
  const order: OpsSectionKey[] = ['maintenance', 'compaction', 'maintErrors', 'memory']
  return order.filter((k) => Boolean(sectionErrors.value[k]))
})

async function loadStats() {
  if (!isAdmin.value) return
  loading.value = true
  loadError.value = ''
  partialStatsError.value = ''
  sectionErrors.value = {}
  try {
    const keys: OpsSectionKey[] = ['maintenance', 'compaction', 'maintErrors', 'memory']
    const results = await Promise.allSettled(keys.map((k) => loadOpsSection(k)))
    for (let i = 0; i < keys.length; i++) {
      const r = results[i]
      const key = keys[i]
      if (r.status === 'fulfilled') clearSectionError(key)
      else setSectionError(key, r.reason)
    }
    rebuildPartialFromSections()
  } catch (e) {
    loadError.value = formatCaughtError(e)
  } finally {
    loading.value = false
  }
}

const connectivityLabel = computed(() => {
  switch (connectivityKind.value) {
    case 'ok':
      return t.value('connectivityOk')
    case 'unreachable':
      return t.value('connectivityUnreachable')
    case 'offline':
      return t.value('connectivityOffline')
    default:
      return t.value('connectivityUnknown')
  }
})

const connectivityHint = computed(() => {
  switch (connectivityKind.value) {
    case 'ok':
      return t.value('connectivityHintOk')
    case 'unreachable':
      return t.value('connectivityHintUnreachable')
    case 'offline':
      return t.value('connectivityHintOffline')
    default:
      return t.value('connectivityHintUnknown')
  }
})

const connectivityToneClass = computed(() => {
  switch (connectivityKind.value) {
    case 'ok':
      return 'bg-emerald-50 text-emerald-800 dark:bg-emerald-950/40 dark:text-emerald-200'
    case 'unreachable':
      return 'bg-red-50 text-red-700 dark:bg-red-950/40 dark:text-red-200'
    case 'offline':
      return 'bg-amber-50 text-amber-900 dark:bg-amber-950/40 dark:text-amber-100'
    default:
      return 'bg-slate-100 text-slate-600 dark:bg-slate-800 dark:text-slate-300'
  }
})

const statsLoadedLabel = computed(() => {
  if (loading.value) return t.value('opsStatsLoading')
  if (!statsLoadedAt.value) return t.value('opsStatsNeverLoaded')
  return formatMessage(t.value('opsStatsLastLoaded'), { time: formatAt(statsLoadedAt.value) })
})


function openConfirm(kind: 'flush' | 'compact' | 'retention') {
  if (confirmLoading.value) return
  if (writeBlocked.value) {
    notifyError(t.value(blockedMessageKey('offlineOpsBlocked')))
    return
  }
  if (adminOpBusy.value) {
    notifyAdminBusy()
    return
  }
  confirmKind.value = kind
}

function reportActionError(kind: OpsActionKey, e: unknown) {
  if (isAdminHeavyBusyError(e)) {
    setAdminOpBusy(true, adminHeavyBusyOpFromError(e) || undefined)
    void refreshAdminOpBusy()
  }
  reportRetryError(kind, e)
  const msg = actionResult.value?.message || formatCaughtError(e)
  if (isAdminHeavyBusyError(e)) notifyAdminBusy(msg)
  else notifyError(msg)
  recordAction(kind, 'error', msg)
}

async function retryLastOpsAction() {
  const kind = lastFailedAction.value
  if (!kind) return
  if (confirmLoading.value) return
  if (writeBlocked.value) {
    notifyError(t.value(blockedMessageKey('offlineOpsBlocked')))
    return
  }
  if (adminOpBusy.value) {
    notifyAdminBusy()
    return
  }
  confirmKind.value = kind as OpsActionKey
  await runConfirmed()
}

function cancelOpsAction() {
  opsActionAbort.cancel()
}

async function runConfirmed() {
  if (!confirmKind.value) return
  if (confirmLoading.value) return
  if (writeBlocked.value) {
    notifyError(t.value(blockedMessageKey('offlineOpsBlocked')))
    confirmKind.value = null
    return
  }
  if (adminOpBusy.value) {
    notifyAdminBusy()
    return
  }
  const kind = confirmKind.value
  confirmLoading.value = true
  setAdminOpBusy(true, kind)
  opsActionStartedAt.value = Date.now()
  clearActionResult()
  const signal = opsActionAbort.begin()
  try {
    let msg = ''
    if (kind === 'flush') {
      await apiPost('/api/v1/admin/flush', {}, { signal })
      msg = t.value('opsFlushDone')
    } else if (kind === 'compact') {
      await apiPost('/api/v1/admin/compact', {}, { signal })
      msg = t.value('opsCompactDone')
    } else {
      await apiPost('/api/v1/admin/retention/apply', {}, { signal })
      msg = t.value('opsRetentionDone')
    }
    setActionOk(msg)
    success(msg)
    recordAction(kind, 'ok', msg)
    confirmKind.value = null
    await loadStats()
  } catch (e) {
    if (isCanceledError(e)) {
      const msg = t.value('opsActionCancelled')
      setActionResult(makeActionResult('info', msg))
      info(msg)
      recordAction(kind, 'error', msg)
    } else if (isTimeoutError(e)) {
      const msg = t.value('opsActionTimedOut')
      setActionResult(makeActionResult('error', msg))
      notifyError(msg)
      recordAction(kind, 'error', msg)
    } else {
      reportActionError(kind, e)
    }
  } finally {
    opsActionAbort.end()
    confirmLoading.value = false
    opsActionStartedAt.value = null
    try {
      await loadOpsSection('maintenance')
    } catch {
      await refreshAdminOpBusy()
    }
  }
}


async function exportStats() {
  if (exportBusy.value) return
  const payload = buildOpsStatsExport({
    maintenance: maintenanceStats.value,
    compaction: compactionStats.value,
    memory: memorySnapshot.value,
    maintenance_errors: maintenanceErrors.value,
  })
  const outcome = await runJSONExport({
    label: 'JSON',
    filename: stampFilename('mts-ops-stats', 'json'),
    total: 1,
    build: async ({ isCancelled, progress }) => {
      progress(0, 1)
      if (isCancelled()) return null
      progress(1, 1)
      return payload
    },
  })
  if (outcome === 'done') success(t.value('opsStatsExported'))
  else if (outcome === 'cancelled') info(t.value('exportCancelledToast'))
  else if (outcome === 'error') notifyError(exportJob.value.error || t.value('failed'))
}

async function copyStats() {
  const res = await copyText(
    formatOpsStatsPretty({
      maintenance: maintenanceStats.value,
      compaction: compactionStats.value,
      memory: memorySnapshot.value,
      maintenance_errors: maintenanceErrors.value,
    }),
  )
  if (res.ok) success(t.value('opsStatsCopied'))
  else notifyError(res.error || t.value('failed'))
}

async function exportMaintErrors() {
  if (!filteredMaintenanceErrors.value.length) {
    warn(t.value('opsMaintErrorsEmpty'))
    return
  }
  if (exportBusy.value) return
  const list = filteredMaintenanceErrors.value.slice()
  const outcome = await runJSONExport({
    label: 'JSON',
    filename: stampFilename('mts-ops-maintenance-errors', 'json'),
    total: list.length,
    build: async ({ isCancelled, progress }) => {
      progress(0, list.length)
      const chunk = 100
      for (let i = 0; i < list.length; i++) {
        if (isCancelled()) return null
        const done = i + 1
        if (done === list.length || done % chunk === 0) {
          progress(done, list.length)
          if (done < list.length) await new Promise((r) => setTimeout(r, 0))
        }
      }
      return buildMaintenanceErrorsExport(list)
    },
  })
  if (outcome === 'done') success(t.value('opsMaintErrorsExported'))
  else if (outcome === 'cancelled') info(t.value('exportCancelledToast'))
  else if (outcome === 'error') notifyError(exportJob.value.error || t.value('failed'))
}

async function copyMaintErrors() {
  if (!filteredMaintenanceErrors.value.length) {
    warn(t.value('opsMaintErrorsEmpty'))
    return
  }
  const res = await copyText(maintenanceErrorsToText(filteredMaintenanceErrors.value))
  if (res.ok) success(t.value('opsMaintErrorsCopied'))
  else notifyError(res.error || t.value('failed'))
}

async function exportActionLog() {
  if (!filteredActionLog.value.length) {
    warn(t.value('opsLogExportEmpty'))
    return
  }
  if (exportBusy.value) return
  const list = filteredActionLog.value.slice()
  const outcome = await runJSONExport({
    label: 'JSON',
    filename: stampFilename('mts-ops-actions', 'json'),
    total: list.length,
    build: async ({ isCancelled, progress }) => {
      progress(0, list.length)
      for (let i = 0; i < list.length; i++) {
        if (isCancelled()) return null
        const done = i + 1
        if (done === list.length || done % 100 === 0) {
          progress(done, list.length)
          if (done < list.length) await new Promise((r) => setTimeout(r, 0))
        }
      }
      return buildOpsActionExport(list)
    },
  })
  if (outcome === 'done') success(t.value('opsLogExportOk'))
  else if (outcome === 'cancelled') info(t.value('exportCancelledToast'))
  else if (outcome === 'error') notifyError(exportJob.value.error || t.value('failed'))
}

function openClearLog() {
  if (clearLogLoading.value) return
  if (writeBlocked.value) {
    notifyError(t.value(blockedMessageKey('offlineOpsBlocked')))
    return
  }
  clearLogOpen.value = true
}

function confirmClearLog() {
  if (clearLogLoading.value) return
  if (writeBlocked.value) {
    notifyError(t.value(blockedMessageKey('offlineOpsBlocked')))
    clearLogOpen.value = false
    return
  }
  clearLogLoading.value = true
  try {
    actionLog.value = []
    clearOpsActionLog()
    info(t.value('opsLogCleared'))
    clearLogOpen.value = false
  } finally {
    clearLogLoading.value = false
  }
}

function formatAt(at: number): string {
  try {
    return new Date(at).toLocaleString()
  } catch {
    return String(at)
  }
}

const filteredMaintenanceErrors = computed(() => {
  const q = maintErrorFilter.value.trim().toLowerCase()
  if (!q) return maintenanceErrors.value
  return maintenanceErrors.value.filter((e) => e.toLowerCase().includes(q))
})

const filteredActionLog = computed(() => {
  const q = actionTextFilter.value.trim().toLowerCase()
  return actionLog.value.filter((item) => {
    if (actionKindFilter.value !== 'all' && item.kind !== actionKindFilter.value) return false
    if (actionStatusFilter.value !== 'all' && item.status !== actionStatusFilter.value) return false
    if (!q) return true
    return (
      item.kind.toLowerCase().includes(q)
      || item.status.toLowerCase().includes(q)
      || item.message.toLowerCase().includes(q)
    )
  })
})

function applyOperationsPrefillFromRoute() {
  if (!isAdmin.value) return
  const pre = parseOperationsPrefill(route.query as Record<string, unknown>)
  let changed = false
  if (pre.maint_q != null && maintErrorFilter.value !== pre.maint_q) {
    maintErrorFilter.value = pre.maint_q
    changed = true
  }
  if (pre.action_kind && pre.action_kind !== 'all' && actionKindFilter.value !== pre.action_kind) {
    actionKindFilter.value = pre.action_kind as typeof actionKindFilter.value
    changed = true
  }
  if (pre.action_status && pre.action_status !== 'all' && actionStatusFilter.value !== pre.action_status) {
    actionStatusFilter.value = pre.action_status as typeof actionStatusFilter.value
    changed = true
  }
  if (pre.action_q != null && actionTextFilter.value !== pre.action_q) {
    actionTextFilter.value = pre.action_q
    changed = true
  }
  if (changed) success(t.value('opsPrefillApplied'))
}

async function copyOperationsShareLink() {
  const path = operationsFormToPrefill({
    maint_q: maintErrorFilter.value,
    action_kind: actionKindFilter.value,
    action_status: actionStatusFilter.value,
    action_q: actionTextFilter.value,
  }, {
    hash: maintErrorFilter.value ? '#ops-maint-errors' : '#ops-action-log',
  })
  const url = `${window.location.origin}${path}`
  const res = await copyText(url)
  if (res.ok) success(t.value('opsShareCopied'))
  else notifyError(res.error || t.value('failed'))
}

onBeforeUnmount(() => {
  cancelOpsAction()
})

onMounted(() => {
  ackAdminOpLastFail?.()
  void loadStats()
  applyOperationsPrefillFromRoute()
})

watch(
  () => route.fullPath,
  (path, prev) => {
    if (!isAdmin.value) return
    if (prev != null && path !== prev) applyOperationsPrefillFromRoute()
  },
)
</script>

<template>
  <PermissionDenied v-if="!isAdmin" />
  <div v-else class="space-y-6" data-testid="ops-page">
    <div class="flex flex-wrap items-center justify-between gap-3">
      <div>
        <h1 class="mts-title">{{ t('opsTitle') }}</h1>
        <p class="text-xs mts-muted">{{ t('opsDesc') }}</p>
      </div>
      <div class="flex flex-wrap gap-2">
        <button type="button" class="mts-btn" :disabled="loading" data-testid="ops-refresh" @click="loadStats" :aria-busy="loading ? 'true' : undefined">
          <RefreshCw class="h-3.5 w-3.5" /> {{ t('refresh') }}
        </button>
        <button type="button" class="mts-btn" data-testid="ops-export-stats" :disabled="exportBusy" @click="exportStats">
          <Download class="h-3.5 w-3.5" /> {{ t('opsExportStats') }}
        </button>
        <button type="button" class="mts-btn" data-testid="ops-share-link" @click="copyOperationsShareLink">
          {{ t('opsShareLink') }}
        </button>
        <button type="button" class="mts-btn" data-testid="ops-copy-stats" @click="copyStats">
          <Copy class="h-3.5 w-3.5" /> {{ t('opsCopyStats') }}
        </button>
      </div>
    </div>

    <ActionResultBanner
      v-if="loadError"
      kind="error"
      :message="loadError"
      retryable
      data-testid="ops-load-error"
      @retry="loadStats"
      @dismiss="loadError = ''"
    />
    <PartialErrorBanner
      v-else-if="partialStatsError"
      :message="`${t('partialAdminStats')}：${partialStatsError}`"
      test-id="ops-partial-error"
      @retry="loadStats"
      @dismiss="partialStatsError = ''"
    />
    <div
      v-if="opsFailedSections.length"
      class="flex flex-wrap items-center gap-2 rounded-lg border border-amber-200 bg-amber-50/80 px-3 py-2 dark:border-amber-900/50 dark:bg-amber-950/30"
      data-testid="ops-partial-sections"
    >
      <span class="text-xs font-medium text-amber-900 dark:text-amber-100">{{ t('overviewPartialRetryHint') }}</span>
      <button
        v-for="key in opsFailedSections"
        :key="key"
        type="button"
        class="mts-btn text-xs"
        :data-testid="`ops-partial-retry-${key}`"
        :disabled="!!sectionRetrying[key]"
        @click="retryOpsSection(key)"
      >
        {{ sectionLabel(key) }} · {{ sectionRetrying[key] ? t('loading') : t('retry') }}
      </button>
    </div>

    <ActionResultBanner
      :result="actionResult"
      :retryable="canRetryAction"
      data-testid="ops-action-result"
      @retry="retryLastOpsAction"
      @dismiss="clearActionResult"
    />

    <InFlightBanner
      :active="confirmLoading"
      :started-at-ms="opsActionStartedAt"
      kind="ops"
      @cancel="cancelOpsAction"
    />

    <div
      id="ops-status-strip"
      class="mts-panel flex flex-wrap items-center justify-between gap-3 scroll-mt-20"
      data-testid="ops-status-strip"
    >
      <div class="min-w-0 space-y-1">
        <p class="text-xs font-semibold uppercase tracking-wide mts-muted">{{ t('opsStatusStripTitle') }}</p>
        <div class="flex flex-wrap items-center gap-2 text-sm">
          <span
            class="inline-flex rounded-full px-2 py-0.5 text-xs font-medium"
            :class="connectivityToneClass"
            data-testid="ops-status-connectivity"
            :title="connectivityHint"
          >{{ t('connectivityTitle') }}: {{ connectivityLabel }}</span>
          <span
            v-if="adminOpBusy || confirmLoading"
            class="inline-flex rounded-full bg-sky-50 px-2 py-0.5 text-xs font-medium text-sky-900 dark:bg-sky-950/40 dark:text-sky-100"
            data-testid="ops-admin-busy"
            :title="adminOpBusyDetailTitle"
          >{{ adminOpBusyChipLabel }}</span>
          <span class="text-xs mts-muted" data-testid="ops-status-stats-at">{{ statsLoadedLabel }}</span>
          <span
            v-if="loading || reachChecking"
            class="text-xs mts-muted"
            data-testid="ops-status-loading"
          >{{ loading ? t('opsStatsLoading') : t('connectivityUnknown') }}</span>
        </div>
        <p
          v-if="adminOpBusy && !confirmLoading"
          class="text-xs text-sky-800 dark:text-sky-200"
          data-testid="ops-status-busy-hint"
        >{{ t('opsStatusBusyHint') }}</p>
        <div
          v-if="adminOpLastLabel"
          class="flex flex-wrap items-start gap-2"
          data-testid="ops-status-last-row"
        >
          <p
            :class="adminOpLastToneClass(adminOpLast?.ok)"
            data-testid="ops-status-last"
            :data-ok="adminOpLast?.ok === true ? 'true' : (adminOpLast?.ok === false ? 'false' : undefined)"
            :title="adminOpLast?.error || adminOpLastLabel"
          >
            <span class="font-medium text-slate-700 dark:text-slate-200">{{ t('opsStatusLastLabel') }}:</span>
            {{ adminOpLastLabel }}
          </p>
          <button
            type="button"
            class="mts-btn py-0.5 text-[11px]"
            data-testid="ops-status-last-copy"
            :title="t('opsStatusLastCopy')"
            @click="copyAdminOpLast"
          >
            <Copy class="h-3 w-3" /> {{ t('opsStatusLastCopy') }}
          </button>
        </div>
        <p
          v-else-if="!adminOpBusy"
          class="text-xs mts-muted"
          data-testid="ops-status-last-empty"
        >{{ t('opsStatusLastEmpty') }}</p>
      </div>
      <div class="flex flex-wrap gap-2">
        <button type="button" class="mts-btn" data-testid="ops-status-retry-readyz" :disabled="reachChecking" @click="retryReadyz">
          <RefreshCw class="h-3.5 w-3.5" /> {{ t('connectivityTitle') }}
        </button>
        <button
          type="button"
          class="mts-btn"
          data-testid="ops-status-refresh-busy"
          :disabled="opsStatusRefreshing || adminOpBusyChecking"
          :aria-busy="opsStatusRefreshing || adminOpBusyChecking ? 'true' : undefined"
          :title="t('opsStatusRefreshBusy')"
          @click="refreshOpsBusyOnly"
        >
          <RefreshCw class="h-3.5 w-3.5" :class="(opsStatusRefreshing || adminOpBusyChecking) ? 'animate-spin' : ''" />
          {{ t('opsStatusRefreshBusy') }}
        </button>
        <button type="button" class="mts-btn" data-testid="ops-status-refresh-stats" :disabled="loading" @click="loadStats" :aria-busy="loading ? 'true' : undefined">
          <RefreshCw class="h-3.5 w-3.5" /> {{ t('refresh') }}
        </button>
      </div>
    </div>

    <div id="ops-maint-errors" class="mts-panel scroll-mt-20" data-testid="ops-maint-errors-panel">
      <div class="mb-2 flex flex-wrap items-center justify-between gap-2">
        <div class="flex items-center gap-2 text-sm font-semibold text-slate-800 dark:text-slate-100">
          <AlertTriangle class="h-4 w-4" /> {{ t('maintenanceErrors') }} ({{ maintenanceErrors.length }})
        </div>
        <div class="flex flex-wrap items-center gap-2">
          <input
            v-if="maintenanceErrors.length"
            v-model="maintErrorFilter"
            type="search"
            class="mts-input max-w-xs text-xs"
            data-testid="ops-maint-errors-filter"
            :placeholder="t('opsMaintErrorsFilter')"
          />
          <button type="button" class="mts-btn" data-testid="ops-export-maint-errors" :disabled="exportBusy || (!filteredMaintenanceErrors.length)" @click="exportMaintErrors">
            <Download class="h-3.5 w-3.5" /> {{ t('opsExportMaintErrors') }}
          </button>
          <button type="button" class="mts-btn" data-testid="ops-copy-maint-errors" :disabled="!filteredMaintenanceErrors.length" @click="copyMaintErrors">
            <Copy class="h-3.5 w-3.5" /> {{ t('opsCopyMaintErrors') }}
          </button>
        </div>
      </div>
      <EmptyState
        v-if="!maintenanceErrors.length"
        compact
        data-testid="ops-maint-errors-empty"
        :title="sectionErrors.maintErrors ? t('opsMaintErrorsLoadFailed') : t('noMaintenanceErrors')"
        :description="sectionErrors.maintErrors ? sectionErrors.maintErrors : t('opsNoMaintErrorsDesc')"
      >
        <template #action>
          <button
            type="button"
            class="mts-btn-primary text-xs"
            data-testid="ops-maint-errors-retry"
            :disabled="!!sectionRetrying.maintErrors || loading"
            @click="retryOpsSection('maintErrors')"
          >{{ sectionRetrying.maintErrors ? t('loading') : t('overviewSectionRetry') }}</button>
        </template>
      </EmptyState>
      <EmptyState
        v-else-if="!filteredMaintenanceErrors.length"
        compact
        data-testid="ops-maint-errors-filter-empty"
        :title="t('opsMaintErrorsFilterEmpty')"
        :description="t('opsNoMaintErrorsDesc')"
      >
        <template #action>
          <button type="button" class="mts-btn-primary" data-testid="ops-maint-errors-clear-filters" @click="maintErrorFilter = ''">{{ t('clearFilters') }}</button>
        </template>
      </EmptyState>
      <div v-else class="overflow-hidden rounded-lg border border-red-100 dark:border-red-900/40" data-testid="ops-maint-errors">
        <VirtualTable
          :items="filteredMaintenanceErrors"
          :row-height="MAINT_ROW_HEIGHT"
          :height="Math.min(MAINT_LIST_HEIGHT, Math.max(108, filteredMaintenanceErrors.length * MAINT_ROW_HEIGHT))"
          data-testid="ops-maint-errors-virtual-list"
        >
          <template #default="{ item: e, index }">
            <div
              class="h-full border-b border-red-50 px-2 py-1 text-xs text-red-700 dark:border-red-950/40 dark:text-red-200"
              :data-testid="`ops-maint-error-row-${index}`"
            >
              <div class="truncate rounded bg-red-50 px-2 py-1 dark:bg-red-950/40" :title="e">{{ e }}</div>
            </div>
          </template>
        </VirtualTable>
        <p class="border-t border-red-100 px-3 py-1.5 text-[11px] mts-muted dark:border-red-900/40" data-testid="ops-maint-errors-virtual-hint">
          {{ t('opsMaintErrorsVirtualHint') }}
        </p>
      </div>
    </div>

    <div id="ops-actions" class="grid gap-4 scroll-mt-20 sm:grid-cols-3">
      <button
        type="button"
        id="ops-flush"
        class="mts-card p-5 text-left hover:border-slate-300 dark:hover:border-slate-500 disabled:cursor-not-allowed disabled:opacity-50"
        data-testid="ops-flush"
        :disabled="writeBlocked || confirmLoading || adminOpBusy"
        :title="writeBlocked ? t(blockedMessageKey('offlineOpsBlocked')) : (adminOpBusy ? t('opsAdminBusy') : undefined)"
        @click="openConfirm('flush')"
      >
        <DatabaseBackup class="mb-2 h-5 w-5 mts-muted" />
        <p class="text-sm font-semibold text-slate-800 dark:text-slate-100">{{ t('opsActionFlush') }}</p>
        <p class="mt-1 text-xs mts-muted">{{ t('opsFlushHint') }}</p>
      </button>
      <button
        type="button"
        class="mts-card p-5 text-left hover:border-slate-300 dark:hover:border-slate-500 disabled:cursor-not-allowed disabled:opacity-50"
        id="ops-compact"
        data-testid="ops-compact"
        :disabled="writeBlocked || confirmLoading || adminOpBusy"
        :title="writeBlocked ? t(blockedMessageKey('offlineOpsBlocked')) : (adminOpBusy ? t('opsAdminBusy') : undefined)"
        @click="openConfirm('compact')"
      >
        <Layers class="mb-2 h-5 w-5 mts-muted" />
        <p class="text-sm font-semibold text-slate-800 dark:text-slate-100">{{ t('opsActionCompact') }}</p>
        <p class="mt-1 text-xs mts-muted">{{ t('opsCompactHint') }}</p>
      </button>
      <button
        type="button"
        class="mts-card p-5 text-left hover:border-slate-300 dark:hover:border-slate-500 disabled:cursor-not-allowed disabled:opacity-50"
        id="ops-retention"
        data-testid="ops-retention"
        :disabled="writeBlocked || confirmLoading || adminOpBusy"
        :title="writeBlocked ? t(blockedMessageKey('offlineOpsBlocked')) : (adminOpBusy ? t('opsAdminBusy') : undefined)"
        @click="openConfirm('retention')"
      >
        <Timer class="mb-2 h-5 w-5 mts-muted" />
        <p class="text-sm font-semibold text-slate-800 dark:text-slate-100">{{ t('opsActionRetention') }}</p>
        <p class="mt-1 text-xs mts-muted">{{ t('opsRetentionHint') }}</p>
      </button>
    </div>

    <div class="grid gap-4 lg:grid-cols-2">
      <div class="mts-card p-5" data-testid="ops-maint-stats">
        <h2 class="mb-3 text-sm font-semibold text-slate-800 dark:text-slate-100">{{ t('maintenanceStats') }}</h2>
        <EmptyState
          v-if="!maintenanceStats"
          compact
          data-testid="ops-maint-stats-empty"
          :title="t('opsNoMaintStats')"
          :description="t('opsStatsEmptyHint')"
        >
          <template #action>
            <button type="button" class="mts-btn-primary text-xs" data-testid="ops-maint-stats-retry" :disabled="!!sectionRetrying.maintenance || loading" @click="retryOpsSection('maintenance')">
              {{ sectionRetrying.maintenance ? t('loading') : t('overviewSectionRetry') }}
            </button>
          </template>
        </EmptyState>
        <dl v-else class="grid grid-cols-2 gap-2 text-xs text-slate-600 dark:text-slate-300">
          <div>{{ t('opsStatCompactActive') }}: <b>{{ maintenanceStats.compaction_active }}</b></div>
          <div>{{ t('opsStatCompactBacklog') }}: <b>{{ maintenanceStats.compaction_backlog }}</b></div>
          <div>{{ t('opsStatCompactFailure') }}: <b>{{ maintenanceStats.compaction_failure }}</b></div>
          <div>{{ t('opsStatDownsampleInflight') }}: <b>{{ maintenanceStats.downsample_inflight }}</b></div>
          <div>{{ t('opsStatDownsampleFailure') }}: <b>{{ maintenanceStats.downsample_failure }}</b></div>
          <div>{{ t('opsStatErrors') }}: <b>{{ maintenanceStats.maintenance_error_count }}</b></div>
        </dl>
      </div>
      <div class="mts-card p-5" data-testid="ops-compact-stats">
        <h2 class="mb-3 text-sm font-semibold text-slate-800 dark:text-slate-100">{{ t('compactionStats') }}</h2>
        <EmptyState
          v-if="!compactionStats"
          compact
          data-testid="ops-compact-stats-empty"
          :title="t('opsNoCompactStats')"
          :description="t('opsStatsEmptyHint')"
        >
          <template #action>
            <button type="button" class="mts-btn-primary text-xs" data-testid="ops-compact-stats-retry" :disabled="!!sectionRetrying.compaction || loading" @click="retryOpsSection('compaction')">
              {{ sectionRetrying.compaction ? t('loading') : t('overviewSectionRetry') }}
            </button>
          </template>
        </EmptyState>
        <dl v-else class="grid grid-cols-2 gap-2 text-xs text-slate-600 dark:text-slate-300">
          <div>{{ t('opsStatTotal') }}: <b>{{ compactionStats.total }}</b></div>
          <div>{{ t('opsStatSuccess') }}: <b>{{ compactionStats.success }}</b></div>
          <div>{{ t('opsStatFailure') }}: <b>{{ compactionStats.failure }}</b></div>
          <div>{{ t('opsStatBacklog') }}: <b>{{ compactionStats.backlog }}</b></div>
          <div>{{ t('opsStatActive') }}: <b>{{ compactionStats.active }}</b></div>
        </dl>
        <p v-if="compactionStats?.last_error" class="mt-2 text-xs text-red-600 dark:text-red-300">{{ compactionStats.last_error }}</p>
      </div>
      <div class="mts-card p-5 lg:col-span-2" data-testid="ops-memory-stats">
        <h2 class="mb-3 text-sm font-semibold text-slate-800 dark:text-slate-100">{{ t('opsMemoryStats') }}</h2>
        <EmptyState
          v-if="!memorySnapshot"
          compact
          data-testid="ops-memory-stats-empty"
          :title="t('opsNoMemoryStats')"
          :description="t('opsStatsEmptyHint')"
        >
          <template #action>
            <button type="button" class="mts-btn-primary text-xs" data-testid="ops-memory-stats-retry" :disabled="!!sectionRetrying.memory || loading" @click="retryOpsSection('memory')">
              {{ sectionRetrying.memory ? t('loading') : t('overviewSectionRetry') }}
            </button>
          </template>
        </EmptyState>
        <dl v-else class="grid grid-cols-2 gap-2 text-xs text-slate-600 dark:text-slate-300 md:grid-cols-4">
          <div v-for="(v, k) in memorySnapshot" :key="String(k)">
            <span class="mts-muted">{{ k }}</span>: <b>{{ v }}</b>
          </div>
        </dl>
      </div>
    </div>

    <div id="ops-action-log" class="mts-card scroll-mt-20 p-4" data-testid="ops-action-log">
      <div class="mb-3 flex flex-wrap items-center justify-between gap-2">
        <h2 class="text-sm font-semibold">{{ t('opsActionLog') }}</h2>
        <div class="flex flex-wrap gap-2">
          <ExportJobBanner class="w-full basis-full" :job="exportJob" :retryable="canRetryExport" @cancel="cancelExport" @retry="retryLastExport" @dismiss="resetExport" />
        <button type="button" class="mts-btn" data-testid="ops-export-log" :disabled="exportBusy || !filteredActionLog.length" @click="exportActionLog">
            <Download class="h-3.5 w-3.5" />
            {{ t('opsExportLog') }}
          </button>
          <button type="button" class="mts-btn" data-testid="ops-clear-log" :disabled="!actionLog.length || writeBlocked || clearLogLoading" :title="writeBlocked ? t(blockedMessageKey('offlineOpsBlocked')) : undefined" @click="openClearLog">
            <Eraser class="h-3.5 w-3.5" />
            {{ t('opsClearLog') }}
          </button>
        </div>
      </div>
      <p class="mb-2 text-xs mts-muted">{{ t('opsActionLogHint') }}</p>
      <div id="ops-action-filter-bar" class="mb-3 flex flex-wrap items-end gap-2 scroll-mt-20" data-testid="ops-action-filter-bar">
        <label class="text-xs mts-muted">{{ t('opsActionFilterKind') }}
          <select v-model="actionKindFilter" class="mts-input mt-1" data-testid="ops-action-filter-kind">
            <option value="all">{{ t('opsActionFilterAll') }}</option>
            <option value="flush">flush</option>
            <option value="compact">compact</option>
            <option value="retention">retention</option>
            <option value="other">other</option>
          </select>
        </label>
        <label class="text-xs mts-muted">{{ t('opsActionFilterStatus') }}
          <select v-model="actionStatusFilter" class="mts-input mt-1" data-testid="ops-action-filter-status">
            <option value="all">{{ t('opsActionFilterAll') }}</option>
            <option value="ok">ok</option>
            <option value="error">error</option>
          </select>
        </label>
        <label class="text-xs mts-muted">{{ t('filter') }}
          <input
            v-model="actionTextFilter"
            type="search"
            class="mts-input mt-1 min-w-[12rem]"
            data-testid="ops-action-filter-search"
            :placeholder="t('opsActionFilterSearch')"
          />
        </label>
        <span class="text-xs mts-muted" data-testid="ops-action-filter-count">
          {{ formatMessage(t('opsActionFilterCount'), { shown: String(filteredActionLog.length), total: String(actionLog.length) }) }}
        </span>
      </div>
      <EmptyState
        v-if="!actionLog.length"
        compact
        :title="t('opsLogEmpty')"
        :description="t('opsLogEmptyDesc')"
      />
      <EmptyState
        v-else-if="!filteredActionLog.length"
        compact
        data-testid="ops-action-filter-empty"
        :title="t('opsActionFilterEmpty')"
        :description="t('opsLogEmptyDesc')"
      >
        <template #action>
          <button
            type="button"
            class="mts-btn-primary"
            data-testid="ops-action-clear-filters"
            @click="actionKindFilter = 'all'; actionStatusFilter = 'all'; actionTextFilter = ''"
          >{{ t('clearFilters') }}</button>
        </template>
      </EmptyState>
      <div v-else class="overflow-hidden rounded-lg border border-slate-100 dark:border-slate-800">
        <VirtualTable
          :items="filteredActionLog"
          :row-height="ACTION_ROW_HEIGHT"
          :height="Math.min(ACTION_LIST_HEIGHT, Math.max(144, filteredActionLog.length * ACTION_ROW_HEIGHT))"
          data-testid="ops-action-virtual-list"
        >
          <template #default="{ item }">
            <div
              class="flex h-full items-start justify-between gap-2 border-b border-slate-100 px-2 py-1.5 text-xs dark:border-slate-800"
              :data-testid="`ops-action-row-${item.id}`"
            >
              <div class="min-w-0">
                <span
                  class="mr-2 rounded px-1.5 py-0.5 font-medium"
                  :class="item.status === 'ok' ? 'bg-emerald-50 text-emerald-800 dark:bg-emerald-950/40 dark:text-emerald-200' : 'bg-red-50 text-red-700 dark:bg-red-950/40 dark:text-red-200'"
                >{{ item.kind }} · {{ item.status }}</span>
                <span class="text-slate-700 dark:text-slate-200">{{ item.message }}</span>
              </div>
              <span class="shrink-0 mts-muted">{{ formatAt(item.at) }}</span>
            </div>
          </template>
        </VirtualTable>
        <p class="border-t border-slate-100 px-3 py-1.5 text-[11px] mts-muted dark:border-slate-800" data-testid="ops-action-virtual-hint">
          {{ t('opsActionVirtualHint') }}
        </p>
      </div>
    </div>

    <ConfirmDialog
      :open="!!confirmKind"
      :write-blocked="writeBlocked"
      :block-reason="blockReason"
      :offline-message-key="'offlineOpsBlocked'"
      :title="confirmKind ? confirmTitle[confirmKind] : ''"
      :message="confirmKind ? confirmMessage[confirmKind] : ''"
      :confirm-label="t('opsExecute')"
      :require-text="confirmRequireText"
      danger
      :loading="confirmLoading"
      allow-cancel-while-loading
      data-testid="ops-danger-confirm"
      @update:open="(v) => { if (!v) confirmKind = null }"
      @confirm="runConfirmed"
      @cancel="cancelOpsAction"
    />
    <ConfirmDialog
      v-model:open="clearLogOpen"
      :write-blocked="writeBlocked"
      :block-reason="blockReason"
      :offline-message-key="'offlineOpsBlocked'"
      :title="t('opsClearLogTitle')"
      :message="t('opsClearLogMsg')"
      :confirm-label="t('opsClearLog')"
      danger
      :loading="clearLogLoading"
      allow-cancel-while-loading
      data-testid="ops-clear-log-confirm"
      @confirm="confirmClearLog"
    />
  </div>
</template>
