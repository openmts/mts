<script setup lang="ts">
import { ref, computed, inject, onMounted, onBeforeUnmount, watch, nextTick, type ComputedRef } from 'vue'
import { useHashScroll } from '@/composables/useHashScroll'
import { useRoute, useRouter } from 'vue-router'
import { parseDownsamplePrefill, downsampleFormToPrefill } from '@/utils/routePrefill'
import { copyText } from '@/utils/clipboard'
import { apiGet, apiPost, apiDelete, apiPostNDJSONStream } from '@/api/client'
import { useAdminOpBusy } from '@/composables/useAdminOpBusy'
import { parseAdminOpStatusPayload } from '@/utils/adminOpBusy'
import { useMutationGuard } from '@/composables/useMutationGuard'
import {
  isDownsampleCreateDraftDirty,
  shouldBlockLeaveAdminCreate,
} from '@/utils/adminFormDirty'
import { registerDirtyChecker } from '@/utils/routeDirty'
import { listDatabases, listFields, listMeasurements } from '@/api/meta'
import { fieldNames } from '@/utils/seriesMeta'
import { useAuth } from '@/composables/useAuth'
import PermissionDenied from '@/components/PermissionDenied.vue'
import AdminOpLastChip from '@/components/AdminOpLastChip.vue'
import ConfirmDialog from '@/components/ConfirmDialog.vue'
import InFlightBanner from '@/components/InFlightBanner.vue'
import ActionResultBanner from '@/components/ActionResultBanner.vue'
import PartialErrorBanner from '@/components/PartialErrorBanner.vue'
import EmptyState from '@/components/EmptyState.vue'
import ListSelectionToolbar from '@/components/ListSelectionToolbar.vue'
import VirtualTable from '@/components/VirtualTable.vue'
import { useNotify } from '@/composables/useNotify'
import { useNotifyAdminBusy } from '@/composables/useNotifyAdminBusy'
import { actionResultAdminBusyAction } from '@/utils/adminOpBusy'
import { formatCaughtError, isCanceledError, isTimeoutError } from '@/utils/apiError'
import { createActionAbort } from '@/utils/actionAbort'
import {
  applyBatchProgressEvent,
  batchProgressPercent,
  emptyBatchProgress,
  type BatchProgressState,
  type BatchMutationSummary,
} from '@/utils/batchProgress'
import { formatMessage } from '@/utils/formatMessage'
import { createFocusTrap, type FocusTrapHandle } from '@/utils/focusTrap'
import { filterDownsamplePolicies, filterDownsampleStatuses, type DownsampleEnabledFilter, type DownsampleStatusHealthFilter } from '@/utils/listFilter'
import { useI18n } from '@/composables/useI18n'
import type { MessageKey } from '@/i18n/messages'
import { buildDownsampleExport, downsampleToCSV } from '@/utils/downsampleExport'
import { buildDownsamplePolicyDetailFields, downsamplePolicyDetailToJSONText, listDownsampleFunctions, downsamplePolicyDetailToMarkdownText } from '@/utils/downsamplePolicyDetail'
import { stampFilename } from '@/utils/download'
import { useExportJob } from '@/composables/useExportJob'
import ExportJobBanner from '@/components/ExportJobBanner.vue'
import { makeActionResult } from '@/utils/actionResult'
import { useActionRetry } from '@/composables/useActionRetry'
import { parseHumanDurationToNs, formatNsDuration } from '@/utils/duration'
import {
  buildDownsampleRangeBody,
  formatRunResultMessage,
  rangeActionPath,
  rangeErrorMessage,
  suggestDownsampleRange,
  validateDownsampleRange,
  type DownsampleRangeMode,
} from '@/utils/downsampleRange'
import {
  Plus, Trash2, Play, Pause, RefreshCw, PlayCircle, RotateCcw, FlaskConical, Wrench, Timer, Download, Eye
} from 'lucide-vue-next'
import type {
  DownsamplePolicy, DownsampleStatus, DownsampleRunResult, DownsampleDryRunResult,
} from '@/api/types'

interface PoliciesResponse {
  policies: DownsamplePolicy[]
  admin_op_busy?: boolean
  op?: string
  started_at_unix?: number
  last?: unknown
}
interface StatusesResponse {
  statuses: DownsampleStatus[]
  admin_op_busy?: boolean
  op?: string
  started_at_unix?: number
  last?: unknown
}

useHashScroll()
const route = useRoute()
const router = useRouter()
const { isAdmin } = useAuth()
const { offline, writeBlocked, blockReason, blockedMessageKey } = useMutationGuard()
const { t, locale } = useI18n()
const { applyAdminOpStatus } = useAdminOpBusy()
const { success, info, error: notifyError, warn } = useNotify()
const { notifyMaybeAdminBusy } = useNotifyAdminBusy()
const adminOpBusySummary = inject<ComputedRef<{ lastSummary?: string; lastError?: string; lastOk?: boolean | null }> | undefined>('adminOpBusySummary', undefined)
const downsampleAdminLastLabel = computed(() => (adminOpBusySummary?.value?.lastSummary || '').trim())
const downsampleAdminLastErrorDetail = computed(() => {
  if (adminOpBusySummary?.value?.lastOk !== false) return ''
  return (adminOpBusySummary?.value?.lastError || '').trim()
})

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
const policies = ref<DownsamplePolicy[]>([])
const statuses = ref<DownsampleStatus[]>([])
const loadError = ref('')
const policiesError = ref('')
const statusesError = ref('')
type DsActionKey = 'create' | 'delete' | 'toggle' | 'batch' | 'range' | 'run' | 'reset'
const {
  lastFailedAction,
  actionResult,
  canRetryAction,
  clearActionResult,
  setActionOk,
  setActionError,
  setActionResult,
  reportActionError: reportRetryError,
} = useActionRetry<DsActionKey>()
const dsAdminBusyAction = computed(() =>
  actionResultAdminBusyAction({
    message: actionResult.value?.message || '',
    openLabel: t.value('adminOpBusyOpenOps'),
  }),
)
const lastToggleName = ref('')
const lastToggleWantEnabled = ref(false)
const lastRunName = ref('')
const lastResetName = ref('')
const policyFilter = ref('')
const statusHealthFilter = ref<DownsampleStatusHealthFilter>('')
const statusMinLag = ref(0)
const enabledFilter = ref<DownsampleEnabledFilter>('')
const selectedNames = ref<string[]>([])
const detailPolicyName = ref('')
const detailMissingNotified = ref('')
const detailListReady = ref(false)
const loadDataInFlight = ref(false)
const batchOpen = ref(false)
const batchMode = ref<'enable' | 'disable'>('enable')
const batchLoading = ref(false)
const batchProgress = ref<BatchProgressState>(emptyBatchProgress())
const dsActionStartedAt = ref<number | null>(null)
const dsActionAbort = createActionAbort()
const createLoading = ref(false)
const dsToggleLoading = ref(false)
const dsRunLoading = ref(false)

const rangeOpen = ref(false)
const rangeMode = ref<DownsampleRangeMode>('repair')
const rangeName = ref('')
const rangeStartUnix = ref(0)
const rangeEndUnix = ref(0)
const rangeAdvanceWatermark = ref(false)
const rangeLoading = ref(false)
const showCreate = ref(false)
const intervalHuman = ref('1m')
const refreshHuman = ref('1m')
const lookbackHuman = ref('1m')
const deleteOpen = ref(false)
const deleteName = ref('')
const deleteLoading = ref(false)
const newPolicy = ref<DownsamplePolicy>({
  name: '',
  source_database: '',
  source_measurement: '',
  source_retention: 'autogen',
  target_database: '',
  target_measurement: '',
  target_retention: 'autogen',
  interval: 60_000_000_000,
  refresh_interval: 60_000_000_000,
  lookback: 60_000_000_000,
  batch_size: 100,
  functions: [{ function: 'mean', field: 'value', as: 'mean_value' }],
  group_by_tags: [],
  enabled: true,
})
const createDatabases = ref<string[]>([])
const createSourceMeasurements = ref<string[]>([])
const createSourceFields = ref<string[]>([])
const createMetaLoading = ref(false)
const createMetaError = ref('')

const rangePanelRef = ref<HTMLElement | null>(null)
const createPanelRef = ref<HTMLElement | null>(null)
let rangeTrap: FocusTrapHandle | null = null
let createTrap: FocusTrapHandle | null = null

function releaseRangeTrap() {
  rangeTrap?.release()
  rangeTrap = null
}
function releaseCreateTrap() {
  createTrap?.release()
  createTrap = null
}

watch(rangeOpen, async (open) => {
  releaseRangeTrap()
  if (!open) {
    if (!showCreate.value) document.body.style.overflow = ''
    return
  }
  document.body.style.overflow = 'hidden'
  await nextTick()
  if (rangePanelRef.value) {
    rangeTrap = createFocusTrap(rangePanelRef.value)
    rangeTrap.focusFirst()
  }
})

watch(showCreate, async (open) => {
  releaseCreateTrap()
  if (!open) {
    if (!rangeOpen.value) document.body.style.overflow = ''
    return
  }
  document.body.style.overflow = 'hidden'
  await nextTick()
  if (createPanelRef.value) {
    createTrap = createFocusTrap(createPanelRef.value)
    createTrap.focusFirst()
  }
})

onBeforeUnmount(() => {
  releaseRangeTrap()
  releaseCreateTrap()
  document.body.style.overflow = ''
})

async function loadCreateMetaDatabases() {
  try {
    createDatabases.value = await listDatabases()
    createMetaError.value = ''
  } catch (e) {
    const msg = formatCaughtError(e)
    if (createDatabases.value.length) createMetaError.value = msg
    else {
      createDatabases.value = []
      createMetaError.value = msg
    }
  }
}

async function loadCreateSourceMeasurements(db: string) {
  if (!db.trim()) {
    createSourceMeasurements.value = []
    createSourceFields.value = []
    return
  }
  createMetaLoading.value = true
  try {
    createSourceMeasurements.value = await listMeasurements(db)
    createSourceFields.value = []
    createMetaError.value = ''
  } catch (e) {
    const msg = formatCaughtError(e)
    if (createSourceMeasurements.value.length) createMetaError.value = msg
    else {
      createSourceMeasurements.value = []
      createSourceFields.value = []
      createMetaError.value = msg
    }
  } finally {
    createMetaLoading.value = false
  }
}

