<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount, watch, nextTick } from 'vue'
import { useHashScroll } from '@/composables/useHashScroll'
import { useRoute } from 'vue-router'
import { parseDownsamplePrefill, downsampleFormToPrefill } from '@/utils/routePrefill'
import { copyText } from '@/utils/clipboard'
import { apiGet, apiPost, apiDelete } from '@/api/client'
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
import ConfirmDialog from '@/components/ConfirmDialog.vue'
import ActionResultBanner from '@/components/ActionResultBanner.vue'
import EmptyState from '@/components/EmptyState.vue'
import ListSelectionToolbar from '@/components/ListSelectionToolbar.vue'
import VirtualTable from '@/components/VirtualTable.vue'
import { useNotify } from '@/composables/useNotify'
import { formatCaughtError } from '@/utils/apiError'
import { formatMessage } from '@/utils/formatMessage'
import { createFocusTrap, type FocusTrapHandle } from '@/utils/focusTrap'
import { filterDownsamplePolicies, type DownsampleEnabledFilter } from '@/utils/listFilter'
import { useI18n } from '@/composables/useI18n'
import { buildDownsampleExport, downsampleToCSV } from '@/utils/downsampleExport'
import { stampFilename } from '@/utils/download'
import { useExportJob } from '@/composables/useExportJob'
import ExportJobBanner from '@/components/ExportJobBanner.vue'
import { makeActionResult, type ActionResult } from '@/utils/actionResult'
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
  Plus, Trash2, Play, Pause, RefreshCw, PlayCircle, RotateCcw, FlaskConical, Wrench, Timer, Download,
} from 'lucide-vue-next'
import type {
  DownsamplePolicy, DownsampleStatus, DownsampleRunResult, DownsampleDryRunResult,
} from '@/api/types'

interface PoliciesResponse { policies: DownsamplePolicy[] }
interface StatusesResponse { statuses: DownsampleStatus[] }

useHashScroll()
const route = useRoute()
const { isAdmin } = useAuth()
const { offline, writeBlocked, blockReason, blockedMessageKey } = useMutationGuard()
const { t, locale } = useI18n()
const { success, info, error: notifyError, warn } = useNotify()
const {
  exportJob,
  exportBusy,
  cancelExport,
  resetExport,
  runTextExport,
  runJSONExport,
} = useExportJob()
const policies = ref<DownsamplePolicy[]>([])
const statuses = ref<DownsampleStatus[]>([])
const loadError = ref('')
const actionResult = ref<ActionResult | null>(null)
type DsActionKey = 'create' | 'delete' | 'toggle' | 'batch' | 'range'
const lastFailedAction = ref<DsActionKey | null>(null)
const lastToggleName = ref('')
const lastToggleWantEnabled = ref(false)
const policyFilter = ref('')
const enabledFilter = ref<DownsampleEnabledFilter>('')
const selectedNames = ref<string[]>([])
const batchOpen = ref(false)
const batchMode = ref<'enable' | 'disable'>('enable')
const batchLoading = ref(false)

