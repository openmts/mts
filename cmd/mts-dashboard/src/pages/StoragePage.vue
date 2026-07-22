<script setup lang="ts">
import { computed, inject, ref, onMounted, onBeforeUnmount, watch, type ComputedRef } from 'vue'
import { useRoute } from 'vue-router'
import { apiPost, apiGet, apiDelete } from '@/api/client'
import { useMutationGuard } from '@/composables/useMutationGuard'
import { useAuth } from '@/composables/useAuth'
import { useAdminOpBusy } from '@/composables/useAdminOpBusy'
import { actionResultAdminBusyAction, adminOpKindLabelKey, adminHeavyBusyOpFromError, isAdminHeavyBusyError, joinAdminOpChip } from '@/utils/adminOpBusy'
import type { MessageKey } from '@/i18n/messages'
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
import { formatMessage } from '@/utils/formatMessage'
import { scheduleScrollToHash } from '@/utils/hashScroll'
import { useI18n } from '@/composables/useI18n'
import { textForLocale, type LocaleCode } from '@/utils/localizedText'
import { makeActionResult } from '@/utils/actionResult'
import { useActionRetry } from '@/composables/useActionRetry'
import { formatBytes } from '@/utils/formatBytes'
import { stampFilename } from '@/utils/download'
import { useExportJob } from '@/composables/useExportJob'
import ExportJobBanner from '@/components/ExportJobBanner.vue'
import { buildStorageConfigExport, formatStorageExportPretty } from '@/utils/storageExport'
import { copyText } from '@/utils/clipboard'
import { storageFormToPrefill, parseStoragePrefill } from '@/utils/routePrefill'
import {
  defaultSelectedSnapshotPath,
  selectableDataSnapshots,
} from '@/utils/storageSnapshots'
import { BACKUP_DRILL_STEPS, drillProgress } from '@/utils/backupDrill'
import { EDGE_HTTPS_ACCEPTANCE_STEPS, edgeHttpsProgress } from '@/utils/edgeHttpsAcceptance'
import { completedIds, loadReadinessState, setReadinessFlag } from '@/utils/readinessState'
import { CheckCircle, Camera, Download, Trash2, RefreshCw, ClipboardList } from 'lucide-vue-next'

interface ValidateResponse { ok: boolean; data_dir: string; health: Record<string, unknown> }
interface SnapshotResponse { ok: boolean; path: string }
interface DataSnapshotResponse { ok: boolean; path: string; source?: string; files?: number; bytes?: number }
interface RestoreDrillResponse {
  ok: boolean
  source: string
  target: string
  files?: number
  bytes?: number
  check_issues?: number
  check_fatals?: number
  check_root?: string
}
interface DataSnapshotInfo { name: string; kind: string; path: string; size_bytes: number; mod_time: string }
interface DataSnapshotsResponse { snapshots: DataSnapshotInfo[] }
interface SnapshotInfo { name: string; path: string; size_bytes: number; mod_time: string }
interface SnapshotsResponse { snapshots: SnapshotInfo[] }
interface ExportData { generated_at: string; config: Record<string, unknown>; health: Record<string, unknown> }
interface ExportResponse { export: ExportData }