async function loadCreateSourceFields(db: string, measurement: string) {
  if (!db.trim() || !measurement.trim()) {
    createSourceFields.value = []
    return
  }
  createMetaLoading.value = true
  try {
    createSourceFields.value = fieldNames(await listFields(db, measurement))
    createMetaError.value = ''
  } catch (e) {
    const msg = formatCaughtError(e)
    if (createSourceFields.value.length) createMetaError.value = msg
    else {
      createSourceFields.value = []
      createMetaError.value = msg
    }
  } finally {
    createMetaLoading.value = false
  }
}

watch(showCreate, async (open) => {
  if (!open) return
  await loadCreateMetaDatabases()
  if (newPolicy.value.source_database) {
    await loadCreateSourceMeasurements(newPolicy.value.source_database)
  }
  if (newPolicy.value.source_database && newPolicy.value.source_measurement) {
    await loadCreateSourceFields(newPolicy.value.source_database, newPolicy.value.source_measurement)
  }
})

watch(
  () => newPolicy.value.source_database,
  async (db) => {
    if (!showCreate.value) return
    await loadCreateSourceMeasurements(db)
  },
)

watch(
  () => [newPolicy.value.source_database, newPolicy.value.source_measurement] as const,
  async ([db, m]) => {
    if (!showCreate.value) return
    await loadCreateSourceFields(db, m)
  },
)

function applyDownsamplePrefillFromRoute() {
  const pre = parseDownsamplePrefill(route.query as Record<string, unknown>)
  let changed = false
  if (pre.q != null && policyFilter.value !== pre.q) {
    policyFilter.value = pre.q
    changed = true
  }
  if (pre.enabled === 'enabled' || pre.enabled === 'disabled') {
    if (enabledFilter.value !== pre.enabled) {
      enabledFilter.value = pre.enabled as DownsampleEnabledFilter
      changed = true
    }
  }
  if (pre.policy != null) {
    const name = pre.policy.trim()
    if (name && detailPolicyName.value !== name) {
      detailPolicyName.value = name
      changed = true
    }
  }
  if (changed) success(t.value('downsamplePrefillApplied'))
}

async function copyDownsampleShareLink() {
  const path = downsampleFormToPrefill({
    q: policyFilter.value,
    enabled: enabledFilter.value || undefined,
  }, { hash: '#downsample-filters' })
  const url = `${window.location.origin}${path}`
  const res = await copyText(url)
  if (res.ok) success(t.value('downsampleShareCopied'))
  else notifyError(res.error || t.value('failed'))
}

onMounted(() => {
  unregisterDownsampleDirty = registerDirtyChecker('downsample', () => downsampleFormDirty.value)
  window.addEventListener('beforeunload', onDownsampleBeforeUnload)

  void loadData()
  applyDownsamplePrefillFromRoute()
})

watch(
  () => route.fullPath,
  (path, prev) => {
    if (prev != null && path !== prev) applyDownsamplePrefillFromRoute()
  },
)

watch(statusHealthFilter, () => {
  if (!isAdmin.value) return
  void loadData()
})


async function loadData() {
  if (!isAdmin.value || loadDataInFlight.value) return
  loadDataInFlight.value = true
  try {
  loadError.value = ''
  const statusQS = new URLSearchParams()
  if (statusHealthFilter.value) statusQS.set('health', statusHealthFilter.value)
  if (statusHealthFilter.value === 'lagging' && Number(statusMinLag.value) > 0) {
    statusQS.set('min_lag_seconds', String(Number(statusMinLag.value) || 0))
  }
  const statusPath = statusQS.toString()
    ? `/api/v1/admin/downsample/statuses?${statusQS.toString()}`
    : '/api/v1/admin/downsample/statuses'
  const results = await Promise.allSettled([
    apiGet<PoliciesResponse>('/api/v1/admin/downsample/policies'),
    apiGet<StatusesResponse>(statusPath),
  ])
  if (results[0].status === 'fulfilled') {
    applyAdminOpStatus(parseAdminOpStatusPayload(results[0].value))
    policies.value = results[0].value.policies ?? []
    policiesError.value = ''
  } else {
    const msg = formatCaughtError(results[0].reason)
    if (policies.value.length) policiesError.value = msg
    else {
      policies.value = []
      policiesError.value = msg
    }
  }
  if (results[1].status === 'fulfilled') {
    applyAdminOpStatus(parseAdminOpStatusPayload(results[1].value))
    statuses.value = results[1].value.statuses ?? []
    statusesError.value = ''
  } else {
    const msg = formatCaughtError(results[1].reason)
    if (statuses.value.length) statusesError.value = msg
    else {
      statuses.value = []
      statusesError.value = msg
    }
  }
  // 两项皆失败且无任何快照时，使用整页 loadError（兼容旧 e2e）
  if (
    results[0].status === 'rejected'
    && results[1].status === 'rejected'
    && !policies.value.length
    && !statuses.value.length
  ) {
    loadError.value = policiesError.value || statusesError.value || formatCaughtError(results[0].reason)
    policiesError.value = ''
    statusesError.value = ''
  }
  detailListReady.value = true
  await fetchDetailPolicyIfNeeded()
  await fetchDetailStatusIfNeeded()
  maybeNotifyDetailMissing()
  } finally {
    loadDataInFlight.value = false
  }
}

const filteredPolicies = computed(() =>
  filterDownsamplePolicies(policies.value, policyFilter.value, enabledFilter.value),
)

const filteredStatuses = computed(() => {
  const names = new Set(filteredPolicies.value.map((p) => p.name))
  // 策略筛选命中名；无策略时展示全部 statuses
  let list = statuses.value
  if (policies.value.length && (policyFilter.value || enabledFilter.value)) {
    list = list.filter((s) => names.has(s.policy_name))
  }
  return filterDownsampleStatuses(
    list,
    '',
    statusHealthFilter.value,
    Number(statusMinLag.value) || 0,
  )
})

const statusByName = computed(() => {
  const m = new Map<string, DownsampleStatus>()
  for (const s of statuses.value) m.set(s.policy_name, s)
  return m
})

const POLICY_ROW_HEIGHT = 56
const STATUS_ROW_HEIGHT = 44
const DOWNSAMPLE_LIST_HEIGHT = 400

const selectedSet = computed(() => new Set(selectedNames.value))

const allFilteredSelected = computed(
  () =>
    filteredPolicies.value.length > 0 &&
    filteredPolicies.value.every((p) => selectedSet.value.has(p.name)),
)

function toggleSelect(name: string, checked: boolean) {
  const set = new Set(selectedNames.value)
  if (checked) set.add(name)
  else set.delete(name)
  selectedNames.value = [...set]
}

function toggleSelectAllFiltered(checked: boolean) {
  if (!checked) {
    const filtered = new Set(filteredPolicies.value.map((p) => p.name))
    selectedNames.value = selectedNames.value.filter((n) => !filtered.has(n))
    return
  }
  const set = new Set(selectedNames.value)
  for (const p of filteredPolicies.value) set.add(p.name)
  selectedNames.value = [...set]
}

function clearSelection() {
  selectedNames.value = []
}

function openBatch(mode: 'enable' | 'disable') {
  if (batchLoading.value) return
  if (writeBlocked.value) {
    notifyError(t.value(blockedMessageKey('offlineAdminBlocked')))
    return
  }
  if (!selectedNames.value.length) return
  batchMode.value = mode
  batchOpen.value = true
}



function cancelDsAction() {
  dsActionAbort.cancel()
}

function onRangeCancel() {
  if (rangeLoading.value) {
    cancelDsAction()
    return
  }
  rangeOpen.value = false
}

function reportDsCatch(key: DsActionKey, e: unknown) {
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
  reportActionError(key, e)
}

function reportActionError(key: DsActionKey, e: unknown) {
  reportRetryError(key, e)
  const msg = actionResult.value?.message || formatCaughtError(e)
  notifyMaybeAdminBusy(msg, e)
}

async function retryLastDownsampleAction() {
  const key = lastFailedAction.value as DsActionKey | null
  if (!key) return
  if (key === 'create') return createPolicy()
  if (key === 'delete' && deleteName.value) {
    deleteOpen.value = true
    return confirmDelete()
  }
  if (key === 'toggle' && lastToggleName.value) {
    const pol = policies.value.find((x) => x.name === lastToggleName.value)
    if (pol) return togglePolicy(pol)
  }
  if (key === 'batch') {
    batchOpen.value = true
    return confirmBatch()
  }
  if (key === 'range' && rangeName.value) {
    rangeOpen.value = true
    return confirmRange()
  }
  if (key === 'run' && lastRunName.value) return runPolicy(lastRunName.value)
  if (key === 'reset' && lastResetName.value) return resetPolicy(lastResetName.value)
}