const rangeOpen = ref(false)
const rangeMode = ref<DownsampleRangeMode>('repair')
const rangeName = ref('')
const rangeStartUnix = ref(0)
const rangeEndUnix = ref(0)
const rangeAdvanceWatermark = ref(false)
const rangeLoading = ref(false)
const showCreate = ref(false)
const intervalHuman = ref('1m')
const deleteOpen = ref(false)
const deleteName = ref('')
const deleteLoading = ref(false)
const newPolicy = ref<DownsamplePolicy>({
  name: '', source_database: '', source_measurement: '',
  target_database: '', target_measurement: '',
  interval: 60_000_000_000, functions: [{ function: 'mean', field: 'value', as: 'mean_value' }],
  group_by_tags: [], enabled: true,
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
  createMetaError.value = ''
  try {
    createDatabases.value = await listDatabases()
  } catch (e) {
    createDatabases.value = []
    createMetaError.value = formatCaughtError(e)
  }
}

async function loadCreateSourceMeasurements(db: string) {
  createSourceMeasurements.value = []
  createSourceFields.value = []
  if (!db.trim()) return
  createMetaLoading.value = true
  createMetaError.value = ''
  try {
    createSourceMeasurements.value = await listMeasurements(db)
  } catch (e) {
    createSourceMeasurements.value = []
    createMetaError.value = formatCaughtError(e)
  } finally {
    createMetaLoading.value = false
  }
}

async function loadCreateSourceFields(db: string, measurement: string) {
  createSourceFields.value = []
  if (!db.trim() || !measurement.trim()) return
  createMetaLoading.value = true
  try {
    createSourceFields.value = fieldNames(await listFields(db, measurement))
  } catch (e) {
    createSourceFields.value = []
    createMetaError.value = formatCaughtError(e)
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

async function loadData() {
  if (!isAdmin.value) return
  try {
    const [polData, statData] = await Promise.all([
      apiGet<PoliciesResponse>('/api/v1/admin/downsample/policies'),
      apiGet<StatusesResponse>('/api/v1/admin/downsample/statuses'),
    ])
    policies.value = polData.policies ?? []
    statuses.value = statData.statuses ?? []
  } catch (e) {
    loadError.value = formatCaughtError(e)
  }
}

const filteredPolicies = computed(() =>
  filterDownsamplePolicies(policies.value, policyFilter.value, enabledFilter.value),
)

const filteredStatuses = computed(() => {
  const names = new Set(filteredPolicies.value.map((p) => p.name))
  // 若无策略列表过滤命中但 statuses 仍有数据，按同名过滤；无策略时展示全部 statuses
  if (!policies.value.length) return statuses.value
  if (!policyFilter.value && !enabledFilter.value) return statuses.value
  return statuses.value.filter((s) => names.has(s.policy_name))
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
  if (writeBlocked.value) {
    notifyError(t.value(blockedMessageKey('offlineAdminBlocked')))
    return
  }
  if (!selectedNames.value.length) return
  batchMode.value = mode
  batchOpen.value = true
}


function clearActionResult() {
  actionResult.value = null
  lastFailedAction.value = null
}

function reportActionError(key: DsActionKey, e: unknown) {
  lastFailedAction.value = key
  const msg = formatCaughtError(e)
  actionResult.value = makeActionResult('error', msg)
  notifyError(msg)
}

async function retryLastDownsampleAction() {
  const key = lastFailedAction.value
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
}

async function confirmBatch() {
  if (writeBlocked.value) {
    const msg = t.value(blockedMessageKey('offlineAdminBlocked'))
    actionResult.value = makeActionResult('error', msg)
    notifyError(msg)
    return
  }
  const names = [...selectedNames.value]
  if (!names.length) {
    batchOpen.value = false
    return
  }
  batchLoading.value = true
  clearActionResult()
  let ok = 0
  const errors: string[] = []
  const action = batchMode.value === 'enable' ? 'enable' : 'disable'
  try {
    for (const name of names) {
      try {
        await apiPost(`/api/v1/admin/downsample/policies/${encodeURIComponent(name)}/${action}`)
        ok += 1
      } catch (e) {
        errors.push(`${name}: ${formatCaughtError(e)}`)
      }
    }
    await loadData()
    selectedNames.value = []
    batchOpen.value = false
    if (errors.length === 0) {
      const msg = batchMode.value === 'enable'
        ? `${t.value('downsampleBatchEnable')}: ${ok}`
        : `${t.value('downsampleBatchDisable')}: ${ok}`
      actionResult.value = makeActionResult('ok', msg)
      success(msg)
    } else {
      const msg = `${ok}/${names.length}; ${errors.slice(0, 3).join('; ')}`
      if (ok === 0) {
        lastFailedAction.value = 'batch'
        actionResult.value = makeActionResult('error', msg)
        notifyError(msg)
      } else {
        actionResult.value = makeActionResult('info', msg)
        success(msg)
      }
    }
  } finally {
    batchLoading.value = false
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
    actionResult.value = makeActionResult('error', msg)
    notifyError(msg)
    return
  }
  if (!newPolicy.value.name.trim()) return
  clearActionResult()
  try {
    newPolicy.value.interval = parseHumanDurationToNs(intervalHuman.value)
  } catch (e) {
    const msg = formatCaughtError(e)
    actionResult.value = makeActionResult('error', msg)
    notifyError(msg)
    return
  }
  if (!newPolicy.value.source_database || !newPolicy.value.source_measurement) {
    const msg = t.value('downsampleNeedSource')
    actionResult.value = makeActionResult('error', msg)
    notifyError(msg)
    return
  }
  try {
    await apiPost('/api/v1/admin/downsample/policies', newPolicy.value)
    showCreate.value = false
    newPolicy.value = {
      name: '', source_database: '', source_measurement: '',
      target_database: '', target_measurement: '',
      interval: 60_000_000_000, functions: [{ function: 'mean', field: 'value', as: 'mean_value' }],
      group_by_tags: [], enabled: true,
    }
    intervalHuman.value = '1m'
    await loadData()
    lastFailedAction.value = null
    actionResult.value = makeActionResult('ok', t.value('downsampleCreated'))
    success(t.value('downsampleCreated'))
  } catch (e) {
    reportActionError('create', e)
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
    actionResult.value = makeActionResult('error', msg)
    notifyError(msg)
    return
  }
  deleteLoading.value = true
  try {
    await apiDelete(`/api/v1/admin/downsample/policies/${encodeURIComponent(deleteName.value)}`)
    deleteOpen.value = false
    await loadData()
    lastFailedAction.value = null
    actionResult.value = makeActionResult('ok', t.value('downsampleDeleted'))
    success(t.value('downsampleDeleted'))
  } catch (e) {
    reportActionError('delete', e)
  } finally {
    deleteLoading.value = false
  }
}

async function togglePolicy(policy: DownsamplePolicy) {
  if (writeBlocked.value) {
    const msg = t.value(blockedMessageKey('offlineAdminBlocked'))
    actionResult.value = makeActionResult('error', msg)
    notifyError(msg)
    return
  }
  const action = policy.enabled ? 'disable' : 'enable'
  lastToggleName.value = policy.name
  lastToggleWantEnabled.value = !policy.enabled
  try {
    await apiPost(`/api/v1/admin/downsample/policies/${encodeURIComponent(policy.name)}/${action}`)
    await loadData()
    lastFailedAction.value = null
    const msg = policy.enabled ? t.value('downsampleDisabledOk') : t.value('downsampleEnabledOk')
    actionResult.value = makeActionResult('ok', msg)
    success(msg)
  } catch (e) {
    reportActionError('toggle', e)
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
    actionResult.value = makeActionResult('error', msg)
    notifyError(msg)
    return
  }
  try {
    const data = await apiPost<{ result: DownsampleRunResult }>(
      `/api/v1/admin/downsample/policies/${encodeURIComponent(name)}/run`,
      {},
    )
    const msg = formatRunResultMessage('run', name, data.result)
    actionResult.value = makeActionResult('ok', msg)
    await loadData()
    success(msg)
  } catch (e) {
    const msg = formatCaughtError(e)
    actionResult.value = makeActionResult('error', msg)
    notifyError(msg)
  }
}

async function resetPolicy(name: string) {
  if (writeBlocked.value) {
    const msg = t.value(blockedMessageKey('offlineAdminBlocked'))
    actionResult.value = makeActionResult('error', msg)
    notifyError(msg)
    return
  }
  try {
    await apiPost(`/api/v1/admin/downsample/policies/${encodeURIComponent(name)}/reset`, {
      reset: { allow_policy_replace: true },
    })
    await loadData()
    const msg = `${t.value('downsampleResetOk')}: ${name}`
    actionResult.value = makeActionResult('ok', msg)
    success(msg)
  } catch (e) {
    const msg = formatCaughtError(e)
    actionResult.value = makeActionResult('error', msg)
    notifyError(msg)
  }
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
    actionResult.value = makeActionResult('error', msg)
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
    actionResult.value = makeActionResult('error', msg)
    notifyError(msg)
    return
  }
  rangeLoading.value = true
  clearActionResult()
  try {
    const body = buildDownsampleRangeBody({
      startUnix: rangeStartUnix.value,
      endUnix: rangeEndUnix.value,
      advanceWatermark: rangeMode.value === 'run-range' ? rangeAdvanceWatermark.value : false,
    })
    const path = rangeActionPath(rangeName.value, rangeMode.value)
    if (rangeMode.value === 'dry-run') {
      const data = await apiPost<{ result: DownsampleDryRunResult }>(path, body)
      const msg = formatRunResultMessage('dry-run', rangeName.value, data.result)
      rangeOpen.value = false
      lastFailedAction.value = null
      actionResult.value = makeActionResult('info', msg)
      success(msg)
    } else {
      const data = await apiPost<{ result: DownsampleRunResult }>(path, body)
      const msg = formatRunResultMessage(rangeMode.value, rangeName.value, data.result)
      rangeOpen.value = false
      await loadData()
      lastFailedAction.value = null
      actionResult.value = makeActionResult('ok', msg)
      success(msg)
    }
  } catch (e) {
    reportActionError('range', e)
  } finally {
    rangeLoading.value = false
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
        <p class="text-xs text-slate-500 dark:text-slate-400">{{ t('downsampleDesc') }}</p>
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
    <ActionResultBanner
      :result="actionResult"
      :retryable="!!(actionResult && actionResult.kind === 'error' && lastFailedAction)"
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
          <button type="button" class="mts-btn" data-testid="downsample-batch-enable" :disabled="!selectedNames.length || writeBlocked" :title="writeBlocked ? t(blockedMessageKey('offlineAdminBlocked')) : undefined" @click="openBatch('enable')">{{ t('downsampleBatchEnable') }}</button>
          <button type="button" class="mts-btn" data-testid="downsample-batch-disable" :disabled="!selectedNames.length || writeBlocked" :title="writeBlocked ? t(blockedMessageKey('offlineAdminBlocked')) : undefined" @click="openBatch('disable')">{{ t('downsampleBatchDisable') }}</button>
          <ExportJobBanner class="w-full basis-full" :job="exportJob" @cancel="cancelExport" @dismiss="resetExport" />
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
      <EmptyState data-testid="downsample-empty-filter" :title="t('downsampleFilterEmpty')" :description="t('downsampleFilterEmptyDesc')" />
    </div>

    <div v-else class="overflow-hidden rounded-xl border border-slate-200 bg-white dark:border-slate-700 dark:bg-slate-900" data-testid="downsample-policies-table">
      <div
        class="grid grid-cols-[2.5rem_minmax(7rem,0.9fr)_minmax(10rem,1.4fr)_minmax(5rem,0.6fr)_minmax(5rem,0.6fr)_minmax(7rem,0.9fr)_minmax(11rem,1.2fr)] border-b border-slate-100 bg-slate-50/50 text-left text-xs uppercase text-slate-500 dark:border-slate-800 dark:bg-slate-900/40 dark:text-slate-400"
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
        <div class="px-3 py-2.5">{{ t('downsampleColInterval') }}</div>
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
            class="grid h-full grid-cols-[2.5rem_minmax(7rem,0.9fr)_minmax(10rem,1.4fr)_minmax(5rem,0.6fr)_minmax(5rem,0.6fr)_minmax(7rem,0.9fr)_minmax(11rem,1.2fr)] items-center border-b border-slate-50 dark:border-slate-800"
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
            <div class="truncate px-3 font-medium text-slate-700 dark:text-slate-200" :title="policy.name">{{ policy.name }}</div>
            <div class="truncate px-3 text-xs text-slate-600 dark:text-slate-300" :title="`${policy.source_database}/${policy.source_measurement} → ${policy.target_database}/${policy.target_measurement}`">
              {{ policy.source_database }}/{{ policy.source_measurement }} → {{ policy.target_database }}/{{ policy.target_measurement }}
            </div>
            <div class="px-3 text-slate-600 dark:text-slate-300">{{ formatDuration(policy.interval) }}</div>
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
                <button type="button" class="rounded p-1 text-slate-400 hover:text-slate-600 disabled:cursor-not-allowed disabled:opacity-40" :disabled="writeBlocked" :title="writeBlocked ? t(blockedMessageKey('offlineAdminBlocked')) : t('downsampleRunTitle')" :data-testid="`downsample-run-${policy.name}`" @click="runPolicy(policy.name)">
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
                <button type="button" class="rounded p-1 text-slate-400 hover:text-slate-600 disabled:cursor-not-allowed disabled:opacity-40" :disabled="writeBlocked" :title="writeBlocked ? t(blockedMessageKey('offlineAdminBlocked')) : t('downsampleResetTitle')" :data-testid="`downsample-reset-${policy.name}`" @click="resetPolicy(policy.name)">
                  <RotateCcw class="h-4 w-4" />
                </button>
                <button type="button" class="rounded p-1 text-slate-400 hover:text-slate-600 disabled:cursor-not-allowed disabled:opacity-40" :disabled="writeBlocked" :title="writeBlocked ? t(blockedMessageKey('offlineAdminBlocked')) : undefined" :data-testid="`downsample-toggle-${policy.name}`" @click="togglePolicy(policy)">
                  <component :is="policy.enabled ? Pause : Play" class="h-4 w-4" />
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

    <div id="downsample-status" class="mts-card scroll-mt-20 overflow-hidden p-0" data-testid="downsample-status-panel">
      <div class="border-b border-slate-100 px-4 py-3 dark:border-slate-800">
        <h2 class="text-sm font-semibold text-slate-800 dark:text-slate-100">{{ t('downsampleStatusPanel') }}</h2>
        <p class="text-[11px] mts-muted">{{ t('downsampleStatusPanelHint') }}</p>
      </div>
      <div v-if="!filteredStatuses.length" class="p-4">
        <EmptyState compact :title="t('downsampleStatusEmpty')" :description="t('downsampleStatusEmptyDesc')" />
      </div>
      <div v-else class="overflow-x-auto">
        <div class="overflow-hidden rounded-lg border border-slate-100 dark:border-slate-800" data-testid="downsample-status-table">
          <div
            class="grid grid-cols-[minmax(7rem,1fr)_minmax(7rem,1fr)_minmax(7rem,1fr)_minmax(7rem,1fr)_minmax(5rem,0.6fr)_minmax(6rem,0.8fr)] border-b border-slate-100 bg-slate-50/50 text-left text-xs uppercase text-slate-500 dark:border-slate-800 dark:text-slate-400"
            data-testid="downsample-status-header"
          >
            <div class="px-3 py-2.5">{{ t('downsampleColName') }}</div>
            <div class="px-3 py-2.5">{{ t('downsampleColCompleted') }}</div>
            <div class="px-3 py-2.5">{{ t('downsampleColLastRun') }}</div>
            <div class="px-3 py-2.5">{{ t('downsampleColLastSuccess') }}</div>
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
                class="grid h-full grid-cols-[minmax(7rem,1fr)_minmax(7rem,1fr)_minmax(7rem,1fr)_minmax(7rem,1fr)_minmax(5rem,0.6fr)_minmax(6rem,0.8fr)] items-center border-b border-slate-50 dark:border-slate-800"
                :data-testid="`downsample-status-row-${st.policy_name}`"
              >
                <div class="truncate px-3 font-medium text-slate-700 dark:text-slate-200">{{ st.policy_name }}</div>
                <div class="truncate px-3 text-xs mts-muted">{{ formatUnix(st.completed_until_unix) }}</div>
                <div class="truncate px-3 text-xs mts-muted">{{ formatUnix(st.last_run_unix) }}</div>
                <div class="truncate px-3 text-xs mts-muted">{{ formatUnix(st.last_success_unix || 0) }}</div>
                <div class="truncate px-3 text-xs mts-muted">{{ st.lag_seconds != null ? `${st.lag_seconds}s` : t('emptyValue') }}</div>
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
            <input v-model="newPolicy.name" class="w-full rounded border border-slate-300 px-2 py-1.5 text-xs dark:border-slate-600" />
          </div>
          <div>
            <label class="mb-1 block text-xs mts-muted">{{ t('downsampleColInterval') }}</label>
            <input v-model="intervalHuman" :placeholder="t('downsamplePhInterval')" class="w-full rounded border border-slate-300 px-2 py-1.5 text-xs dark:border-slate-600" />
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
          <button class="rounded-lg bg-slate-800 px-4 py-2 text-sm font-medium text-white disabled:opacity-50" data-testid="downsample-create-submit" :disabled="writeBlocked" :title="writeBlocked ? t(blockedMessageKey('offlineAdminBlocked')) : undefined" @click="createPolicy">{{ t('create') }}</button>
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
      @confirm="confirmDelete"
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
      @confirm="confirmBatch"
    />

    <div
      v-if="rangeOpen"
      class="fixed inset-0 z-50 flex items-center justify-center bg-black/30 p-4"
      data-testid="downsample-range-dialog"
      @click.self="!rangeLoading && (rangeOpen = false)"
      @keydown.esc="!rangeLoading && (rangeOpen = false)"
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
          <button type="button" class="mts-btn" :disabled="rangeLoading" @click="rangeOpen = false">{{ t('cancel') }}</button>
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