const route = useRoute()
const { isAdmin } = useAuth()
const { adminOpBusy, adminOpKind, adminOpBusyChecking, setAdminOpBusy, refreshAdminOpBusy } = useAdminOpBusy()
const storageBusyRefreshing = ref(false)
async function refreshStorageBusyOnly() {
  if (storageBusyRefreshing.value || adminOpBusyChecking.value) return
  storageBusyRefreshing.value = true
  try {
    await refreshAdminOpBusy()
  } finally {
    storageBusyRefreshing.value = false
  }
}
const { offline, writeBlocked, blockReason, blockedMessageKey } = useMutationGuard()
const { success, info, error: notifyError } = useNotify()
const { notifyAdminBusy, notifyMaybeAdminBusy } = useNotifyAdminBusy({ busyMessageKey: 'storageAdminBusy' })
const {
  exportJob,
  exportBusy,
  cancelExport,
  resetExport,
  retryLastExport,
  canRetryExport,
  runJSONExport,
} = useExportJob()
const { t , locale } = useI18n()
const adminOpBusySummary = inject<ComputedRef<{ busy: boolean; opLabel: string; elapsed: string; detail: string; lastSummary?: string; lastError?: string; lastOk?: boolean | null }> | undefined>('adminOpBusySummary', undefined)
const storageAdminLastLabel = computed(() => (adminOpBusySummary?.value?.lastSummary || '').trim())
const storageAdminLastErrorDetail = computed(() => {
  if (adminOpBusySummary?.value?.lastOk !== false) return ''
  return (adminOpBusySummary?.value?.lastError || '').trim()
})
const storageAdminBusyChipLabel = computed(() => {
  if (!adminOpBusy.value) return t.value('storageAdminBusyChip')
  const key = adminOpKindLabelKey(adminOpKind.value) as MessageKey
  const kind = adminOpBusySummary?.value?.opLabel || t.value(key) || t.value('adminOpKindGeneric')
  const base = joinAdminOpChip(t.value('storageAdminBusyChip'), kind)
  const elapsed = adminOpBusySummary?.value?.elapsed
  return elapsed ? `${base} · ${elapsed}` : base
})
const storageAdminBusyTitle = computed(() => adminOpBusySummary?.value?.detail || t.value('storageAdminBusy'))
const uiLocale = computed<LocaleCode>(() => (locale.value === 'en' ? 'en' : 'zh'))
const validateResult = ref<ValidateResponse | null>(null)
const snapshotResult = ref<SnapshotResponse | null>(null)
const dataSnapshotResult = ref<DataSnapshotResponse | null>(null)
const restoreDrillResult = ref<RestoreDrillResponse | null>(null)
const dataSnapshots = ref<DataSnapshotInfo[]>([])
const selectedDataSnapshotPath = ref('')
const snapshots = ref<SnapshotInfo[]>([])
const exportData = ref<ExportData | null>(null)
type StorageActionKey = 'validate' | 'snapshot' | 'data-snapshot' | 'restore-side' | 'export-config' | 'delete'
const {
  lastFailedAction,
  actionResult,
  canRetryAction,
  clearActionResult,
  setActionOk,
  setActionError,
  setActionResult,
  reportActionError: reportRetryError,
} = useActionRetry<StorageActionKey>()
const storageAdminBusyAction = computed(() =>
  actionResultAdminBusyAction({
    message: actionResult.value?.message || '',
    openLabel: t.value('adminOpBusyOpenOps'),
  }),
)
const snapshotListError = ref('')
const dataListLoading = ref(false)
const dataListError = ref('')
const loading = ref('')
const actionStartedAt = ref<number | null>(null)
const actionAbort = createActionAbort()
const listLoading = ref(false)
const SNAPSHOT_ROW_HEIGHT = 44
const DATA_ROW_HEIGHT = 48
const SNAPSHOT_LIST_HEIGHT = 320
const deleteOpen = ref(false)
const deleteName = ref('')
const deleteLoading = ref(false)
const readiness = ref(loadReadinessState())
const edgeDone = computed(() => readiness.value.edgeHttps)
const edgeSteps = EDGE_HTTPS_ACCEPTANCE_STEPS
const edgeStats = computed(() => edgeHttpsProgress(completedIds(edgeDone.value)))
const drillDone = ref<Record<string, boolean>>({
  validate: false,
  snapshot: false,
  'data-snapshot': false,
  'restore-side': false,
  'export-config': false,
})
const drillSteps = BACKUP_DRILL_STEPS
const drillStats = computed(() => drillProgress(Object.entries(drillDone.value).filter(([, v]) => v).map(([k]) => k)))
function toggleEdge(id: string, checked: boolean) {
  readiness.value = setReadinessFlag('edgeHttps', id, checked)
}

function toggleHostDrill(id: string, checked: boolean) {
  drillDone.value = { ...drillDone.value, [id]: checked }
}

async function loadSnapshots(opts?: { soft?: boolean }) {
  const soft = !!opts?.soft
  if (!soft) listLoading.value = true
  try {
    const data = await apiGet<SnapshotsResponse>('/api/v1/admin/storage/snapshots')
    snapshots.value = data.snapshots ?? []
    snapshotListError.value = ''
  } catch (e) {
    const msg = formatCaughtError(e)
    if (soft && snapshots.value.length) {
      snapshotListError.value = msg
    } else {
      snapshots.value = []
      snapshotListError.value = msg
    }
  } finally {
    if (!soft) listLoading.value = false
  }
}

async function loadDataSnapshots(opts?: { soft?: boolean }) {
  const soft = !!opts?.soft
  dataListLoading.value = true
  try {
    const data = await apiGet<DataSnapshotsResponse>('/api/v1/admin/storage/data-snapshots')
    dataSnapshots.value = data.snapshots ?? []
    selectedDataSnapshotPath.value = defaultSelectedSnapshotPath(
      dataSnapshots.value,
      selectedDataSnapshotPath.value || dataSnapshotResult.value?.path || null,
    )
    dataListError.value = ''
  } catch (e) {
    const msg = formatCaughtError(e)
    if (soft && dataSnapshots.value.length) {
      dataListError.value = msg
    } else {
      dataSnapshots.value = []
      selectedDataSnapshotPath.value = ''
      dataListError.value = msg
    }
  } finally {
    dataListLoading.value = false
  }
}

async function reloadStorageLists() {
  snapshotListError.value = ''
  dataListError.value = ''
  await loadSnapshots()
  await loadDataSnapshots()
}


function reportActionError(key: StorageActionKey, e: unknown) {
  reportRetryError(key, e)
  const msg = actionResult.value?.message || formatCaughtError(e)
  if (isAdminHeavyBusyError(e)) notifyAdminBusy(msg)
  else notifyError(msg)
}

async function retryLastStorageAction() {
  const key = lastFailedAction.value as StorageActionKey | null
  if (!key) return
  if (key === 'validate') return doValidate()
  if (key === 'snapshot') return doSnapshot()
  if (key === 'data-snapshot') return doDataSnapshot()
  if (key === 'restore-side') return doRestoreDrill()
  if (key === 'export-config') return doExport()
  if (key === 'delete' && deleteName.value) {
    deleteOpen.value = true
    return confirmDelete()
  }
}

function scrollToCurrentHash() {
  if (typeof window === 'undefined') return
  scheduleScrollToHash(window.location.hash || route.hash)
}

onMounted(() => {
  if (isAdmin.value) {
    void refreshAdminOpBusy()
    void loadSnapshots()
    void loadDataSnapshots()
  }
  scrollToCurrentHash()
  if (typeof window !== 'undefined') {
    window.addEventListener('hashchange', scrollToCurrentHash)
  }
})