async function confirmBatch() {
  if (batchLoading.value) return
  if (writeBlocked.value) {
    const msg = t.value(blockedMessageKey('offlineAdminBlocked'))
    setActionError(msg)
    notifyError(msg)
    return
  }
  const names = [...selectedNames.value]
  if (!names.length) {
    batchOpen.value = false
    return
  }
  batchLoading.value = true
  batchProgress.value = { ...emptyBatchProgress(), total: names.length }
  dsActionStartedAt.value = Date.now()
  const signal = dsActionAbort.begin()
  clearActionResult()
  const action = batchMode.value === 'enable' ? 'enable' : 'disable'
  let data: BatchMutationSummary | null = null
  const cancelledBox: { summary: BatchMutationSummary | null } = { summary: null }
  try {
    await apiPostNDJSONStream(
      '/api/v1/admin/downsample/policies/batch?stream=1',
      { names, action },
      (_line, record, parseError) => {
        if (parseError || record == null) return
        const applied = applyBatchProgressEvent(batchProgress.value, record)
        batchProgress.value = applied.next
        if (applied.summary) {
          data = applied.summary
          if (applied.summary.cancelled) cancelledBox.summary = applied.summary
        }
        if (applied.error) throw new Error(applied.error)
      },
      { signal, headers: { Accept: 'application/x-ndjson' } },
    )
    const cancelledSummary = cancelledBox.summary
    if (cancelledSummary) {
      applyAdminOpStatus(parseAdminOpStatusPayload(cancelledSummary as unknown as { admin_op_busy?: unknown; op?: unknown; started_at_unix?: unknown; last?: unknown }))
      const processed = cancelledSummary.ok_count + cancelledSummary.skip_count + cancelledSummary.fail_count
      const msg = formatMessage(t.value('listBatchCancelledPartial'), {
        done: processed || batchProgress.value.done,
        total: batchProgress.value.total || processed,
        ok: cancelledSummary.ok_count,
        skip: cancelledSummary.skip_count,
        fail: cancelledSummary.fail_count,
      })
      setActionResult(makeActionResult('info', msg))
      info(msg)
      try { await loadData() } catch { /* soft */ }
      selectedNames.value = []

      batchOpen.value = false
      return
    }
    if (!data) {
      data = {
        ok: batchProgress.value.fail === 0,
        ok_count: batchProgress.value.ok,
        skip_count: batchProgress.value.skip,
        fail_count: batchProgress.value.fail,
        items: [],
      }
    }
    applyAdminOpStatus(parseAdminOpStatusPayload(data as unknown as { admin_op_busy?: unknown; op?: unknown; started_at_unix?: unknown; last?: unknown }))
    await loadData()
    selectedNames.value = []
    batchOpen.value = false
    const ok = data.ok_count ?? 0
    const fail = data.fail_count ?? 0
    const errors = (data.items ?? [])
      .filter((it) => it.status === 'error')
      .map((it) => `${it.name}: ${it.message || it.status}`)
    if (errors.length === 0) {
      const msg = batchMode.value === 'enable'
        ? `${t.value('downsampleBatchEnable')}: ${ok}`
        : `${t.value('downsampleBatchDisable')}: ${ok}`
      setActionOk(msg)
      success(msg)
    } else {
      const msg = `${ok}/${names.length}; ${errors.slice(0, 3).join('; ')}`
      if (ok === 0) {
        lastFailedAction.value = 'batch'
        actionResult.value = makeActionResult('error', msg)
        notifyError(msg)
      } else {
        setActionResult(makeActionResult('info', msg))
        success(msg)
      }
    }
  } catch (e) {
    if (isCanceledError(e) && batchProgress.value.done > 0) {
      const prog = batchProgress.value
      const msg = formatMessage(t.value('listBatchCancelledPartial'), {
        done: prog.done,
        total: prog.total || names.length,
        ok: prog.ok,
        skip: prog.skip,
        fail: prog.fail,
      })
      setActionResult(makeActionResult('info', msg))
      info(msg)
      try { await loadData() } catch { /* soft */ }
      selectedNames.value = []
      batchOpen.value = false
    } else {
      reportDsCatch('batch', e)
    }
  } finally {
    dsActionAbort.end()
    batchLoading.value = false
    dsActionStartedAt.value = null
    batchProgress.value = emptyBatchProgress()
  }
}

const newPolicyTagsText = computed({
  get: () => newPolicy.value.group_by_tags.join(', '),
  set: (v: string) => {
    newPolicy.value.group_by_tags = v.split(',').map((x) => x.trim()).filter((x) => x.length > 0)
  },
})


const downsampleFormDirty = computed(() => {
  if (!showCreate.value) return false
  return isDownsampleCreateDraftDirty({
    name: newPolicy.value.name,
    source_database: newPolicy.value.source_database,
    source_measurement: newPolicy.value.source_measurement,
    target_database: newPolicy.value.target_database,
    target_measurement: newPolicy.value.target_measurement,
    interval_human: intervalHuman.value,
    group_by_tags: newPolicy.value.group_by_tags.join(','),
    enabled: newPolicy.value.enabled,
    functions_json: JSON.stringify(newPolicy.value.functions ?? []),
  })
})

function onDownsampleBeforeUnload(e: BeforeUnloadEvent) {
  if (!downsampleFormDirty.value) return
  e.preventDefault()
  e.returnValue = ''
}

let unregisterDownsampleDirty: (() => void) | null = null

async function createPolicy() {
  if (writeBlocked.value) {
    const msg = t.value(blockedMessageKey('offlineAdminBlocked'))
    setActionError(msg)
    notifyError(msg)
    return
  }
  if (!newPolicy.value.name.trim()) return
  clearActionResult()
  try {
    newPolicy.value.interval = parseHumanDurationToNs(intervalHuman.value)
    newPolicy.value.refresh_interval = parseHumanDurationToNs(refreshHuman.value || intervalHuman.value)
    newPolicy.value.lookback = parseHumanDurationToNs(lookbackHuman.value || intervalHuman.value)
  } catch (e) {
    const msg = formatCaughtError(e)
    setActionError(msg)
    notifyError(msg)
    return
  }
  if (!newPolicy.value.source_database || !newPolicy.value.source_measurement) {
    const msg = t.value('downsampleNeedSource')
    setActionError(msg)
    notifyError(msg)
    return
  }
  // POC 默认补齐 retention / 运行参数，避免 source incomplete
  if (!newPolicy.value.source_retention) newPolicy.value.source_retention = 'autogen'
  if (!newPolicy.value.target_database) newPolicy.value.target_database = newPolicy.value.source_database
  if (!newPolicy.value.target_retention) newPolicy.value.target_retention = newPolicy.value.source_retention || 'autogen'
  if (!newPolicy.value.target_measurement) {
    newPolicy.value.target_measurement = `${newPolicy.value.source_measurement}_ds`
  }
  const intervalNs = newPolicy.value.interval
  if (!newPolicy.value.refresh_interval || newPolicy.value.refresh_interval <= 0) {
    newPolicy.value.refresh_interval = intervalNs
  }
  if (newPolicy.value.lookback == null || newPolicy.value.lookback < 0) {
    newPolicy.value.lookback = intervalNs
  }
  if (!newPolicy.value.batch_size || newPolicy.value.batch_size <= 0) {
    newPolicy.value.batch_size = 100
  }
  createLoading.value = true
  dsActionStartedAt.value = Date.now()
  const signal = dsActionAbort.begin()
  try {
    const created = await apiPost<{ admin_op_busy?: boolean; op?: string; started_at_unix?: number; last?: unknown }>('/api/v1/admin/downsample/policies', newPolicy.value, { signal })
    applyAdminOpStatus(parseAdminOpStatusPayload(created))
    showCreate.value = false
    newPolicy.value = {
      name: '',
      source_database: '',
      source_measurement: '',
      source_retention: 'autogen',
      target_database: '',
      target_measurement: '',
      target_retention: 'autogen',
      interval: 60_000_000_000,
      refresh_interval: 60_000_000_000,
      lookback: 60_000_000_000,
      batch_size: 100,
      functions: [{ function: 'mean', field: 'value', as: 'mean_value' }],
      group_by_tags: [],
      enabled: true,
    }
    intervalHuman.value = '1m'
    refreshHuman.value = '1m'
    lookbackHuman.value = '1m'
    await loadData()
    setActionOk(t.value('downsampleCreated'))
    success(t.value('downsampleCreated'))
  } catch (e) {
    reportDsCatch('create', e)
  } finally {
    dsActionAbort.end()
    createLoading.value = false
    dsActionStartedAt.value = null
  }
}

function requestDelete(name: string) {
  if (writeBlocked.value) {
    notifyError(t.value(blockedMessageKey('offlineAdminBlocked')))
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
  deleteLoading.value = true
  dsActionStartedAt.value = Date.now()
  const signal = dsActionAbort.begin()
  try {
    const del = await apiDelete<{ admin_op_busy?: boolean; op?: string; started_at_unix?: number; last?: unknown }>(`/api/v1/admin/downsample/policies/${encodeURIComponent(deleteName.value)}`, { signal })
    applyAdminOpStatus(parseAdminOpStatusPayload(del))
    deleteOpen.value = false
    await loadData()
    setActionOk(t.value('downsampleDeleted'))
    success(t.value('downsampleDeleted'))
  } catch (e) {
    reportDsCatch('delete', e)
  } finally {
    dsActionAbort.end()
    deleteLoading.value = false
    dsActionStartedAt.value = null
  }
}

async function togglePolicy(policy: DownsamplePolicy) {
  if (writeBlocked.value) {
    const msg = t.value(blockedMessageKey('offlineAdminBlocked'))
    setActionError(msg)
    notifyError(msg)
    return
  }
  const action = policy.enabled ? 'disable' : 'enable'
  lastToggleName.value = policy.name
  lastToggleWantEnabled.value = !policy.enabled
  dsToggleLoading.value = true
  dsActionStartedAt.value = Date.now()
  const signal = dsActionAbort.begin()
  try {
    const act = await apiPost<{ admin_op_busy?: boolean; op?: string; started_at_unix?: number; last?: unknown }>(`/api/v1/admin/downsample/policies/${encodeURIComponent(policy.name)}/${action}`, undefined, { signal })
    applyAdminOpStatus(parseAdminOpStatusPayload(act))
    await loadData()
    lastFailedAction.value = null
    const msg = policy.enabled ? t.value('downsampleDisabledOk') : t.value('downsampleEnabledOk')
    setActionOk(msg)
    success(msg)
  } catch (e) {
    reportDsCatch('toggle', e)
  } finally {
    dsActionAbort.end()
    dsToggleLoading.value = false
    dsActionStartedAt.value = null
  }
}

function addPolicyFunction() {
  newPolicy.value.functions.push({ function: 'mean', field: '', as: '' })
}
function removePolicyFunction(idx: number) {
  newPolicy.value.functions.splice(idx, 1)
}
function getStatus(name: string) {
  return statusByName.value.get(name)
}
function formatUnix(v: number) {
  if (!v) return '-'
  return new Date(v > 1e12 ? v / 1e6 : v * 1000).toLocaleString()
}
function formatDuration(ns: number) {
  try { return formatNsDuration(ns) } catch { return String(ns) }
}

async function runPolicy(name: string) {
  if (writeBlocked.value) {
    const msg = t.value(blockedMessageKey('offlineAdminBlocked'))
    setActionError(msg)
    notifyError(msg)
    return
  }
  lastRunName.value = name
  dsRunLoading.value = true
  dsActionStartedAt.value = Date.now()
  const signal = dsActionAbort.begin()
  clearActionResult()
  try {
    const data = await apiPost<{ result: DownsampleRunResult; admin_op_busy?: boolean; op?: string; started_at_unix?: number; last?: unknown }>(
      `/api/v1/admin/downsample/policies/${encodeURIComponent(name)}/run`,
      {},
      { signal },
    )
    applyAdminOpStatus(parseAdminOpStatusPayload(data))
    const msg = formatRunResultMessage('run', name, data.result)
    setActionOk(msg)
    await loadData()
    success(msg)
  } catch (e) {
    reportDsCatch('run', e)
  } finally {
    dsActionAbort.end()
    dsRunLoading.value = false
    dsActionStartedAt.value = null
  }
}

async function resetPolicy(name: string) {
  if (writeBlocked.value) {
    const msg = t.value(blockedMessageKey('offlineAdminBlocked'))
    setActionError(msg)
    notifyError(msg)
    return
  }
  lastResetName.value = name
  dsRunLoading.value = true
  dsActionStartedAt.value = Date.now()
  const signal = dsActionAbort.begin()
  clearActionResult()
  try {
    const reset = await apiPost<{ admin_op_busy?: boolean; op?: string; started_at_unix?: number; last?: unknown }>(`/api/v1/admin/downsample/policies/${encodeURIComponent(name)}/reset`, {
      reset: { allow_policy_replace: true },
    }, { signal })
    applyAdminOpStatus(parseAdminOpStatusPayload(reset))
    await loadData()
    const msg = `${t.value('downsampleResetOk')}: ${name}`
    setActionOk(msg)
    success(msg)
  } catch (e) {
    reportDsCatch('reset', e)
  } finally {
    dsActionAbort.end()
    dsRunLoading.value = false
    dsActionStartedAt.value = null
  }
}

const detailPolicy = computed(() => {
  const name = detailPolicyName.value.trim()
  if (!name) return null
  return policies.value.find((x) => x.name === name) || null
})
const detailPolicyMissing = computed(() => {
  const name = detailPolicyName.value.trim()
  if (!name || !detailListReady.value) return false
  // 列表已拉取后仍找不到时视为缺失（含空列表深链）
  return !detailPolicy.value
})
const detailFields = computed(() =>
  buildDownsamplePolicyDetailFields(
    detailPolicy.value || undefined,
    (ns) => formatDuration(ns || 0),
    t.value('emptyValue'),
  ).map((f) => ({ ...f, label: t.value(f.labelKey as MessageKey) })),
)
const detailFunctions = computed(() => listDownsampleFunctions(detailPolicy.value?.functions))
const detailStatus = computed(() => {
  const name = detailPolicyName.value.trim()
  if (!name) return null
  return getStatus(name) || null
})



async function fetchDetailStatusIfNeeded() {
  const name = detailPolicyName.value.trim()
  if (!name) return
  if (getStatus(name)) return
  try {
    const data = await apiGet<{ status?: DownsampleStatus }>(
      `/api/v1/admin/downsample/policies/${encodeURIComponent(name)}/status`,
    )
    applyAdminOpStatus(parseAdminOpStatusPayload(data as { admin_op_busy?: unknown; op?: unknown; started_at_unix?: unknown; last?: unknown }))
    const st = data?.status
    if (st?.policy_name) {
      const idx = statuses.value.findIndex((s) => s.policy_name === st.policy_name)
      if (idx >= 0) {
        const next = statuses.value.slice()
        next[idx] = st
        statuses.value = next
      } else {
        statuses.value = [...statuses.value, st]
      }
    }
  } catch {
    // ignore
  }
}

async function fetchDetailPolicyIfNeeded() {
  const name = detailPolicyName.value.trim()
  if (!name || detailPolicy.value) return
  // 列表未含深链策略时，直读单策略 GET 对齐 server（失败则走 missing 提示）
  try {
    const data = await apiGet<{ policy?: typeof policies.value[number] }>(
      `/api/v1/admin/downsample/policies/${encodeURIComponent(name)}`,
    )
    applyAdminOpStatus(parseAdminOpStatusPayload(data as { admin_op_busy?: unknown; op?: unknown; started_at_unix?: unknown; last?: unknown }))
    const pol = data?.policy
    if (pol?.name) {
      const exists = policies.value.some((p) => p.name === pol.name)
      if (!exists) policies.value = [...policies.value, pol]
    }
  } catch {
    // ignore; maybeNotifyDetailMissing handles UX
  }
}

function maybeNotifyDetailMissing() {
  const name = detailPolicyName.value.trim()
  if (!name || !detailPolicyMissing.value) {
    if (!name) detailMissingNotified.value = ''
    return
  }
  if (detailMissingNotified.value === name) return
  detailMissingNotified.value = name
  notifyError(t.value('downsampleDetailMissing'))
}

function openPolicyDetail(name: string) {
  detailPolicyName.value = name
  detailMissingNotified.value = ''
  syncPolicyDetailQuery(name)
}
function closePolicyDetail() {
  detailPolicyName.value = ''
  detailMissingNotified.value = ''
  syncPolicyDetailQuery('')
}
function syncPolicyDetailQuery(name: string) {
  const q = { ...(route.query as Record<string, string | string[] | null | undefined>) }
  const next = name.trim()
  if (next) q.policy = next
  else delete q.policy
  const cur = typeof route.query.policy === 'string' ? route.query.policy : ''
  if ((cur || '') === next) return
  void router.replace({ path: '/downsample', query: q, hash: next ? '#downsample-detail' : route.hash || undefined })
}
async function copyPolicyDetailJSON() {
  const text = downsamplePolicyDetailToJSONText(detailPolicy.value || undefined)
  if (!text || text.includes('"policy": null')) {
    notifyError(t.value('opsStatusLastEmpty'))
    return
  }
  const res = await copyText(text)
  if (res.ok) success(t.value('downsampleDetailCopied'))
  else notifyError(res.error || t.value('failed'))
}
async function copyPolicyDetailMarkdown() {
  if (!detailPolicy.value) {
    notifyError(t.value('opsStatusLastEmpty'))
    return
  }
  const md = downsamplePolicyDetailToMarkdownText(
    detailPolicy.value,
    (ns) => formatDuration(ns || 0),
    t.value('emptyValue'),
  )
  if (!md.trim()) {
    notifyError(t.value('opsStatusLastEmpty'))
    return
  }
  const res = await copyText(md)
  if (res.ok) success(t.value('downsampleDetailMarkdownCopied'))
  else notifyError(res.error || t.value('failed'))
}
async function copyPolicyDetailLink() {
  const name = detailPolicyName.value.trim()
  if (!name) {
    notifyError(t.value('opsStatusLastEmpty'))
    return
  }
  const path = downsampleFormToPrefill({
    q: policyFilter.value,
    enabled: enabledFilter.value || undefined,
    policy: name,
  }, { hash: '#downsample-detail' })
  const url = `${window.location.origin}${path}`
  const res = await copyText(url)
  if (res.ok) success(t.value('downsampleDetailLinkCopied'))
  else notifyError(res.error || t.value('failed'))
}
function detailEnabledLabel(enabled: boolean | undefined): string {
  return enabled ? t.value('downsampleEnabledOnly') : t.value('downsampleDisabledOnly')
}

function openRange(mode: DownsampleRangeMode, name: string) {
  if (writeBlocked.value) {
    notifyError(t.value(blockedMessageKey('offlineAdminBlocked')))
    return
  }
  rangeMode.value = mode
  rangeName.value = name
  const st = getStatus(name)
  const sug = suggestDownsampleRange(st?.completed_until_unix)
  rangeStartUnix.value = sug.startUnix
  rangeEndUnix.value = sug.endUnix
  rangeAdvanceWatermark.value = mode === 'run-range'
  rangeOpen.value = true
}

const rangeTitle = computed(() => {
  if (rangeMode.value === 'repair') return t.value('downsampleRepairTitle')
  if (rangeMode.value === 'run-range') return t.value('downsampleRunRangeTitle')
  return t.value('downsampleDryRunTitle')
})

const rangeHint = computed(() => {
  if (rangeMode.value === 'repair') return t.value('downsampleRepairMsg')
  if (rangeMode.value === 'run-range') return t.value('downsampleRunRangeMsg')
  return t.value('downsampleDryRunMsg')
})

const rangeConfirmLabel = computed(() => {
  if (rangeMode.value === 'repair') return t.value('downsampleRepair')
  if (rangeMode.value === 'run-range') return t.value('downsampleRunRange')
  return t.value('downsampleDryRun')
})

async function confirmRange() {
  if (writeBlocked.value) {
    const msg = t.value(blockedMessageKey('offlineAdminBlocked'))
    setActionError(msg)
    notifyError(msg)
    return
  }
  if (!rangeName.value) return
  const loc = locale.value === 'en' ? 'en' : 'zh'
  const check = validateDownsampleRange({
    startUnix: rangeStartUnix.value,
    endUnix: rangeEndUnix.value,
    advanceWatermark: rangeAdvanceWatermark.value,
  })
  if (!check.ok) {
    const msg = rangeErrorMessage(check.error, loc)
    setActionError(msg)
    notifyError(msg)
    return
  }
  rangeLoading.value = true
  dsActionStartedAt.value = Date.now()
  const signal = dsActionAbort.begin()
  clearActionResult()
  try {
    const body = buildDownsampleRangeBody({
      startUnix: rangeStartUnix.value,
      endUnix: rangeEndUnix.value,
      advanceWatermark: rangeMode.value === 'run-range' ? rangeAdvanceWatermark.value : false,
    })
    const path = rangeActionPath(rangeName.value, rangeMode.value)
    if (rangeMode.value === 'dry-run') {
      const data = await apiPost<{ result: DownsampleDryRunResult; admin_op_busy?: boolean; op?: string; started_at_unix?: number; last?: unknown }>(path, body, { signal })
      applyAdminOpStatus(parseAdminOpStatusPayload(data))
      const msg = formatRunResultMessage('dry-run', rangeName.value, data.result)
      rangeOpen.value = false
      setActionResult(makeActionResult('info', msg))
      success(msg)
    } else {
      const data = await apiPost<{ result: DownsampleRunResult; admin_op_busy?: boolean; op?: string; started_at_unix?: number; last?: unknown }>(path, body, { signal })
      applyAdminOpStatus(parseAdminOpStatusPayload(data))
      const msg = formatRunResultMessage(rangeMode.value, rangeName.value, data.result)
      rangeOpen.value = false
      await loadData()
      lastFailedAction.value = null
      setActionOk(msg)
      success(msg)
    }
  } catch (e) {
    reportDsCatch('range', e)
  } finally {
    dsActionAbort.end()
    rangeLoading.value = false
    dsActionStartedAt.value = null
  }
}

async function exportJSON() {
  if (!filteredPolicies.value.length) {
    warn(t.value('inventoryExportEmpty'))
    return
  }
  if (exportBusy.value) return
  const list = filteredPolicies.value.slice()
  const outcome = await runJSONExport({
    label: 'JSON',
    filename: stampFilename('mts-downsample-policies', 'json'),
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
      return buildDownsampleExport(list)
    },
  })
  if (outcome === 'done') success(t.value('inventoryExported'))
  else if (outcome === 'cancelled') info(t.value('exportCancelledToast'))
  else if (outcome === 'error') notifyError(exportJob.value.error || t.value('failed'))
}