onBeforeUnmount(() => {
  cancelStorageAction()
  if (typeof window !== 'undefined') {
    window.removeEventListener('hashchange', scrollToCurrentHash)
  }
})

watch(
  () => route.hash,
  () => {
    scrollToCurrentHash()
  },
)



function guardAdminHeavy(): boolean {
  if (writeBlocked.value) {
    const msg = t.value(blockedMessageKey('offlineAdminBlocked'))
    setActionError(msg)
    notifyError(msg)
    return false
  }
  if (loading.value) return false
  if (adminOpBusy.value) {
    notifyAdminBusy()
    return false
  }
  return true
}

function beginStorageAction(key: string): AbortSignal {
  loading.value = key
  actionStartedAt.value = Date.now()
  clearActionResult()
  return actionAbort.begin()
}

function endStorageAction() {
  actionAbort.end()
  loading.value = ''
  actionStartedAt.value = null
  void refreshAdminOpBusy()
}

function cancelStorageAction() {
  actionAbort.cancel()
}

function reportStorageCatch(key: StorageActionKey, e: unknown) {
  if (isAdminHeavyBusyError(e)) {
    setAdminOpBusy(true, adminHeavyBusyOpFromError(e) || undefined)
    void refreshAdminOpBusy()
  }
  if (isCanceledError(e)) {
    const msg = t.value('storageActionCancelled')
    setActionResult(makeActionResult('info', msg))
    info(msg)
    return
  }
  if (isTimeoutError(e)) {
    const msg = t.value('storageActionTimedOut')
    setActionResult(makeActionResult('error', msg))
    notifyError(msg)
    return
  }
  reportActionError(key, e)
}

async function doValidate() {
  if (loading.value) return
  if (writeBlocked.value) {
    const msg = t.value(blockedMessageKey('offlineAdminBlocked'))
    setActionError(msg)
    notifyError(msg)
    return
  }
  const signal = beginStorageAction('validate')
  try {
    validateResult.value = await apiPost<ValidateResponse>('/api/v1/admin/storage/validate', undefined, { signal })
    drillDone.value = { ...drillDone.value, validate: true }
    const msg = validateResult.value.ok ? t.value('storageValidateOk') : t.value('storageValidateDone')
    setActionResult(makeActionResult(validateResult.value.ok ? 'ok' : 'warn', msg))
    success(msg)
  } catch (e) {
    reportStorageCatch('validate', e)
  } finally { endStorageAction() }
}

async function doSnapshot() {
  if (!guardAdminHeavy()) return
  setAdminOpBusy(true, 'config_snapshot')
  const signal = beginStorageAction('snapshot')
  try {
    snapshotResult.value = await apiPost<SnapshotResponse>('/api/v1/admin/storage/snapshot', undefined, { signal })
    drillDone.value = { ...drillDone.value, snapshot: true }
    const msg = `${t.value('createSnapshot')}：${snapshotResult.value.path || 'ok'}`
    setActionOk(msg)
    success(t.value('createSnapshot'))
    await loadSnapshots()
  } catch (e) {
    reportStorageCatch('snapshot', e)
  } finally { endStorageAction() }
}

async function doDataSnapshot() {
  if (!guardAdminHeavy()) return
  setAdminOpBusy(true, 'data_snapshot')
  const signal = beginStorageAction('data-snapshot')
  try {
    dataSnapshotResult.value = await apiPost<DataSnapshotResponse>('/api/v1/admin/storage/data-snapshot', { flush: true }, { signal })
    if (dataSnapshotResult.value.path) selectedDataSnapshotPath.value = dataSnapshotResult.value.path
    drillDone.value = { ...drillDone.value, 'data-snapshot': true }
    const msg = formatMessage(t.value('storageDataSnapshotMsg'), { path: dataSnapshotResult.value.path || 'ok', files: dataSnapshotResult.value.files ?? 0 })
    setActionOk(msg)
    success(t.value('storageDataSnapshotOk'))
    await loadDataSnapshots()
  } catch (e) {
    reportStorageCatch('data-snapshot', e)
  } finally { endStorageAction() }
}

async function doRestoreDrill() {
  if (!guardAdminHeavy()) return
  setAdminOpBusy(true, 'restore_drill')
  const signal = beginStorageAction('restore-drill')
  try {
    const source = selectedDataSnapshotPath.value || dataSnapshotResult.value?.path || ''
    const body = source ? { source_path: source } : {}
    restoreDrillResult.value = await apiPost<RestoreDrillResponse>('/api/v1/admin/storage/restore-drill', body, { signal })
    const ok = !!restoreDrillResult.value.ok && (restoreDrillResult.value.check_fatals ?? 0) === 0
    drillDone.value = { ...drillDone.value, 'restore-side': ok }
    const msg = ok
      ? formatMessage(t.value('storageRestoreDone'), { target: restoreDrillResult.value.target })
      : formatMessage(t.value('storageRestoreFatal'), { fatals: restoreDrillResult.value.check_fatals ?? '?' })
    setActionResult(makeActionResult(ok ? 'ok' : 'warn', msg))
    if (ok) success(t.value('storageRestoreOk'))
    else notifyError(msg)
    await loadDataSnapshots()
  } catch (e) {
    reportStorageCatch('restore-side', e)
  } finally { endStorageAction() }
}

async function doExport() {
  if (loading.value) return
  if (writeBlocked.value) {
    const msg = t.value(blockedMessageKey('offlineAdminBlocked'))
    setActionError(msg)
    notifyError(msg)
    return
  }
  const signal = beginStorageAction('export')
  try {
    const data = await apiGet<ExportResponse>('/api/v1/admin/storage/export', { signal })
    exportData.value = data.export
    drillDone.value = { ...drillDone.value, 'export-config': true }
    setActionOk(t.value('storageConfigExported'))
    success(t.value('storageConfigExportToast'))
  } catch (e) {
    reportStorageCatch('export-config', e)
  } finally { endStorageAction() }
}

async function downloadExport() {
  if (!exportData.value) {
    notifyError(t.value('storageExportEmpty'))
    return
  }
  if (exportBusy.value) return
  const payload = buildStorageConfigExport(exportData.value)
  const outcome = await runJSONExport({
    label: 'JSON',
    filename: stampFilename('mts-storage-export', 'json'),
    total: 1,
    build: async ({ isCancelled, progress }) => {
      progress(0, 1)
      if (isCancelled()) return null
      progress(1, 1)
      return payload
    },
  })
  if (outcome === 'done') {
    setActionOk(t.value('storageDownloadStarted'))
    success(t.value('storageDownloadToast'))
  } else if (outcome === 'cancelled') {
    info(t.value('exportCancelledToast'))
  } else if (outcome === 'error') {
    notifyError(exportJob.value.error || t.value('failed'))
  }
}

async function copyExport() {
  if (!exportData.value) {
    notifyError(t.value('storageExportEmpty'))
    return
  }
  const res = await copyText(formatStorageExportPretty(exportData.value))
  if (res.ok) success(t.value('storageExportCopied'))
  else notifyError(res.error || t.value('failed'))
}

const drillSourceOptions = computed(() => selectableDataSnapshots(dataSnapshots.value))

function useSnapshotAsDrillSource(path: string) {
  if (!path) return
  selectedDataSnapshotPath.value = path
  success(t.value('storageUseAsDrillSource'))
}

async function copySnapshotPath(path: string) {
  if (!path) return
  const res = await copyText(path)
  if (res.ok) success(t.value('storagePathCopied'))
  else notifyError(res.error || t.value('failed'))
}

function requestDelete(name: string) {
  if (writeBlocked.value) {
    const msg = t.value(blockedMessageKey('offlineAdminBlocked'))
    setActionError(msg)
    notifyError(msg)
    return
  }
  deleteName.value = name
  deleteOpen.value = true
}

async function confirmDelete() {
  if (writeBlocked.value) {
    const msg = t.value(blockedMessageKey('offlineAdminBlocked'))
    setActionError(msg)
    notifyError(msg)
    return
  }
  const signal = beginStorageAction('delete')
  deleteLoading.value = true
  try {
    await apiDelete(`/api/v1/admin/storage/snapshots?name=${encodeURIComponent(deleteName.value)}`, { signal })
    deleteOpen.value = false
    setActionOk(formatMessage(t.value('storageSnapshotDeleted'), { name: deleteName.value }))
    success(t.value('storageSnapshotDeletedToast'))
    await loadSnapshots()
  } catch (e) {
    reportStorageCatch('delete', e)
  } finally {
    deleteLoading.value = false
    endStorageAction()
  }
}