async function exportCSV() {
  if (!filteredPolicies.value.length) {
    warn(t.value('inventoryExportEmpty'))
    return
  }
  if (exportBusy.value) return
  const list = filteredPolicies.value.slice()
  const outcome = await runTextExport({
    label: 'CSV',
    filename: stampFilename('mts-downsample-policies', 'csv'),
    mime: 'text/csv;charset=utf-8',
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
      return downsampleToCSV(list)
    },
  })
  if (outcome === 'done') success(t.value('inventoryExported'))
  else if (outcome === 'cancelled') info(t.value('exportCancelledToast'))
  else if (outcome === 'error') notifyError(exportJob.value.error || t.value('failed'))
}

onBeforeUnmount(() => {
  cancelDsAction()
  unregisterDownsampleDirty?.()
  unregisterDownsampleDirty = null
  window.removeEventListener('beforeunload', onDownsampleBeforeUnload)
})
</script>

<template>
  <PermissionDenied v-if="!isAdmin" />
  <div v-else class="space-y-6" data-testid="downsample-page">
    <div class="flex flex-wrap items-center justify-between gap-3">
      <div>
        <h1 class="text-lg font-semibold text-slate-800 dark:text-slate-100">{{ t('downsampleTitle') }}</h1>
        <div class="mt-1 flex flex-wrap items-center gap-2">
          <p class="text-xs text-slate-500 dark:text-slate-400">{{ t('downsampleDesc') }}</p>
          <AdminOpLastChip
            v-if="downsampleAdminLastLabel"
            :label="downsampleAdminLastLabel"
            :last-ok="adminOpBusySummary?.lastOk"
            :last-error="downsampleAdminLastErrorDetail"
            test-id="downsample-admin-last"
            show-copy
            copy-test-id="downsample-admin-last-copy"
            error-test-id="downsample-admin-last-error"
          />

        </div>
      </div>
      <div class="flex gap-2">
        <button class="inline-flex items-center gap-1 rounded-lg border border-slate-200 bg-white px-3 py-1.5 text-xs dark:border-slate-700 dark:bg-slate-900" @click="loadData">
          <RefreshCw class="h-3.5 w-3.5" /> {{ t('refresh') }}
        </button>
        <button
          type="button"
          class="inline-flex items-center gap-1 rounded-lg bg-slate-800 px-3 py-1.5 text-xs font-medium text-white disabled:cursor-not-allowed disabled:opacity-50"
          data-testid="downsample-open-create"
          :disabled="writeBlocked"
          :title="writeBlocked ? t(blockedMessageKey('offlineAdminBlocked')) : undefined"
          @click="showCreate = true"
        >
          <Plus class="h-3.5 w-3.5" /> {{ t('downsampleCreate') }}
        </button>
      </div>
    </div>

    <ActionResultBanner v-if="loadError" kind="error" :message="loadError" retryable data-testid="downsample-load-error" @retry="loadData" @dismiss="loadError = ''" />
    <InFlightBanner
      :active="deleteLoading || batchLoading || rangeLoading || createLoading || dsToggleLoading || dsRunLoading"
      :started-at-ms="dsActionStartedAt"
      kind="admin"
      :progress-percent="batchLoading ? batchProgressPercent(batchProgress) : null"
      :progress-label="batchLoading && batchProgress.total ? formatMessage(t('batchProgressLabel'), { done: batchProgress.done, total: batchProgress.total, ok: batchProgress.ok, skip: batchProgress.skip, fail: batchProgress.fail }) : undefined"
      @cancel="cancelDsAction"
    />
    <PartialErrorBanner
      v-if="!loadError && policiesError"
      :message="`${t('downsamplePoliciesLoadFailed')}：${policiesError}`"
      test-id="downsample-policies-error"
      @retry="loadData"
      @dismiss="policiesError = ''"
    />
    <PartialErrorBanner
      v-if="!loadError && statusesError"
      :message="`${t('downsampleStatusesLoadFailed')}：${statusesError}`"
      test-id="downsample-statuses-error"
      @retry="loadData"
      @dismiss="statusesError = ''"
    />
    <ActionResultBanner
      :result="actionResult"
      :retryable="canRetryAction"
      :action-label="dsAdminBusyAction?.label || ''"
      :action-path="dsAdminBusyAction?.path || ''"
      data-testid="downsample-action-result"
      @retry="retryLastDownsampleAction"
      @dismiss="clearActionResult"
    />

    <div id="downsample-filters" class="scroll-mt-20 flex flex-wrap items-end gap-3" data-testid="downsample-filter-bar">
      <label class="text-xs mts-muted">{{ t('filter') }}
        <input
          v-model="policyFilter"
          type="search"
          class="mts-input mt-1 min-w-[14rem]"
          data-testid="downsample-filter"
          :placeholder="t('downsampleFilterPlaceholder')"
        />
      </label>
      <label class="text-xs mts-muted">{{ t('downsampleStatusFilter') }}
        <select v-model="enabledFilter" class="mts-input mt-1" data-testid="downsample-enabled-filter">
          <option value="">{{ t('downsampleEnabledAll') }}</option>
          <option value="enabled">{{ t('downsampleEnabledOnly') }}</option>
          <option value="disabled">{{ t('downsampleDisabledOnly') }}</option>
        </select>
      </label>
      <span class="text-xs mts-muted" data-testid="downsample-filter-count">{{ filteredPolicies.length }} / {{ policies.length }}</span>
      <ListSelectionToolbar
        prefix="downsample"
        :selected-count="selectedNames.length"
        :has-visible="!!filteredPolicies.length"
        clear-test-id="downsample-clear-select"
        @select-all="toggleSelectAllFiltered(true)"
        @clear="clearSelection"
      >
        <template #actions>
          <button type="button" class="mts-btn" data-testid="downsample-batch-enable" :disabled="!selectedNames.length || writeBlocked || batchLoading" :title="writeBlocked ? t(blockedMessageKey('offlineAdminBlocked')) : undefined" @click="openBatch('enable')">{{ t('downsampleBatchEnable') }}</button>
          <button type="button" class="mts-btn" data-testid="downsample-batch-disable" :disabled="!selectedNames.length || writeBlocked || batchLoading" :title="writeBlocked ? t(blockedMessageKey('offlineAdminBlocked')) : undefined" @click="openBatch('disable')">{{ t('downsampleBatchDisable') }}</button>
          <ExportJobBanner class="w-full basis-full" :job="exportJob" :retryable="canRetryExport" @cancel="cancelExport" @retry="retryLastExport" @dismiss="resetExport" />
          <button type="button" class="mts-btn" data-testid="downsample-export-json" :disabled="exportBusy || !filteredPolicies.length" @click="exportJSON">
            <Download class="h-3.5 w-3.5" /> {{ t('inventoryExportJSON') }}
          </button>
          <button type="button" class="mts-btn" data-testid="downsample-export-csv" :disabled="exportBusy || !filteredPolicies.length" @click="exportCSV">
            <Download class="h-3.5 w-3.5" /> {{ t('inventoryExportCSV') }}
          </button>
          <button type="button" class="mts-btn" data-testid="downsample-share-link" @click="copyDownsampleShareLink">
            {{ t('downsampleShareLink') }}
          </button>
        </template>
      </ListSelectionToolbar>
    </div>

    <div v-if="!policies.length" class="mts-card">
      <EmptyState data-testid="downsample-empty" :title="t('downsampleEmpty')" :description="t('downsampleEmptyDesc')">
        <template #action>
          <button type="button" class="mts-btn-primary" data-testid="downsample-empty-create" :disabled="writeBlocked" :title="writeBlocked ? t(blockedMessageKey('offlineAdminBlocked')) : undefined" @click="showCreate = true">{{ t('downsampleCreate') }}</button>
        <span v-if="downsampleFormDirty" data-testid="downsample-dirty-badge" class="rounded-full bg-amber-100 px-2 py-0.5 text-[11px] font-medium text-amber-800 dark:bg-amber-950/50 dark:text-amber-200">{{ t('adminDirtyBadge') }}</span>
        </template>
      </EmptyState>
    </div>

    <div v-else-if="!filteredPolicies.length" class="mts-card">
      <EmptyState data-testid="downsample-empty-filter" :title="t('downsampleFilterEmpty')" :description="t('downsampleFilterEmptyDesc')">
        <template #action>
          <button type="button" class="mts-btn-primary" data-testid="downsample-clear-filters" @click="policyFilter = ''; enabledFilter = ''">{{ t('clearFilters') }}</button>
        </template>
      </EmptyState>
    </div>

    <div v-else class="overflow-hidden rounded-xl border border-slate-200 bg-white dark:border-slate-700 dark:bg-slate-900" data-testid="downsample-policies-table">
      <div
        class="grid grid-cols-[2.5rem_minmax(7rem,0.9fr)_minmax(10rem,1.4fr)_minmax(5rem,0.55fr)_minmax(5rem,0.55fr)_minmax(5rem,0.55fr)_minmax(7rem,0.85fr)_minmax(11rem,1.15fr)] border-b border-slate-100 bg-slate-50/50 text-left text-xs uppercase text-slate-500 dark:border-slate-800 dark:bg-slate-900/40 dark:text-slate-400"
        data-testid="downsample-policies-header"
      >
        <div class="px-3 py-2.5">
          <input
            type="checkbox"
            data-testid="downsample-select-all-box"
            :checked="allFilteredSelected"
            @change="toggleSelectAllFiltered(($event.target as HTMLInputElement).checked)"
          />
        </div>
        <div class="px-3 py-2.5">{{ t('downsampleColName') }}</div>
        <div class="px-3 py-2.5">{{ t('downsampleColPath') }}</div>
        <div class="px-3 py-2.5" :title="t('downsampleColIntervalHint')">{{ t('downsampleColInterval') }}</div>
        <div class="px-3 py-2.5">{{ t('downsampleColRefresh') }}</div>
        <div class="px-3 py-2.5">{{ t('downsampleStatusFilter') }}</div>
        <div class="px-3 py-2.5">{{ t('downsampleColCompleted') }}</div>
        <div class="px-3 py-2.5">{{ t('action') }}</div>
      </div>
      <VirtualTable
        :items="filteredPolicies"
        :row-height="POLICY_ROW_HEIGHT"
        :height="DOWNSAMPLE_LIST_HEIGHT"
        data-testid="downsample-virtual-list"
      >
        <template #default="{ item: policy }">
          <div
            class="grid h-full grid-cols-[2.5rem_minmax(7rem,0.9fr)_minmax(10rem,1.4fr)_minmax(5rem,0.55fr)_minmax(5rem,0.55fr)_minmax(5rem,0.55fr)_minmax(7rem,0.85fr)_minmax(11rem,1.15fr)] items-center border-b border-slate-50 dark:border-slate-800"
            :data-testid="`downsample-row-${policy.name}`"
          >
            <div class="px-3">
              <input
                type="checkbox"
                class="h-3.5 w-3.5"
                :data-testid="`downsample-select-${policy.name}`"
                :checked="selectedSet.has(policy.name)"
                :aria-label="t('listSelectCol') + ' ' + policy.name"
                @change="toggleSelect(policy.name, ($event.target as HTMLInputElement).checked)"
              />
            </div>
            <button
              type="button"
              class="truncate px-3 text-left font-medium text-sky-700 hover:underline dark:text-sky-300"
              :title="policy.name"
              :data-testid="`downsample-open-detail-${policy.name}`"
              @click="openPolicyDetail(policy.name)"
            >{{ policy.name }}</button>
            <div
              class="truncate px-3 text-xs text-slate-600 dark:text-slate-300"
              :title="`${policy.source_database}/${policy.source_retention || 'autogen'}/${policy.source_measurement} → ${policy.target_database}/${policy.target_retention || 'autogen'}/${policy.target_measurement}`"
              :data-testid="`downsample-path-${policy.name}`"
            >
              {{ policy.source_database }}/{{ policy.source_measurement }} → {{ policy.target_database }}/{{ policy.target_measurement }}
            </div>
            <div
              class="px-3 text-slate-600 dark:text-slate-300"
              :title="formatDuration(policy.interval)"
              :data-testid="`downsample-interval-${policy.name}`"
            >{{ formatDuration(policy.interval) }}</div>
            <div
              class="truncate px-3 text-xs text-slate-600 dark:text-slate-300"
              :title="policy.refresh_interval ? formatDuration(policy.refresh_interval) : t('emptyValue')"
              :data-testid="`downsample-refresh-${policy.name}`"
            >{{ policy.refresh_interval ? formatDuration(policy.refresh_interval) : t('emptyValue') }}</div>
            <div class="px-3">
              <span
                class="rounded px-2 py-0.5 text-xs font-medium"
                :class="policy.enabled ? 'bg-green-100 text-green-700 dark:text-green-200' : 'bg-slate-100 text-slate-500 dark:bg-slate-800 dark:text-slate-400'"
              >{{ policy.enabled ? t('downsampleEnabledOnly') : t('downsampleDisabledOnly') }}</span>
            </div>
            <div class="truncate px-3 text-xs text-slate-500" :title="getStatus(policy.name) ? formatUnix(getStatus(policy.name)!.completed_until_unix) : ''">
              {{ getStatus(policy.name) ? formatUnix(getStatus(policy.name)!.completed_until_unix) : t('emptyValue') }}
            </div>
            <div class="px-2">
              <div class="flex flex-wrap items-center gap-0.5">
                <button type="button" class="rounded p-1 text-slate-400 hover:text-slate-600 disabled:cursor-not-allowed disabled:opacity-40" :disabled="writeBlocked || dsRunLoading" :title="writeBlocked ? t(blockedMessageKey('offlineAdminBlocked')) : t('downsampleRunTitle')" :data-testid="`downsample-run-${policy.name}`" @click="runPolicy(policy.name)">
                  <PlayCircle class="h-4 w-4" />
                </button>
                <button type="button" class="rounded p-1 text-slate-400 hover:text-slate-600 disabled:cursor-not-allowed disabled:opacity-40" :disabled="writeBlocked" :title="writeBlocked ? t(blockedMessageKey('offlineAdminBlocked')) : t('downsampleDryRun')" :data-testid="`downsample-dryrun-${policy.name}`" @click="openRange('dry-run', policy.name)">
                  <FlaskConical class="h-4 w-4" />
                </button>
                <button type="button" class="rounded p-1 text-slate-400 hover:text-slate-600 disabled:cursor-not-allowed disabled:opacity-40" :disabled="writeBlocked" :title="writeBlocked ? t(blockedMessageKey('offlineAdminBlocked')) : t('downsampleRunRange')" :data-testid="`downsample-runrange-${policy.name}`" @click="openRange('run-range', policy.name)">
                  <Timer class="h-4 w-4" />
                </button>
                <button type="button" class="rounded p-1 text-slate-400 hover:text-slate-600 disabled:cursor-not-allowed disabled:opacity-40" :disabled="writeBlocked" :title="writeBlocked ? t(blockedMessageKey('offlineAdminBlocked')) : t('downsampleRepair')" :data-testid="`downsample-repair-${policy.name}`" @click="openRange('repair', policy.name)">
                  <Wrench class="h-4 w-4" />
                </button>
                <button type="button" class="rounded p-1 text-slate-400 hover:text-slate-600 disabled:cursor-not-allowed disabled:opacity-40" :disabled="writeBlocked || dsRunLoading" :title="writeBlocked ? t(blockedMessageKey('offlineAdminBlocked')) : t('downsampleResetTitle')" :data-testid="`downsample-reset-${policy.name}`" @click="resetPolicy(policy.name)">
                  <RotateCcw class="h-4 w-4" />
                </button>
                <button type="button" class="rounded p-1 text-slate-400 hover:text-slate-600 disabled:cursor-not-allowed disabled:opacity-40" :disabled="writeBlocked" :title="writeBlocked ? t(blockedMessageKey('offlineAdminBlocked')) : undefined" :data-testid="`downsample-toggle-${policy.name}`" @click="togglePolicy(policy)">
                  <component :is="policy.enabled ? Pause : Play" class="h-4 w-4" />
                </button>
                <button type="button" class="rounded p-1 text-slate-400 hover:text-slate-600" :title="t('downsampleDetailOpen')" :data-testid="`downsample-detail-${policy.name}`" @click="openPolicyDetail(policy.name)">
                  <Eye class="h-4 w-4" />
                </button>
                <button type="button" class="rounded p-1 text-slate-400 hover:text-red-600 disabled:cursor-not-allowed disabled:opacity-40" :disabled="writeBlocked" :title="writeBlocked ? t(blockedMessageKey('offlineAdminBlocked')) : undefined" :data-testid="`downsample-delete-${policy.name}`" @click="requestDelete(policy.name)">
                  <Trash2 class="h-4 w-4" />
                </button>
              </div>
            </div>
          </div>
        </template>
      </VirtualTable>
      <p class="border-t border-slate-100 px-3 py-1.5 text-[11px] mts-muted dark:border-slate-800" data-testid="downsample-virtual-hint">
        {{ t('downsampleVirtualHint') }}
      </p>
    </div>

    <div
      v-if="detailPolicyName.trim()"
      id="downsample-detail"
      class="mts-card scroll-mt-20 overflow-hidden"
      data-testid="downsample-detail-panel"
    >
      <div class="flex flex-wrap items-center justify-between gap-2 border-b border-slate-100 px-3 py-2 dark:border-slate-800">
        <div class="min-w-0">
          <p class="truncate font-mono text-sm font-medium text-slate-800 dark:text-slate-100" data-testid="downsample-detail-name">{{ detailPolicy?.name || detailPolicyName }}</p>
          <p v-if="detailPolicy" class="text-xs mts-muted" data-testid="downsample-detail-enabled">{{ detailEnabledLabel(detailPolicy.enabled) }}</p>
          <p v-else-if="detailPolicyMissing" class="text-xs text-amber-700 dark:text-amber-300" data-testid="downsample-detail-missing">{{ t('downsampleDetailMissing') }}</p>
          <p v-else class="text-xs mts-muted" data-testid="downsample-detail-loading">{{ t('loading') }}</p>
        </div>
        <div class="flex flex-wrap gap-2">
          <button type="button" class="mts-btn" data-testid="downsample-detail-copy-json" :disabled="!detailPolicy" @click="copyPolicyDetailJSON">{{ t('downsampleDetailCopyJSON') }}</button>
          <button type="button" class="mts-btn" data-testid="downsample-detail-copy-md" :disabled="!detailPolicy" @click="copyPolicyDetailMarkdown">{{ t('downsampleDetailCopyMarkdown') }}</button>
          <button type="button" class="mts-btn" data-testid="downsample-detail-copy-link" :disabled="!detailPolicyName.trim()" @click="copyPolicyDetailLink">{{ t('downsampleDetailCopyLink') }}</button>
          <button type="button" class="mts-btn" data-testid="downsample-detail-close" @click="closePolicyDetail">{{ t('downsampleDetailClose') }}</button>
        </div>
      </div>
      <template v-if="detailPolicy">
      <dl class="grid gap-2 p-3 sm:grid-cols-2" data-testid="downsample-detail-fields">
        <div v-for="f in detailFields.filter((x) => x.key !== 'functions')" :key="f.key" class="min-w-0 rounded border border-slate-100 px-2 py-1.5 dark:border-slate-800" :data-testid="`downsample-detail-field-${f.key}`">
          <dt class="text-[11px] mts-muted">{{ f.label }}</dt>
          <dd class="mt-0.5 break-all text-xs text-slate-800 dark:text-slate-100" :class="f.mono ? 'font-mono' : ''">{{ f.key === 'enabled' ? detailEnabledLabel(detailPolicy.enabled) : f.value }}</dd>
        </div>
      </dl>
      <div class="border-t border-slate-100 px-3 py-2 dark:border-slate-800" data-testid="downsample-detail-functions">
        <p class="text-[11px] mts-muted">{{ t('downsampleFunctions') }}</p>
        <ul v-if="detailFunctions.length" class="mt-1 space-y-1" data-testid="downsample-detail-functions-list">
          <li v-for="(fn, idx) in detailFunctions" :key="idx" class="font-mono text-xs text-slate-800 dark:text-slate-100" :data-testid="`downsample-detail-fn-${idx}`">{{ fn }}</li>
        </ul>
        <p v-else class="mt-1 text-xs mts-muted" data-testid="downsample-detail-functions-empty">{{ t('emptyValue') }}</p>
      </div>
      <div v-if="detailStatus" class="border-t border-slate-100 px-3 py-2 text-xs dark:border-slate-800" data-testid="downsample-detail-status">
        <p class="mts-muted">{{ t('downsampleStatusPanel') }}</p>
        <p class="mt-1 font-mono text-slate-700 dark:text-slate-200" data-testid="downsample-detail-status-main">
          {{ t('downsampleColCompleted') }}: {{ formatUnix(detailStatus.completed_until_unix) }}
          · {{ t('downsampleColLastRun') }}: {{ formatUnix(detailStatus.last_run_unix) }}
          · {{ t('downsampleColLastSuccess') }}: {{ formatUnix(detailStatus.last_success_unix || 0) }}
        </p>
        <p class="mt-1 font-mono text-slate-700 dark:text-slate-200" data-testid="downsample-detail-status-extra">
          {{ t('downsampleColNextRun') }}: {{ detailStatus.next_run_unix ? formatUnix(detailStatus.next_run_unix) : t('emptyValue') }}
          · {{ t('downsampleColLag') }}: {{ detailStatus.lag_seconds != null ? `${detailStatus.lag_seconds}s` : t('emptyValue') }}
          · {{ t('downsampleColLastDuration') }}: {{ detailStatus.last_duration ? formatDuration(detailStatus.last_duration) : t('emptyValue') }}
        </p>
        <p class="mt-1 font-mono text-slate-700 dark:text-slate-200" data-testid="downsample-detail-status-stats">
          {{ t('downsampleColWindows') }}: {{ detailStatus.windows_processed ?? t('emptyValue') }}
          · {{ t('downsampleColPoints') }}: {{ detailStatus.points_written ?? t('emptyValue') }}
          · {{ t('downsampleColActive') }}: {{ detailStatus.active ? t('downsampleEnabledOnly') : t('downsampleDisabledOnly') }}
        </p>
        <p v-if="detailStatus.last_error" class="mt-1 text-red-600 dark:text-red-300" data-testid="downsample-detail-status-error">{{ detailStatus.last_error }}</p>
      </div>
      </template>
    </div>

    <div id="downsample-status" class="mts-card scroll-mt-20 overflow-hidden p-0" data-testid="downsample-status-panel">
      <div class="border-b border-slate-100 px-4 py-3 dark:border-slate-800">
        <h2 class="text-sm font-semibold text-slate-800 dark:text-slate-100">{{ t('downsampleStatusPanel') }}</h2>
        <p class="text-[11px] mts-muted">{{ t('downsampleStatusPanelHint') }}</p>
        <div class="mt-2 flex flex-wrap items-end gap-3" data-testid="downsample-status-filter-bar">
          <label class="text-xs mts-muted">{{ t('downsampleStatusHealthFilter') }}
            <select v-model="statusHealthFilter" class="mts-input mt-1" data-testid="downsample-status-health-filter">
              <option value="">{{ t('downsampleStatusHealthAll') }}</option>
              <option value="error">{{ t('downsampleStatusHealthError') }}</option>
              <option value="active">{{ t('downsampleStatusHealthActive') }}</option>
              <option value="lagging">{{ t('downsampleStatusHealthLagging') }}</option>
            </select>
          </label>
          <label v-if="statusHealthFilter === 'lagging'" class="text-xs mts-muted">{{ t('downsampleStatusMinLag') }}
            <input v-model.number="statusMinLag" type="number" min="0" step="1" class="mts-input mt-1 w-28" data-testid="downsample-status-min-lag" />
          </label>
          <span class="text-xs mts-muted" data-testid="downsample-status-filter-count">{{ filteredStatuses.length }} / {{ statuses.length }}</span>
          <button type="button" class="mts-btn" data-testid="downsample-status-clear-filters" @click="statusHealthFilter = ''; statusMinLag = 0">{{ t('clearFilters') }}</button>
        </div>
      </div>
      <div v-if="!filteredStatuses.length" class="p-4">
        <EmptyState compact :title="t('downsampleStatusEmpty')" :description="t('downsampleStatusEmptyDesc')" />
      </div>
      <div v-else class="overflow-x-auto">
        <div class="overflow-hidden rounded-lg border border-slate-100 dark:border-slate-800" data-testid="downsample-status-table">
          <div
            class="grid grid-cols-[minmax(6.5rem,1fr)_minmax(6.5rem,1fr)_minmax(6.5rem,1fr)_minmax(6.5rem,1fr)_minmax(6rem,0.9fr)_minmax(4.5rem,0.55fr)_minmax(6rem,0.8fr)] border-b border-slate-100 bg-slate-50/50 text-left text-xs uppercase text-slate-500 dark:border-slate-800 dark:text-slate-400"
            data-testid="downsample-status-header"
          >
            <div class="px-3 py-2.5">{{ t('downsampleColName') }}</div>
            <div class="px-3 py-2.5">{{ t('downsampleColCompleted') }}</div>
            <div class="px-3 py-2.5">{{ t('downsampleColLastRun') }}</div>
            <div class="px-3 py-2.5">{{ t('downsampleColLastSuccess') }}</div>
            <div class="px-3 py-2.5">{{ t('downsampleColNextRun') }}</div>
            <div class="px-3 py-2.5">{{ t('downsampleColLag') }}</div>
            <div class="px-3 py-2.5">{{ t('downsampleColError') }}</div>
          </div>
          <VirtualTable
            :items="filteredStatuses"
            :row-height="STATUS_ROW_HEIGHT"
            :height="Math.min(DOWNSAMPLE_LIST_HEIGHT, Math.max(176, filteredStatuses.length * STATUS_ROW_HEIGHT))"
            data-testid="downsample-status-virtual-list"
          >
            <template #default="{ item: st }">
              <div
                role="button"
                tabindex="0"
                class="grid h-full cursor-pointer grid-cols-[minmax(6.5rem,1fr)_minmax(6.5rem,1fr)_minmax(6.5rem,1fr)_minmax(6.5rem,1fr)_minmax(6rem,0.9fr)_minmax(4.5rem,0.55fr)_minmax(6rem,0.8fr)] items-center border-b border-slate-50 hover:bg-slate-50/80 dark:border-slate-800 dark:hover:bg-slate-800/40"
                :data-testid="`downsample-status-row-${st.policy_name}`"
                :title="t('downsampleDetailOpen')"
                @click="openPolicyDetail(st.policy_name)"
                @keydown.enter.prevent="openPolicyDetail(st.policy_name)"
              >
                <div class="truncate px-3 font-medium text-sky-700 dark:text-sky-300">{{ st.policy_name }}</div>
                <div class="truncate px-3 text-xs mts-muted">{{ formatUnix(st.completed_until_unix) }}</div>
                <div class="truncate px-3 text-xs mts-muted">{{ formatUnix(st.last_run_unix) }}</div>
                <div class="truncate px-3 text-xs mts-muted">{{ formatUnix(st.last_success_unix || 0) }}</div>
                <div class="truncate px-3 text-xs mts-muted" :data-testid="`downsample-status-next-${st.policy_name}`">{{ st.next_run_unix ? formatUnix(st.next_run_unix) : t('emptyValue') }}</div>
                <div class="truncate px-3 text-xs mts-muted" :data-testid="`downsample-status-lag-${st.policy_name}`">{{ st.lag_seconds != null ? `${st.lag_seconds}s` : t('emptyValue') }}</div>
                <div class="truncate px-3 text-xs" :class="st.last_error ? 'text-red-600 dark:text-red-300' : 'mts-muted'">{{ st.last_error || t('emptyValue') }}</div>
              </div>
            </template>
          </VirtualTable>
          <p class="border-t border-slate-100 px-3 py-1.5 text-[11px] mts-muted dark:border-slate-800" data-testid="downsample-status-virtual-hint">
            {{ t('downsampleStatusVirtualHint') }}
          </p>
        </div>
      </div>
    </div>

    <div
      v-if="showCreate"
      class="fixed inset-0 z-50 flex items-center justify-center bg-black/30 p-4"
      @click.self="showCreate = false"
      @keydown.esc="showCreate = false"
    >
      <div ref="createPanelRef" class="max-h-[80vh] w-[480px] overflow-auto rounded-xl bg-white p-6 shadow-lg dark:bg-slate-900" role="dialog" aria-modal="true" data-testid="downsample-create-dialog">
        <h3 class="mb-4 text-sm font-semibold text-slate-800 dark:text-slate-100">{{ t('downsampleCreate') }}</h3>
        <div class="grid grid-cols-2 gap-3">
          <div>
            <label class="mb-1 block text-xs mts-muted">{{ t('downsampleColName') }}</label>
            <input v-model="newPolicy.name" data-testid="downsample-create-name" class="w-full rounded border border-slate-300 px-2 py-1.5 text-xs dark:border-slate-600" />
          </div>
          <div>
            <label class="mb-1 block text-xs mts-muted">{{ t('downsampleColInterval') }}</label>
            <input v-model="intervalHuman" data-testid="downsample-create-interval" :placeholder="t('downsamplePhInterval')" class="w-full rounded border border-slate-300 px-2 py-1.5 text-xs dark:border-slate-600" />
            <p class="mt-1 text-[11px] mts-muted">{{ t('downsampleIntervalHint') }}</p>
          </div>
          <div>
            <label class="mb-1 block text-xs mts-muted">{{ t('downsampleSourceDb') }}</label>
            <input
              v-model="newPolicy.source_database"
              list="downsample-db-list"
              data-testid="downsample-source-db"
              class="w-full rounded border border-slate-300 px-2 py-1.5 text-xs dark:border-slate-600"
            />
          </div>
          <div>
            <label class="mb-1 block text-xs mts-muted">{{ t('downsampleSourceMsmt') }}</label>
            <input
              v-model="newPolicy.source_measurement"
              list="downsample-source-meas-list"
              data-testid="downsample-source-measurement"
              class="w-full rounded border border-slate-300 px-2 py-1.5 text-xs dark:border-slate-600"
            />
          </div>
          <div>
            <label class="mb-1 block text-xs mts-muted">{{ t('downsampleTargetDb') }}</label>
            <input
              v-model="newPolicy.target_database"
              list="downsample-db-list"
              data-testid="downsample-target-db"
              class="w-full rounded border border-slate-300 px-2 py-1.5 text-xs dark:border-slate-600"
            />
          </div>
          <div>
            <label class="mb-1 block text-xs mts-muted">{{ t('downsampleTargetMsmt') }}</label>
            <input
              v-model="newPolicy.target_measurement"
              list="downsample-source-meas-list"
              data-testid="downsample-target-measurement"
              class="w-full rounded border border-slate-300 px-2 py-1.5 text-xs dark:border-slate-600"
            />
          </div>
          <div>
            <label class="mb-1 block text-xs mts-muted">{{ t('downsampleSourceRetention') }}</label>
            <input
              v-model="newPolicy.source_retention"
              data-testid="downsample-source-retention"
              class="w-full rounded border border-slate-300 px-2 py-1.5 text-xs dark:border-slate-600"
            />
          </div>
          <div>
            <label class="mb-1 block text-xs mts-muted">{{ t('downsampleTargetRetention') }}</label>
            <input
              v-model="newPolicy.target_retention"
              data-testid="downsample-target-retention"
              class="w-full rounded border border-slate-300 px-2 py-1.5 text-xs dark:border-slate-600"
            />
          </div>
          <div>
            <label class="mb-1 block text-xs mts-muted">{{ t('downsampleRefreshInterval') }}</label>
            <input
              v-model="refreshHuman"
              data-testid="downsample-create-refresh"
              :placeholder="t('downsamplePhInterval')"
              class="w-full rounded border border-slate-300 px-2 py-1.5 text-xs dark:border-slate-600"
            />
          </div>
          <div>
            <label class="mb-1 block text-xs mts-muted">{{ t('downsampleLookback') }}</label>
            <input
              v-model="lookbackHuman"
              data-testid="downsample-create-lookback"
              :placeholder="t('downsamplePhInterval')"
              class="w-full rounded border border-slate-300 px-2 py-1.5 text-xs dark:border-slate-600"
            />
          </div>
          <div>
            <label class="mb-1 block text-xs mts-muted">{{ t('downsampleBatchSize') }}</label>
            <input
              v-model.number="newPolicy.batch_size"
              type="number"
              min="1"
              data-testid="downsample-create-batch-size"
              class="w-full rounded border border-slate-300 px-2 py-1.5 text-xs dark:border-slate-600"
            />
          </div>
        </div>
        <datalist id="downsample-db-list">
          <option v-for="db in createDatabases" :key="db" :value="db" />
        </datalist>
        <datalist id="downsample-source-meas-list">
          <option v-for="m in createSourceMeasurements" :key="m" :value="m" />
        </datalist>
        <datalist id="downsample-source-field-list">
          <option v-for="f in createSourceFields" :key="f" :value="f" />
        </datalist>
        <p
          v-if="createMetaLoading || createMetaError || createDatabases.length || createSourceMeasurements.length"
          class="mt-2 text-[11px] mts-muted"
          data-testid="downsample-create-meta"
        >
          <span v-if="createMetaLoading">{{ t('loading') }}</span>
          <span v-else-if="createMetaError" class="text-rose-600">{{ createMetaError }}</span>
          <span v-else>{{ formatMessage(t('downsampleMetaHint'), { dbs: createDatabases.length, meas: createSourceMeasurements.length, fields: createSourceFields.length }) }}</span>
        </p>
        <div class="mt-3">
          <label class="mb-1 block text-xs mts-muted">{{ t('downsampleFunctions') }}</label>
          <div class="space-y-1.5">
            <div v-for="(fn, idx) in newPolicy.functions" :key="idx" class="flex items-center gap-1.5">
              <select v-model="fn.function" class="rounded border border-slate-300 px-1.5 py-1 text-xs dark:border-slate-600">
                <option v-for="opt in ['mean','sum','min','max','first','last','count']" :key="opt" :value="opt">{{ opt }}</option>
              </select>
              <input
                v-model="fn.field"
                list="downsample-source-field-list"
                data-testid="downsample-fn-field"
                :placeholder="t('downsamplePhField')"
                class="flex-1 rounded border border-slate-300 px-1.5 py-1 text-xs dark:border-slate-600"
              />
              <input v-model="fn.as" :placeholder="t('downsamplePhAs')" class="flex-1 rounded border border-slate-300 px-1.5 py-1 text-xs dark:border-slate-600" />
              <button class="rounded p-0.5 text-slate-400 hover:text-red-600" @click="removePolicyFunction(idx)"><Trash2 class="h-3.5 w-3.5" /></button>
            </div>
          </div>
          <button class="mt-1.5 inline-flex items-center gap-1 text-xs mts-muted" @click="addPolicyFunction">
            <Plus class="h-3 w-3" /> {{ t('downsampleAddFunction') }}
          </button>
        </div>
        <div class="mt-3">
          <label class="mb-1 block text-xs mts-muted">{{ t('downsampleGroupByTags') }}</label>
          <input v-model="newPolicyTagsText" :placeholder="t('downsamplePhGroupTags')" class="w-full rounded border border-slate-300 px-2 py-1.5 text-xs dark:border-slate-600" />
        </div>
        <div class="mt-4 flex justify-end gap-2">
          <button class="rounded-lg px-4 py-2 text-sm text-slate-600 hover:bg-slate-100 dark:text-slate-300 dark:hover:bg-slate-800" @click="showCreate = false">{{ t('cancel') }}</button>
          <button class="rounded-lg bg-slate-800 px-4 py-2 text-sm font-medium text-white disabled:opacity-50" data-testid="downsample-create-submit" :disabled="writeBlocked || createLoading" :title="writeBlocked ? t(blockedMessageKey('offlineAdminBlocked')) : undefined" @click="createPolicy">{{ createLoading ? t('loading') : t('create') }}</button>
        </div>
      </div>
    </div>

    <ConfirmDialog
      v-model:open="deleteOpen"
      :write-blocked="writeBlocked"
      :block-reason="blockReason"
      :offline-message-key="'offlineAdminBlocked'"
      :title="t('downsampleDeleteTitle')"
      :message="`${t('downsampleDeleteMsg')} ${deleteName}`"
      :confirm-label="t('delete')"
      danger
      :loading="deleteLoading"
      allow-cancel-while-loading
      @confirm="confirmDelete"
      @cancel="cancelDsAction"
    />
    <ConfirmDialog
      v-model:open="batchOpen"
      :write-blocked="writeBlocked"
      :block-reason="blockReason"
      :offline-message-key="'offlineAdminBlocked'"
      :title="batchMode === 'enable' ? t('downsampleBatchEnableTitle') : t('downsampleBatchDisableTitle')"
      :message="(batchMode === 'enable' ? t('downsampleBatchEnableMsg') : t('downsampleBatchDisableMsg')) + ` (${selectedNames.length})`"
      :confirm-label="batchMode === 'enable' ? t('downsampleBatchEnable') : t('downsampleBatchDisable')"
      :danger="batchMode === 'disable'"
      :loading="batchLoading"
      allow-cancel-while-loading
      @confirm="confirmBatch"
      @cancel="cancelDsAction"
    />

    <div
      v-if="rangeOpen"
      class="fixed inset-0 z-50 flex items-center justify-center bg-black/30 p-4"
      data-testid="downsample-range-dialog"
      @click.self="onRangeCancel"
      @keydown.esc="onRangeCancel"
    >
      <div ref="rangePanelRef" class="w-[440px] rounded-xl bg-white p-6 shadow-lg dark:bg-slate-900" role="dialog" aria-modal="true">
        <h3 class="mb-2 text-sm font-semibold text-slate-800 dark:text-slate-100">{{ rangeTitle }}</h3>
        <p class="mb-1 text-xs mts-muted">{{ rangeHint }}</p>
        <p class="mb-3 text-[11px] mts-muted">{{ rangeName }} · {{ t('downsampleRepairRangeHint') }}</p>
        <div class="grid grid-cols-1 gap-3">
          <label class="text-xs mts-muted">{{ t('downsampleStartUnix') }}
            <input v-model.number="rangeStartUnix" type="number" class="mts-input mt-1 w-full" data-testid="downsample-range-start" />
          </label>
          <label class="text-xs mts-muted">{{ t('downsampleEndUnix') }}
            <input v-model.number="rangeEndUnix" type="number" class="mts-input mt-1 w-full" data-testid="downsample-range-end" />
          </label>
          <label v-if="rangeMode === 'run-range'" class="flex items-center gap-2 text-xs mts-muted">
            <input v-model="rangeAdvanceWatermark" type="checkbox" data-testid="downsample-range-advance" />
            {{ t('downsampleAdvanceWatermark') }}
          </label>
        </div>
        <p
          v-if="writeBlocked"
          class="mt-3 rounded-lg border border-amber-200 bg-amber-50 px-2 py-1.5 text-[11px] text-amber-900 dark:border-amber-900 dark:bg-amber-950/40 dark:text-amber-100"
          data-testid="downsample-range-blocked"
        >{{ t(blockedMessageKey('offlineAdminBlocked')) }}</p>
        <div class="mt-4 flex justify-end gap-2">
          <button type="button" class="mts-btn" data-testid="downsample-range-cancel" @click="onRangeCancel">{{ t('cancel') }}</button>
          <button
            type="button"
            class="mts-btn-primary"
            data-testid="downsample-range-confirm"
            :disabled="rangeLoading || writeBlocked"
            :title="writeBlocked ? t(blockedMessageKey('offlineAdminBlocked')) : undefined"
            @click="confirmRange"
          >
            {{ rangeLoading ? t('loading') : rangeConfirmLabel }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>