function currentStorageSection(): string {
  const h = (route.hash || '').replace(/^#/, '')
  if (h === 'backup-drill' || h === 'edge-https' || h === 'data-restore' || h === 'snapshots') return h
  const pre = parseStoragePrefill(route.query as Record<string, unknown>, route.hash)
  return pre.section || 'backup-drill'
}

async function copyStorageShareLink() {
  const path = storageFormToPrefill({ section: currentStorageSection() })
  const url = `${window.location.origin}${path}`
  const res = await copyText(url)
  if (res.ok) success(t.value('storageShareCopied'))
  else notifyError(res.error || t.value('failed'))
}
</script>

<template>
  <PermissionDenied v-if="!isAdmin" />
  <div v-else class="space-y-6" data-testid="storage-page">
    <div class="flex flex-wrap items-center justify-between gap-3">
      <div>
        <h1 class="text-lg font-semibold text-slate-800 dark:text-slate-100">{{ t('storage') }}</h1>
        <p class="text-xs mts-muted">{{ t('storageSubtitle') }}</p>
      </div>
      <div class="flex flex-wrap gap-2">
        <button type="button" class="mts-btn" data-testid="storage-share-link" @click="copyStorageShareLink">
          {{ t('storageShareLink') }}
        </button>
        <button
          type="button"
          class="mts-btn"
          data-testid="storage-refresh"
          :disabled="listLoading"
          :aria-busy="listLoading ? 'true' : undefined"
          @click="() => { void reloadStorageLists() }"
        >
          <RefreshCw class="h-3.5 w-3.5" :class="listLoading ? 'animate-spin' : ''" /> {{ t('storageRefreshSnapshots') }}
        </button>
      </div>
    </div>

    <ActionResultBanner
      v-if="snapshotListError && !snapshots.length"
      kind="error"
      :message="snapshotListError"
      retryable
      data-testid="storage-list-error"
      @retry="() => loadSnapshots()"
      @dismiss="snapshotListError = ''"
    />
    <PartialErrorBanner
      v-else-if="snapshotListError"
      :message="`${t('storageSnapshotsRefreshFailed')}：${snapshotListError}`"
      test-id="storage-snapshots-refresh-error"
      @retry="() => loadSnapshots({ soft: true })"
      @dismiss="snapshotListError = ''"
    />
    <PartialErrorBanner
      v-if="dataListError && dataSnapshots.length"
      :message="`${t('storageDataSnapshotsRefreshFailed')}：${dataListError}`"
      test-id="storage-data-refresh-error"
      @retry="() => loadDataSnapshots({ soft: true })"
      @dismiss="dataListError = ''"
    />
    <ActionResultBanner
      v-else-if="dataListError && !dataSnapshots.length"
      kind="error"
      :message="dataListError"
      retryable
      data-testid="storage-data-list-error"
      @retry="() => loadDataSnapshots()"
      @dismiss="dataListError = ''"
    />
    <ActionResultBanner
      :result="actionResult"
      :retryable="canRetryAction"
      :action-label="storageAdminBusyAction?.label || ''"
      :action-path="storageAdminBusyAction?.path || ''"
      data-testid="storage-action-result"
      @retry="retryLastStorageAction"
      @dismiss="clearActionResult"
    />

    <div id="backup-drill" class="mts-card p-4 scroll-mt-20">
      <div class="mb-2 flex flex-wrap items-center justify-between gap-2">
        <h2 class="flex items-center gap-2 text-sm font-semibold text-slate-800 dark:text-slate-100">
          <ClipboardList class="h-4 w-4" />
          {{ t('storageBackupDrill') }}
        </h2>
        <span class="text-xs mts-muted">
          {{ t('storageProgress') }} {{ drillStats.completed }}/{{ drillStats.total }}
          · {{ t('storageRequired') }} {{ drillStats.requiredCompleted }}/{{ drillStats.requiredTotal }}
        </span>
      </div>
      <p class="mb-3 text-xs mts-muted">
        {{ t('storageBackupDrillHint') }}
      </p>
      <ol class="space-y-2">
        <li
          v-for="step in drillSteps"
          :key="step.id"
          class="flex items-start gap-2 rounded-lg border border-slate-100 px-3 py-2 text-sm dark:border-slate-800"
        >
          <input
            v-if="step.inDashboard"
            type="checkbox"
            class="mt-1"
            :checked="!!drillDone[step.id]"
            disabled
          />
          <input
            v-else
            type="checkbox"
            class="mt-1"
            :checked="!!drillDone[step.id]"
            @change="toggleHostDrill(step.id, ($event.target as HTMLInputElement).checked)"
          />
          <div class="min-w-0">
            <p class="font-medium text-slate-800 dark:text-slate-100">
              {{ textForLocale(step.title, uiLocale) }}
              <span class="ml-1 text-[11px] font-normal mts-muted">{{ step.severity === 'required' ? t('storageRequired') : t('storageRecommended') }}</span>
              <span v-if="step.inDashboard" class="ml-1 text-[11px] font-normal text-emerald-700 dark:text-emerald-300">{{ t('storageInDashboard') }}</span>
            </p>
            <p class="text-xs mts-muted">{{ textForLocale(step.detail, uiLocale) }}</p>
          </div>
        </li>
      </ol>
    </div>


    <div id="edge-https" class="mts-card p-4 scroll-mt-20">
      <div class="mb-2 flex flex-wrap items-center justify-between gap-2">
        <h2 class="flex items-center gap-2 text-sm font-semibold text-slate-800 dark:text-slate-100">
          {{ t('storageEdgeHttps') }}
        </h2>
        <span class="text-xs mts-muted">
          {{ t('storageProgress') }} {{ edgeStats.done }}/{{ edgeStats.total }}
          · {{ t('storageRequired') }} {{ edgeStats.requiredDone }}/{{ edgeStats.requiredTotal }}
        </span>
      </div>
      <p class="mb-3 text-xs mts-muted">
        {{ t('storageEdgeHttpsHint') }}
      </p>
      <ol class="space-y-2">
        <li
          v-for="step in edgeSteps"
          :key="step.id"
          class="flex items-start gap-2 rounded-lg border border-slate-100 px-3 py-2 text-sm dark:border-slate-800"
        >
          <input
            type="checkbox"
            class="mt-1"
            :checked="!!edgeDone[step.id]"
            @change="toggleEdge(step.id, ($event.target as HTMLInputElement).checked)"
          />
          <div class="min-w-0">
            <p class="font-medium text-slate-800 dark:text-slate-100">
              {{ textForLocale(step.title, uiLocale) }}
              <span class="ml-1 text-[11px] font-normal mts-muted">{{ step.severity === 'required' ? t('storageRequired') : t('storageRecommended') }}</span>
              <span v-if="step.partialAutomated" class="ml-1 text-[11px] font-normal text-emerald-700 dark:text-emerald-300">{{ t('storagePartialAuto') }}</span>
            </p>
            <p class="text-xs mts-muted">{{ textForLocale(step.detail, uiLocale) }}</p>
          </div>
        </li>
      </ol>
    </div>

    <div class="grid grid-cols-1 gap-4 sm:grid-cols-3">
      <div class="mts-panel">
        <div class="mb-3 flex items-center gap-2"><CheckCircle class="h-5 w-5 text-slate-500" /><h3 class="text-sm font-semibold">{{ t('storageValidateTitle') }}</h3></div>
        <button data-testid="storage-validate" :disabled="loading === 'validate' || writeBlocked" :title="writeBlocked ? t(blockedMessageKey('offlineAdminBlocked')) : undefined" class="mts-btn-primary w-full justify-center py-2" @click="doValidate">{{ loading === 'validate' ? t('loading') : t('storageValidateRun') }}</button>
        <pre v-if="validateResult" class="mt-3 max-h-40 overflow-auto rounded bg-slate-950 p-2 text-[11px] text-emerald-400">{{ JSON.stringify(validateResult, null, 2) }}</pre>
      </div>
      <div class="mts-panel">
        <div class="mb-3 flex items-center gap-2"><Camera class="h-5 w-5 text-slate-500" /><h3 class="text-sm font-semibold">{{ t('createSnapshot') }}</h3></div>
        <button data-testid="storage-snapshot" :disabled="loading === 'snapshot' || writeBlocked || adminOpBusy" :title="writeBlocked ? t(blockedMessageKey('offlineAdminBlocked')) : (adminOpBusy ? storageAdminBusyTitle : undefined)" class="mts-btn-primary w-full justify-center py-2" @click="doSnapshot">{{ loading === 'snapshot' ? t('loading') : t('createSnapshot') }}</button>
        <p v-if="snapshotResult?.path" class="mt-2 break-all font-mono text-[11px] mts-muted">{{ snapshotResult.path }}</p>
      </div>
      <div class="mts-panel">
        <div class="mb-3 flex items-center gap-2"><Download class="h-5 w-5 text-slate-500" /><h3 class="text-sm font-semibold">{{ t('export') }}</h3></div>
        <button data-testid="storage-export-fetch" :disabled="loading === 'export' || writeBlocked" :title="writeBlocked ? t(blockedMessageKey('offlineAdminBlocked')) : undefined" class="mts-btn-primary w-full justify-center py-2" @click="doExport">{{ loading === 'export' ? t('loading') : t('export') }}</button>
        <ExportJobBanner class="w-full basis-full" :job="exportJob" :retryable="canRetryExport" @cancel="cancelExport" @retry="retryLastExport" @dismiss="resetExport" />
        <button v-if="exportData" type="button" class="mts-btn mt-2 w-full justify-center" data-testid="storage-export-download" :disabled="exportBusy" @click="downloadExport">{{ t('storageDownloadJson') }}</button>
        <button v-if="exportData" type="button" class="mts-btn mt-2 w-full justify-center" data-testid="storage-export-copy" @click="copyExport">{{ t('storageCopyExport') }}</button>
      </div>
    </div>

    <div class="mts-card overflow-hidden">
      <div class="flex items-center justify-between border-b border-slate-100 px-4 py-2 text-xs mts-muted dark:border-slate-800">
        <span>{{ t('snapshots') }}</span>
        <span>{{ formatMessage(t('storageSnapshotCount'), { count: snapshots.length }) }}</span>
      </div>
      <EmptyState
        v-if="listLoading"
        compact
        :title="t('loading')"
        :description="t('storageLoadingSnapshots')"
      />
      <EmptyState
        v-else-if="!snapshots.length"
        :title="t('storageNoSnapshots')"
        :description="t('storageNoSnapshotsDesc')"
      >
        <template #action>
          <button type="button" class="mts-btn-primary" :disabled="loading === 'snapshot' || writeBlocked || adminOpBusy" :title="writeBlocked ? t(blockedMessageKey('offlineAdminBlocked')) : (adminOpBusy ? storageAdminBusyTitle : undefined)" @click="doSnapshot">{{ t('createSnapshot') }}</button>
        </template>
      </EmptyState>
      <div id="snapshots" class="scroll-mt-20" data-testid="storage-snapshots-table">
        <div
          class="grid grid-cols-[minmax(10rem,1.4fr)_minmax(5rem,0.6fr)_minmax(8rem,1fr)_3rem] border-b border-slate-200 text-left text-[11px] uppercase mts-muted dark:border-slate-700"
          data-testid="storage-snapshots-header"
        >
          <div class="px-4 py-2">{{ t('storageColName') }}</div>
          <div class="px-4 py-2">{{ t('storageColSize') }}</div>
          <div class="px-4 py-2">{{ t('storageColTime') }}</div>
          <div class="px-4 py-2"></div>
        </div>
        <VirtualTable
          :items="snapshots"
          :row-height="SNAPSHOT_ROW_HEIGHT"
          :height="Math.min(SNAPSHOT_LIST_HEIGHT, Math.max(176, snapshots.length * SNAPSHOT_ROW_HEIGHT))"
          data-testid="storage-snapshots-virtual-list"
        >
          <template #default="{ item: s }">
            <div
              class="grid h-full grid-cols-[minmax(10rem,1.4fr)_minmax(5rem,0.6fr)_minmax(8rem,1fr)_3rem] items-center border-b border-slate-100 dark:border-slate-800"
              :data-testid="`storage-snapshot-row-${s.name}`"
            >
              <div class="truncate px-4 font-mono text-xs" :title="s.name">{{ s.name }}</div>
              <div class="px-4 text-xs">{{ formatBytes(s.size_bytes) }}</div>
              <div class="truncate px-4 text-xs mts-muted" :title="s.mod_time">{{ s.mod_time }}</div>
              <div class="px-2 text-right">
                <button type="button" class="rounded p-1 text-slate-400 hover:text-red-600 disabled:cursor-not-allowed disabled:opacity-40" :disabled="writeBlocked" :title="writeBlocked ? t(blockedMessageKey('offlineAdminBlocked')) : t('delete')" :data-testid="`storage-delete-${s.name}`" @click="requestDelete(s.name)">
                  <Trash2 class="h-4 w-4" />
                </button>
              </div>
            </div>
          </template>
        </VirtualTable>
        <p class="border-t border-slate-100 px-3 py-1.5 text-[11px] mts-muted dark:border-slate-800" data-testid="storage-snapshots-virtual-hint">
          {{ t('storageVirtualHint') }}
        </p>
      </div>
    </div>

    <div v-if="exportData" class="mts-panel">
      <h3 class="mb-2 text-sm font-semibold">{{ t('storageExportPreview') }}</h3>
      <pre class="max-h-96 overflow-auto rounded-lg bg-slate-900 p-4 text-xs text-green-400">{{ JSON.stringify(exportData, null, 2) }}</pre>
    </div>

    
    <div id="data-restore" class="mts-panel scroll-mt-20">
      <div class="mb-3 flex flex-wrap items-center justify-between gap-2">
        <h3 class="text-sm font-semibold text-slate-800 dark:text-slate-100">{{ t('storageDataDirRestore') }}</h3>
        <span class="text-xs mts-muted">{{ t('storageDataDirRestoreHint') }}</span>
      </div>
      <p class="mb-3 text-xs mts-muted">
        {{ t('storageDataDirRestoreDesc') }}
      </p>
      <div class="mb-3 space-y-1" data-testid="storage-drill-source">
        <label class="block text-xs font-medium text-slate-700 dark:text-slate-200" for="drill-source">{{ t('storageSelectDataSnapshot') }}</label>
        <select
          id="drill-source"
          v-model="selectedDataSnapshotPath"
          class="mts-input"
          data-testid="storage-drill-source-select"
          :disabled="!drillSourceOptions.length"
        >
          <option v-if="!drillSourceOptions.length" value="">{{ t('storageNoDataSnapshot') }}</option>
          <option v-for="s in drillSourceOptions" :key="s.path || s.name" :value="s.path || s.name">
            {{ s.name }} · {{ formatBytes(s.size_bytes || 0) }}
          </option>
        </select>
        <p class="text-[11px] mts-muted">{{ t('storageSelectDataSnapshotHint') }}</p>
      </div>
      <div class="grid grid-cols-1 gap-3 sm:grid-cols-2">
        <button data-testid="storage-data-snapshot"
          type="button"
          class="mts-btn-primary justify-center py-2"
          :disabled="loading === 'data-snapshot' || writeBlocked || adminOpBusy"
          :title="writeBlocked ? t(blockedMessageKey('offlineAdminBlocked')) : (adminOpBusy ? storageAdminBusyTitle : undefined)"
          @click="doDataSnapshot"
        >
          {{ loading === 'data-snapshot' ? t('loading') : t('storageCreateDataSnapshot') }}
        </button>
        <button
          type="button"
          class="mts-btn justify-center py-2"
          data-testid="storage-restore-drill"
          :disabled="loading === 'restore-drill' || writeBlocked || adminOpBusy"
          :title="writeBlocked ? t(blockedMessageKey('offlineAdminBlocked')) : (adminOpBusy ? storageAdminBusyTitle : undefined)"
          @click="doRestoreDrill"
        >
          {{ loading === 'restore-drill' ? t('loading') : t('storageRunRestoreDrill') }}
        </button>
      </div>
      <pre v-if="dataSnapshotResult" class="mt-3 max-h-32 overflow-auto rounded bg-slate-950 p-2 text-[11px] text-emerald-400">{{ JSON.stringify(dataSnapshotResult, null, 2) }}</pre>
      <pre v-if="restoreDrillResult" class="mt-2 max-h-32 overflow-auto rounded bg-slate-950 p-2 text-[11px] text-sky-300">{{ JSON.stringify(restoreDrillResult, null, 2) }}</pre>
      <EmptyState
        v-if="!dataListLoading && !dataSnapshots.length && !dataListError"
        class="mt-4"
        compact
        data-testid="storage-data-empty"
        :title="t('storageNoDataSnapshotsTitle')"
        :description="t('storageNoDataSnapshotsDesc')"
      >
        <template #action>
          <button
            type="button"
            class="mts-btn-primary"
            data-testid="storage-data-empty-create"
            :disabled="loading === 'data-snapshot' || writeBlocked || adminOpBusy"
            :title="writeBlocked ? t(blockedMessageKey('offlineAdminBlocked')) : undefined"
            @click="doDataSnapshot"
          >{{ t('storageCreateDataSnapshotCta') }}</button>
        </template>
      </EmptyState>
      <div v-if="dataSnapshots.length" class="mt-4 overflow-hidden rounded-lg border border-slate-100 dark:border-slate-800" data-testid="storage-data-table">
        <div
          class="grid grid-cols-[minmax(5rem,0.7fr)_minmax(8rem,1.2fr)_minmax(5rem,0.6fr)_minmax(7rem,1fr)_minmax(10rem,1.2fr)] border-b border-slate-200 text-left text-[11px] uppercase mts-muted dark:border-slate-700"
          data-testid="storage-data-header"
        >
          <div class="px-2 py-2">{{ t('storageColKind') }}</div>
          <div class="px-2 py-2">{{ t('storageColName') }}</div>
          <div class="px-2 py-2">{{ t('storageColSize') }}</div>
          <div class="px-2 py-2">{{ t('storageColTime') }}</div>
          <div class="px-2 py-2"></div>
        </div>
        <VirtualTable
          :items="dataSnapshots"
          :row-height="DATA_ROW_HEIGHT"
          :height="Math.min(SNAPSHOT_LIST_HEIGHT, Math.max(176, dataSnapshots.length * DATA_ROW_HEIGHT))"
          data-testid="storage-data-virtual-list"
        >
          <template #default="{ item: s }">
            <div
              class="grid h-full grid-cols-[minmax(5rem,0.7fr)_minmax(8rem,1.2fr)_minmax(5rem,0.6fr)_minmax(7rem,1fr)_minmax(10rem,1.2fr)] items-center border-b border-slate-100 dark:border-slate-800"
              :data-testid="`storage-data-row-${s.name}`"
            >
              <div class="truncate px-2 text-xs">{{ s.kind }}</div>
              <div class="truncate px-2 font-mono text-xs" :title="s.name">{{ s.name }}</div>
              <div class="px-2 text-xs">{{ formatBytes(s.size_bytes || 0) }}</div>
              <div class="truncate px-2 text-xs mts-muted" :title="s.mod_time">{{ s.mod_time }}</div>
              <div class="px-2 text-right text-xs">
                <button
                  v-if="s.kind !== 'restore-drill'"
                  type="button"
                  class="mts-btn mr-1"
                  :data-testid="`storage-use-source-${s.name}`"
                  @click="useSnapshotAsDrillSource(s.path || s.name)"
                >{{ t('storageUseAsDrillSource') }}</button>
                <button
                  type="button"
                  class="mts-btn"
                  :data-testid="`storage-copy-path-${s.name}`"
                  @click="copySnapshotPath(s.path || s.name)"
                >{{ t('storageCopyPath') }}</button>
              </div>
            </div>
          </template>
        </VirtualTable>
        <p class="border-t border-slate-100 px-3 py-1.5 text-[11px] mts-muted dark:border-slate-800" data-testid="storage-data-virtual-hint">
          {{ t('storageDataVirtualHint') }}
        </p>
      </div>
    </div>

    <div
      v-if="adminOpBusy && !loading"
      class="flex flex-wrap items-center justify-between gap-2 rounded-lg border border-sky-200 bg-sky-50 px-3 py-2 text-xs text-sky-900 dark:border-sky-900/50 dark:bg-sky-950/40 dark:text-sky-100"
      data-testid="storage-admin-busy"
      role="status"
      :title="storageAdminBusyTitle"
    >
      <span class="min-w-0">{{ storageAdminBusyChipLabel }} · {{ t('storageAdminBusy') }}</span>
      <button
        type="button"
        class="mts-btn text-xs shrink-0"
        data-testid="storage-admin-busy-refresh"
        :disabled="storageBusyRefreshing || adminOpBusyChecking"
        :aria-busy="storageBusyRefreshing || adminOpBusyChecking ? 'true' : undefined"
        @click="refreshStorageBusyOnly"
      >
        <RefreshCw class="h-3.5 w-3.5" :class="(storageBusyRefreshing || adminOpBusyChecking) ? 'animate-spin' : ''" />
        {{ t('storageAdminBusyRefresh') }}
      </button>
    </div>
    <div
      v-else-if="!loading && storageAdminLastLabel"
      class="flex flex-wrap items-center justify-between gap-2 rounded-lg border px-3 py-2 text-xs"
      :class="adminOpBusySummary?.lastOk === false
        ? 'border-red-200 bg-red-50 text-red-900 dark:border-red-900/50 dark:bg-red-950/40 dark:text-red-100'
        : (adminOpBusySummary?.lastOk === true
          ? 'border-emerald-200 bg-emerald-50 text-emerald-900 dark:border-emerald-900/50 dark:bg-emerald-950/40 dark:text-emerald-100'
          : 'border-slate-200 bg-slate-50 text-slate-700 dark:border-slate-800 dark:bg-slate-900/40 dark:text-slate-200')"
      data-testid="storage-admin-last"
      role="status"
      :data-ok="adminOpBusySummary?.lastOk === true ? 'true' : (adminOpBusySummary?.lastOk === false ? 'false' : undefined)"
      :title="storageAdminLastLabel"
    >
      <div class="min-w-0">
        <span class="min-w-0 truncate">
          <span class="font-medium">{{ t('opsStatusLastLabel') }}:</span>
          {{ storageAdminLastLabel }}
        </span>
        <p
          v-if="storageAdminLastErrorDetail"
          class="mt-1 break-all font-mono text-[11px] opacity-90"
          data-testid="storage-admin-last-error"
        >{{ t('adminOpLastErrorLabel') }}: {{ storageAdminLastErrorDetail }}</p>
      </div>
    </div>
    <InFlightBanner
      :active="!!loading"
      :started-at-ms="actionStartedAt"
      kind="storage"
      @cancel="cancelStorageAction"
    />
    <ConfirmDialog
      v-model:open="deleteOpen"
      :write-blocked="writeBlocked"
      :block-reason="blockReason"
      :offline-message-key="'offlineAdminBlocked'"
      :title="t('storageDeleteSnapshotTitle')"
      :message="formatMessage(t('storageDeleteSnapshotMsg'), { name: deleteName })"
      :confirm-label="t('delete')"
      danger
      :loading="deleteLoading"
      allow-cancel-while-loading
      @confirm="confirmDelete"
      @cancel="cancelStorageAction"
    />
  </div>
</template>
