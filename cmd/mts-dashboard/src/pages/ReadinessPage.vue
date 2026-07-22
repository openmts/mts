<script setup lang="ts">
import { computed, inject, onMounted, onBeforeUnmount, ref, watch, type ComputedRef } from 'vue'
import { registerDirtyChecker } from '@/utils/routeDirty'
import { snapshotForm } from '@/utils/formDirty'
import { useRoute, useRouter } from 'vue-router'
import { apiGet } from '@/api/client'
import { formatCaughtError } from '@/utils/apiError'
import { useAuth } from '@/composables/useAuth'
import { useAdminOpBusy } from '@/composables/useAdminOpBusy'
import { adminOpKindLabelKey, isAdminHeavyBusyMessage, joinAdminOpChip, parseAdminOpStatusPayload } from '@/utils/adminOpBusy'
import type { MessageKey } from '@/i18n/messages'
import { useNotify } from '@/composables/useNotify'
import { useNotifyAdminBusy } from '@/composables/useNotifyAdminBusy'
import { useI18n } from '@/composables/useI18n'
import { useExportJob } from '@/composables/useExportJob'
import ExportJobBanner from '@/components/ExportJobBanner.vue'
import { formatMessage } from '@/utils/formatMessage'
import { healthStatusLabel, healthStatusToneClass } from '@/utils/healthStatusLabel'
import { scheduleScrollToHash } from '@/utils/hashScroll'
import { readinessFormToPrefill, parseReadinessPrefill } from '@/utils/routePrefill'
import { buildExportPreflight, formatExportPreflightText } from '@/utils/exportPreflight'
import { buildOpsNextSteps } from '@/utils/opsNextSteps'
import { copyText } from '@/utils/clipboard'
import {
  DEPLOY_TEMPLATES,
  deployKitFilename,
  formatDeployKitMarkdown,
  formatDeployTemplateLabel,
  templatesForSignoffField,
} from '@/utils/deployTemplates'
import {
  DEPLOY_DRILL_STEPS,
  deployRunbookDrillFilename,
  formatDeployDrillAreaLabel,
  formatDeployRunbookDrillMarkdown,
  type DeployDrillArea,
} from '@/utils/deployRunbookDrill'
import { textForLocale, type LocaleCode } from '@/utils/localizedText'
import PermissionDenied from '@/components/PermissionDenied.vue'
import VirtualTable from '@/components/VirtualTable.vue'
import EmptyState from '@/components/EmptyState.vue'
import ActionResultBanner from '@/components/ActionResultBanner.vue'
import PartialErrorBanner from '@/components/PartialErrorBanner.vue'
import AdminOpLastChip from '@/components/AdminOpLastChip.vue'
import {
  PRODUCTION_CHECKLIST,
  automatedCoverage,
  requiredChecklist,
} from '@/utils/productionChecklist'
import { EDGE_HTTPS_ACCEPTANCE_STEPS, edgeHttpsProgress } from '@/utils/edgeHttpsAcceptance'
import { BACKUP_SCHEDULE_STEPS, backupScheduleProgress } from '@/utils/backupSchedule'
import {
  completedIds,
  isReadinessDirty,
  loadReadinessState,
  setReadinessFlag,
  setSignoffNote,
  type ReadinessState,
  type SignoffNotes,
} from '@/utils/readinessState'
import {
  localizedSignoffGuideSteps,
  applySignoffExample,
  signoffGuideSummary,
} from '@/utils/signoffGuide'
import {
  assessSignoffCompleteness,
  composeSignoffArchiveNote,
  confirmExportWithMissingSignoff,
  signoffFieldLabel,
  signoffProgressPercent,
  signoffFieldAnchorId,
  formatSignoffMissingClipboard,
  type SignoffNoteField,
} from '@/utils/signoffExport'
import { computeReadinessScore, formatReadinessReasons, readinessLevel } from '@/utils/readinessScore'
import {
  buildReadinessExport,
  parseReadinessImport,
  persistImportedReadiness,
} from '@/utils/readinessIO'
import {
  archiveFilenames,
  buildReadinessArchive,
  formatReadinessArchiveMarkdown,
} from '@/utils/readinessArchive'
import {
  acceptancePackFilenames,
  buildAcceptancePack,
  formatAcceptancePackMarkdown,
} from '@/utils/acceptancePack'
import { clientBuildInfo } from '@/utils/buildInfo'
import { loadOpsActionLog } from '@/utils/opsActionLog'
import {
  ClipboardCheck,
  Download,
  ExternalLink,
  FileCode2,
  HardDrive,
  RefreshCw,
  ShieldCheck,
  Upload,
  Package,
} from 'lucide-vue-next'

interface DoctorCheck { level: string; code: string; message: string }
interface DoctorResponse {
  ok: boolean
  http_tls_enabled?: boolean
  checks?: DoctorCheck[]
  lines?: string[]
  admin_op_busy?: boolean
  op?: string
  started_at_unix?: number
  last?: unknown
}

interface VersionResponse {
  version: string
  commit: string
  built_at: string
}

const { adminOpBusy, adminOpKind, adminOpBusyChecking, refreshAdminOpBusy, applyAdminOpStatus } = useAdminOpBusy()
const readinessBusyRefreshing = ref(false)
async function refreshReadinessBusyOnly() {
  if (readinessBusyRefreshing.value || adminOpBusyChecking.value) return
  readinessBusyRefreshing.value = true
  try {
    await refreshAdminOpBusy()
  } finally {
    readinessBusyRefreshing.value = false
  }
}
const { isAdmin, currentUser } = useAuth()
const { t, locale } = useI18n()
const adminOpBusySummary = inject<ComputedRef<{ busy: boolean; opLabel: string; elapsed: string; detail?: string; lastSummary?: string; lastError?: string; lastOk?: boolean | null }> | undefined>('adminOpBusySummary', undefined)
const readinessAdminLastLabel = computed(() => (adminOpBusySummary?.value?.lastSummary || '').trim())
const readinessAdminLastErrorDetail = computed(() => {
  if (adminOpBusySummary?.value?.lastOk !== false) return ''
  return (adminOpBusySummary?.value?.lastError || '').trim()
})
const readinessAdminLastFailed = computed(() => adminOpBusySummary?.value?.lastOk === false)
const adminOpKindDisplay = computed(() => {
  if (!adminOpBusy.value) return ''
  const key = adminOpKindLabelKey(adminOpKind.value) as MessageKey
  return adminOpBusySummary?.value?.opLabel || t.value(key) || t.value('adminOpKindGeneric')
})
const readinessAdminBusyTitle = computed(() => adminOpBusySummary?.value?.detail || t.value('opsAdminBusy'))
const readinessAdminBusyChipLabel = computed(() => {
  if (!adminOpBusy.value) return t.value('opsAdminBusyChip')
  const kind = adminOpKindDisplay.value
  const base = joinAdminOpChip(t.value('opsAdminBusyChip'), kind)
  const elapsed = adminOpBusySummary?.value?.elapsed
  return elapsed ? `${base} · ${elapsed}` : base
})
const uiLocale = computed<LocaleCode>(() => (locale.value === 'en' ? 'en' : 'zh'))

function formatDoctorLevel(level?: string) {
  return healthStatusLabel(level, uiLocale.value === 'en' ? 'en' : 'zh')
}
const router = useRouter()
const route = useRoute()
const { success, error: notifyError } = useNotify()
const { notifyMaybeAdminBusy } = useNotifyAdminBusy()

function notifyExportFail(message: string) {
  const msg = String(message || '').trim() || t.value('failed')
  const err = isAdminHeavyBusyMessage(msg)
    ? { code: 'resource_exhausted', status: 429, message: msg }
    : { message: msg }
  notifyMaybeAdminBusy(msg, err, { treatLocalBusy: true })
}
const {
  exportJob,
  exportBusy,
  cancelExport,
  resetExport,
  retryLastExport,
  canRetryExport,
  runJSONExport,
  runTextExport,
  runBundleExport,
} = useExportJob()

const state = ref<ReadinessState>(loadReadinessState())
const readinessBaseline = ref(snapshotForm(state.value))
const formDirty = computed(() => isReadinessDirty(readinessBaseline.value, state.value))

function markReadinessClean() {
  readinessBaseline.value = snapshotForm(state.value)
}

function onBeforeUnload(e: BeforeUnloadEvent) {
  if (!formDirty.value) return
  e.preventDefault()
  e.returnValue = ''
}

let unregisterDirty: (() => void) | null = null

const DOC_ROW_HEIGHT = 40
const DOC_LIST_HEIGHT = 280
const doctor = ref<DoctorResponse | null>(null)
const doctorError = ref('')
const versionError = ref('')
const loadingDoctor = ref(false)
const loadingVersion = ref(false)
const actionMsg = ref('')
const actionKind = ref<'ok' | 'error' | 'info'>('info')
const importMerge = ref(true)
const fileInput = ref<HTMLInputElement | null>(null)
const serverVersion = ref<VersionResponse | null>(null)

const productionDone = computed(() => completedIds(state.value.production))
const edgeDone = computed(() => completedIds(state.value.edgeHttps))
const scheduleDone = computed(() => completedIds(state.value.backupSchedule))

const productionCoverage = computed(() => automatedCoverage())
const requiredItems = computed(() => requiredChecklist())
const requiredDoneCount = computed(
  () => requiredItems.value.filter((i) => !!state.value.production[i.id]).length,
)
const edgeStats = computed(() => edgeHttpsProgress(edgeDone.value))
const scheduleStats = computed(() => backupScheduleProgress(scheduleDone.value))
const signoffCompleteness = computed(() => assessSignoffCompleteness(state.value.signoffNotes))
const signoffMissingLabels = computed(() =>
  signoffCompleteness.value.missing.map((f) => signoffFieldLabel(f, uiLocale.value)),
)
const signoffProgress = computed(() => signoffProgressPercent(state.value.signoffNotes))

const doctorWarns = computed(() => (doctor.value?.checks ?? []).filter((c) => c.level === 'warn'))
const doctorOKs = computed(() => (doctor.value?.checks ?? []).filter((c) => c.level === 'ok'))

const scoreBreakdown = computed(() => {
  const requiredRatio =
    requiredItems.value.length === 0 ? 1 : requiredDoneCount.value / requiredItems.value.length
  const edgeRatio =
    edgeStats.value.requiredTotal === 0
      ? 1
      : edgeStats.value.requiredDone / edgeStats.value.requiredTotal
  const scheduleRatio =
    scheduleStats.value.requiredTotal === 0
      ? 1
      : scheduleStats.value.requiredDone / scheduleStats.value.requiredTotal
  return computeReadinessScore({
    requiredChecklistRatio: requiredRatio,
    edgeHttpsRequiredRatio: edgeRatio,
    backupScheduleRequiredRatio: scheduleRatio,
    doctorLoaded: doctor.value != null,
    doctorOk: doctor.value?.ok,
    doctorWarnCount: doctorWarns.value.length,
    httpTlsEnabled: doctor.value == null ? null : !!doctor.value.http_tls_enabled,
    adminOpBusy: adminOpBusy.value,
    adminOpLastFailed: readinessAdminLastFailed.value,
  })
})
const scoreReasonsLabel = computed(() =>
  formatReadinessReasons(scoreBreakdown.value.reasons, uiLocale.value),
)
const readinessScore = computed(() => scoreBreakdown.value.total)
const scoreLevel = computed(() => readinessLevel(readinessScore.value))

const exportPreflight = computed(() => {
  const requiredRatio =
    requiredItems.value.length === 0 ? 1 : requiredDoneCount.value / requiredItems.value.length
  const edgeRatio =
    edgeStats.value.requiredTotal === 0
      ? 1
      : edgeStats.value.requiredDone / edgeStats.value.requiredTotal
  const scheduleRatio =
    scheduleStats.value.requiredTotal === 0
      ? 1
      : scheduleStats.value.requiredDone / scheduleStats.value.requiredTotal
  return buildExportPreflight({
    locale: uiLocale.value,
    requiredChecklistRatio: requiredRatio,
    edgeHttpsRequiredRatio: edgeRatio,
    backupScheduleRequiredRatio: scheduleRatio,
    doctorLoaded: doctor.value != null,
    doctorOk: doctor.value?.ok,
    doctorWarnCount: doctorWarns.value.length,
    httpTlsEnabled: doctor.value == null ? null : !!doctor.value.http_tls_enabled,
    adminOpBusy: adminOpBusy.value,
    adminOpKindLabel: adminOpKindDisplay.value || '',
    signoffNotes: state.value.signoffNotes,
    deployKitReviewed: !!state.value.deployKit?.reviewed,
  })
})
const readinessNextSteps = computed(() =>
  buildOpsNextSteps({
    locale: uiLocale.value,
    preflight: exportPreflight.value,
    signoffNotes: state.value.signoffNotes,
    limit: 4,
  }),
)

function readinessLevelLabel(level: string): string {
  if (level === 'good') return t.value('readinessLevelGood')
  if (level === 'warn') return t.value('readinessLevelWarn')
  return t.value('readinessLevelBad')
}


function flash(kind: 'ok' | 'error' | 'info', message: string) {
  actionKind.value = kind
  actionMsg.value = message
}

function toggle(
  section: 'production' | 'edgeHttps' | 'backupSchedule' | 'deployKit',
  id: string,
  checked: boolean,
) {
  state.value = setReadinessFlag(section, id, checked)
}

function saveSignoff(field: keyof SignoffNotes, value: string) {
  state.value = setSignoffNote(field, value)
}

function isSignoffFieldFilled(field: SignoffNoteField): boolean {
  return !signoffCompleteness.value.missing.includes(field)
}

function focusSignoffField(field: SignoffNoteField) {
  const id = signoffFieldAnchorId(field)
  const el = typeof document !== 'undefined' ? document.getElementById(id) : null
  if (el instanceof HTMLElement) {
    el.scrollIntoView({ behavior: 'smooth', block: 'center' })
    el.focus()
  }
}

function jumpDeployTemplate(tplId: string) {
  jumpPreflight(`#deploy-tpl-${tplId}`)
}

function jumpRelatedSignoff(field?: SignoffNoteField | null) {
  if (!field) return
  jumpPreflight('#signoff-notes')
  // next frame focus field
  if (typeof window !== 'undefined') {
    window.setTimeout(() => focusSignoffField(field), 50)
  } else {
    focusSignoffField(field)
  }
}

function relatedTemplatesFor(field: SignoffNoteField) {
  return templatesForSignoffField(field)
}

function guideStepsFor(field: SignoffNoteField) {
  return localizedSignoffGuideSteps(field, uiLocale.value)
}

function guideSummaryFor(field: SignoffNoteField) {
  return signoffGuideSummary(field, uiLocale.value)
}

function applyExampleToSignoff(field: SignoffNoteField) {
  const current = state.value.signoffNotes?.[field] ?? ''
  const next = applySignoffExample(current, field, uiLocale.value)
  if (next == null) {
    notifyError(t.value('readinessSignoffExampleBlocked'))
    return
  }
  saveSignoff(field, next)
  success(t.value('readinessSignoffExampleApplied'))
  focusSignoffField(field)
}

async function copySignoffMissing() {
  const text = formatSignoffMissingClipboard(signoffCompleteness.value.missing, uiLocale.value)
  const r = await copyText(text)
  if (r.ok) {
    success(t.value('readinessSignoffCopied'))
    flash('ok', t.value('readinessSignoffCopied'))
  } else {
    notifyError(t.value('readinessCopyFailed'))
    flash('error', t.value('readinessCopyFailed'))
  }
}

async function loadServerVersion() {
  loadingVersion.value = true
  try {
    serverVersion.value = await apiGet<VersionResponse>('/api/v1/admin/version')
    versionError.value = ''
  } catch (e) {
    const msg = formatCaughtError(e)
    // soft-keep：已有版本信息时保留快照
    if (serverVersion.value) {
      versionError.value = msg
      notifyMaybeAdminBusy(msg, e, { treatLocalBusy: true })
    } else {
      serverVersion.value = null
      versionError.value = msg
      notifyMaybeAdminBusy(msg, e)
    }
  } finally {
    loadingVersion.value = false
  }
}

async function loadDoctor() {
  loadingDoctor.value = true
  try {
    doctor.value = await apiGet<DoctorResponse>('/api/v1/admin/doctor')
    applyAdminOpStatus(parseAdminOpStatusPayload(doctor.value))
    doctorError.value = ''
  } catch (e) {
    const msg = formatCaughtError(e)
    // soft-keep：已有 doctor 快照时保留
    if (doctor.value) {
      doctorError.value = msg
      notifyMaybeAdminBusy(msg, e, { treatLocalBusy: true })
    } else {
      doctor.value = null
      doctorError.value = msg
      notifyMaybeAdminBusy(msg, e)
    }
  } finally {
    loadingDoctor.value = false
  }
}

function go(path: string) {
  void router.push(path)
}

async function exportState() {
  if (exportBusy.value) return
  const payload = buildReadinessExport(state.value)
  const stamp = new Date().toISOString().replace(/[:.]/g, '-').slice(0, 19)
  const outcome = await runJSONExport({
    label: 'JSON',
    filename: `mts-readiness-${stamp}.json`,
    total: 1,
    build: async ({ isCancelled, progress }) => {
      progress(0, 1)
      if (isCancelled()) return null
      progress(1, 1)
      return payload
    },
  })
  if (outcome === 'done') flash('ok', t.value('readinessExportOk'))
  else if (outcome === 'cancelled') flash('info', t.value('exportCancelledToast'))
  else if (outcome === 'error') {
    const m = exportJob.value.error || t.value('failed')
    flash('error', m)
    notifyExportFail(m)
  }
}

function openImport() {
  fileInput.value?.click()
}

async function onImportFile(ev: Event) {
  const input = ev.target as HTMLInputElement
  const file = input.files?.[0]
  input.value = ''
  if (!file) return
  try {
    const text = await file.text()
    const parsed = parseReadinessImport(JSON.parse(text) as unknown)
    if (!parsed.ok) {
      flash('error', `${t.value('readinessImportFail')}: ${parsed.error}`)
      return
    }
    state.value = persistImportedReadiness(parsed.state, { merge: importMerge.value })
    markReadinessClean()
    flash('ok', t.value('readinessImportOk'))
  } catch (e) {
    flash('error', `${t.value('readinessImportFail')}: ${formatCaughtError(e)}`)
    notifyMaybeAdminBusy(`${t.value('readinessImportFail')}: ${formatCaughtError(e)}`, e)
  }
}

function doctorArchiveSummary() {
  return {
    loaded: doctor.value != null,
    ok: doctor.value?.ok,
    http_tls_enabled: doctor.value == null ? null : !!doctor.value.http_tls_enabled,
    warn_count: doctorWarns.value.length,
    checks: doctor.value?.checks,
    error: doctorError.value || undefined,
  }
}

function buildArchiveNote(): string {
  return composeSignoffArchiveNote(state.value.signoffNotes, { locale: uiLocale.value })
}

function confirmMissingSignoffExport(): boolean {
  return confirmExportWithMissingSignoff(signoffCompleteness.value, uiLocale.value)
}

async function downloadArchive() {
  if (exportBusy.value) return
  if (!confirmMissingSignoffExport()) {
    flash('info', t.value('readinessExportCancelled'))
    return
  }
  const archive = buildReadinessArchive({
    operator: currentUser.value || 'admin',
    note: buildArchiveNote(),
    state: state.value,
    score: scoreBreakdown.value,
    doctor: doctorArchiveSummary(),
    locale: uiLocale.value,
  })
  const names = archiveFilenames()
  const md = formatReadinessArchiveMarkdown(archive)
  const outcome = await runBundleExport({
    label: t.value('readinessArchive') || 'Archive',
    total: 2,
    build: async ({ isCancelled, progress }) => {
      progress(0, 2)
      if (isCancelled()) return null
      progress(1, 2)
      await new Promise((r) => setTimeout(r, 0))
      if (isCancelled()) return null
      progress(2, 2)
      return [
        { kind: 'json', filename: names.json, payload: archive },
        { kind: 'text', filename: names.md, text: md, mime: 'text/markdown' },
      ]
    },
  })
  const preflightToast = formatMessage(t.value('readinessExportPreflightToast'), {
    warn: exportPreflight.value.warnCount,
    info: exportPreflight.value.infoCount,
    ok: exportPreflight.value.okCount,
  })
  if (outcome === 'cancelled') {
    flash('info', t.value('exportCancelledToast'))
    return
  }
  if (outcome === 'error') {
    const m = exportJob.value.error || t.value('failed')
    flash('error', m)
    notifyExportFail(m)
    return
  }
  if (!signoffCompleteness.value.complete) {
    flash('info', `${t.value('readinessArchiveOkWithGaps')} · ${preflightToast}`)
    success(`${t.value('readinessArchiveOkWithGaps')} · ${preflightToast}`)
  } else {
    flash('ok', `${t.value('readinessArchiveOk')} · ${preflightToast}`)
    success(`${t.value('readinessArchiveOk')} · ${preflightToast}`)
  }
}

async function downloadAcceptancePack() {
  if (exportBusy.value) return
  if (!confirmMissingSignoffExport()) {
    flash('info', t.value('readinessExportCancelled'))
    return
  }
  const archive = buildReadinessArchive({
    operator: currentUser.value || 'admin',
    note: buildArchiveNote(),
    state: state.value,
    score: scoreBreakdown.value,
    doctor: doctorArchiveSummary(),
    locale: uiLocale.value,
  })
  const pack = buildAcceptancePack({
    archive,
    client: clientBuildInfo(),
    server: serverVersion.value,
    opsActions: loadOpsActionLog(),
    operator: currentUser.value || 'admin',
    note: buildArchiveNote(),
    locale: uiLocale.value,
  })
  const names = acceptancePackFilenames()
  const md = formatAcceptancePackMarkdown(pack)
  const outcome = await runBundleExport({
    label: t.value('readinessAcceptancePack') || 'Acceptance pack',
    total: 2,
    build: async ({ isCancelled, progress }) => {
      progress(0, 2)
      if (isCancelled()) return null
      progress(1, 2)
      await new Promise((r) => setTimeout(r, 0))
      if (isCancelled()) return null
      progress(2, 2)
      return [
        { kind: 'json', filename: names.json, payload: pack },
        { kind: 'text', filename: names.md, text: md, mime: 'text/markdown' },
      ]
    },
  })
  const preflightToast = formatMessage(t.value('readinessExportPreflightToast'), {
    warn: exportPreflight.value.warnCount,
    info: exportPreflight.value.infoCount,
    ok: exportPreflight.value.okCount,
  })
  if (outcome === 'cancelled') {
    flash('info', t.value('exportCancelledToast'))
    return
  }
  if (outcome === 'error') {
    const m = exportJob.value.error || t.value('failed')
    flash('error', m)
    notifyExportFail(m)
    return
  }
  if (!signoffCompleteness.value.complete) {
    flash('info', `${t.value('readinessAcceptancePackOkWithGaps')} · ${preflightToast}`)
    success(`${t.value('readinessAcceptancePackOkWithGaps')} · ${preflightToast}`)
  } else {
    flash('ok', `${t.value('readinessAcceptancePackOk')} · ${preflightToast}`)
    success(`${t.value('readinessAcceptancePackOk')} · ${preflightToast}`)
  }
}


const quickActions = [
  { id: 'data-restore', labelKey: 'readinessGoDataRestore', path: '/storage#data-restore' },
  { id: 'backup-drill', labelKey: 'readinessGoBackupDrill', path: '/storage#backup-drill' },
  { id: 'edge-https', labelKey: 'readinessGoEdgeHttps', path: '/storage#edge-https' },
] as const

const deployTemplates = DEPLOY_TEMPLATES
const backupScriptHint = deployTemplates.find((x) => x.id === 'backup-env')?.body
  ? `${deployTemplates.find((x) => x.id === 'backup-env')!.body.trim()}
./scripts/mts-backup.sh --dry-run
./scripts/mts-backup.sh`
  : `export MTS_BASE_URL='https://mts.example.com'
export MTS_ADMIN_TOKEN='***'
./scripts/mts-backup.sh`

function markDeployKitHint(id: 'reviewed' | 'downloaded' | 'copied') {
  state.value = setReadinessFlag('deployKit', id, true)
}

async function copyDeployBody(body: string) {
  const r = await copyText(body)
  if (r.ok) {
    markDeployKitHint('copied')
    markDeployKitHint('reviewed')
    success(t.value('readinessCopied'))
  } else {
    notifyError(t.value('readinessCopyFailed'))
  }
}

async function downloadDeployKit() {
  if (exportBusy.value) return
  const md = formatDeployKitMarkdown(uiLocale.value)
  const outcome = await runTextExport({
    label: 'Markdown',
    filename: deployKitFilename(),
    mime: 'text/markdown',
    total: 1,
    build: async ({ isCancelled, progress }) => {
      progress(0, 1)
      if (isCancelled()) return null
      progress(1, 1)
      return md
    },
  })
  if (outcome === 'done') {
    markDeployKitHint('downloaded')
    markDeployKitHint('reviewed')
    success(t.value('readinessDeployKitDownloaded'))
    flash('ok', t.value('readinessDeployKitDownloaded'))
  } else if (outcome === 'cancelled') flash('info', t.value('exportCancelledToast'))
  else if (outcome === 'error') {
    const m = exportJob.value.error || t.value('failed')
    flash('error', m)
    notifyExportFail(m)
  }
}

const deployDrillSteps = DEPLOY_DRILL_STEPS
const deployDrillAreas: DeployDrillArea[] = ['edge_https', 'scheduler', 'offsite_backup']

function drillStepsForArea(area: DeployDrillArea) {
  return deployDrillSteps.filter((s) => s.area === area)
}

function areaLabel(area: DeployDrillArea) {
  return formatDeployDrillAreaLabel(area, uiLocale.value)
}

async function downloadDeployDrill() {
  if (exportBusy.value) return
  const md = formatDeployRunbookDrillMarkdown(uiLocale.value)
  const outcome = await runTextExport({
    label: 'Markdown',
    filename: deployRunbookDrillFilename(),
    mime: 'text/markdown',
    total: 1,
    build: async ({ isCancelled, progress }) => {
      progress(0, 1)
      if (isCancelled()) return null
      progress(1, 1)
      return md
    },
  })
  if (outcome === 'done') {
    markDeployKitHint('reviewed')
    success(t.value('readinessDeployDrillDownloaded'))
    flash('ok', t.value('readinessDeployDrillDownloaded'))
  } else if (outcome === 'cancelled') flash('info', t.value('exportCancelledToast'))
  else if (outcome === 'error') {
    const m = exportJob.value.error || t.value('failed')
    flash('error', m)
    notifyExportFail(m)
  }
}

async function copyDeployDrill() {
  const md = formatDeployRunbookDrillMarkdown(uiLocale.value)
  const r = await copyText(md)
  if (r.ok) {
    markDeployKitHint('copied')
    markDeployKitHint('reviewed')
    success(t.value('readinessDeployDrillCopied'))
  } else {
    notifyError(t.value('readinessCopyFailed'))
  }
}


function scrollToCurrentHash() {
  if (typeof window === 'undefined') return
  scheduleScrollToHash(window.location.hash || route.hash)
}

function currentReadinessSection(): string {
  const h = (route.hash || (typeof window !== 'undefined' ? window.location.hash : '') || '').replace(/^#/, '')
  const known = new Set(['export-preflight', 'deploy-kit', 'signoff-notes', 'deploy-runbook-drill', 'readiness-action'])
  if (known.has(h)) return h
  const pre = parseReadinessPrefill(route.query as Record<string, unknown>, route.hash)
  return pre.section || 'export-preflight'
}

async function copyReadinessShareLink() {
  const path = readinessFormToPrefill({ section: currentReadinessSection() })
  const url = `${window.location.origin}${path}`
  const res = await copyText(url)
  if (res.ok) success(t.value('readinessShareCopied'))
  else notifyError(res.error || t.value('failed'))
}

function jumpPreflight(target?: string) {
  if (!target) return
  if (target.startsWith('/')) {
    void router.push(target)
    return
  }
  const hash = target.startsWith('#') ? target : `#${target}`
  if (typeof window !== 'undefined') {
    if (window.location.hash === hash) {
      scheduleScrollToHash(hash)
    } else {
      window.location.hash = hash
    }
  }
}

async function copyPreflightSummary() {
  const text = formatExportPreflightText(exportPreflight.value, uiLocale.value)
  const r = await copyText(text)
  if (r.ok) {
    success(t.value('readinessPreflightCopied'))
    flash('ok', t.value('readinessPreflightCopied'))
  } else {
    notifyError(t.value('readinessPreflightCopyFailed'))
    flash('error', t.value('readinessPreflightCopyFailed'))
  }
}

onMounted(() => {
  unregisterDirty = registerDirtyChecker('readiness', () => formDirty.value, 'local')
  if (typeof window !== 'undefined') {
    window.addEventListener('beforeunload', onBeforeUnload)
  }
  markReadinessClean()
  if (isAdmin.value) {
    void loadDoctor()
    void loadServerVersion()
  }
  scrollToCurrentHash()
  if (typeof window !== 'undefined') {
    window.addEventListener('hashchange', scrollToCurrentHash)
  }
})

onBeforeUnmount(() => {
  unregisterDirty?.()
  unregisterDirty = null
  if (typeof window !== 'undefined') {
    window.removeEventListener('beforeunload', onBeforeUnload)
    window.removeEventListener('hashchange', scrollToCurrentHash)
  }
})

watch(
  () => route.hash,
  () => {
    scrollToCurrentHash()
  },
)
</script>

<template>
  <PermissionDenied v-if="!isAdmin" />
  <div v-else class="space-y-6" data-testid="readiness-page">
    <div class="flex flex-wrap items-start justify-between gap-3">
      <div>
        <h1 class="mts-title flex items-center gap-2">
          <ClipboardCheck class="h-5 w-5" />
          {{ t('readinessTitle') }}
          <span
            v-if="formDirty"
            class="rounded-full bg-amber-100 px-2 py-0.5 text-[11px] font-medium text-amber-800 dark:bg-amber-950/50 dark:text-amber-200"
            data-testid="readiness-dirty-badge"
            :title="t('readinessDirtyTitle')"
          >{{ t('readinessDirtyBadge') }}</span>
        </h1>
        <p class="text-xs mts-muted">{{ t('readinessDesc') }}</p>
        <p v-if="state.updatedAt" class="mt-1 text-[11px] mts-muted">
          {{ t('readinessUpdatedAt') }} {{ new Date(state.updatedAt).toLocaleString() }}
        </p>
      </div>
      <div class="flex flex-wrap items-center gap-2">
        <button type="button" class="mts-btn" data-testid="readiness-share-link" @click="copyReadinessShareLink">
          {{ t('readinessShareLink') }}
        </button>
        <ExportJobBanner class="w-full basis-full" :job="exportJob" :retryable="canRetryExport" @cancel="cancelExport" @retry="retryLastExport" @dismiss="resetExport" />
        <button type="button" class="mts-btn" data-testid="readiness-export" :disabled="exportBusy" @click="exportState">
          <Download class="h-3.5 w-3.5" />
          {{ t('readinessExport') }}
        </button>
        <button type="button" class="mts-btn" data-testid="readiness-import" @click="openImport">
          <Upload class="h-3.5 w-3.5" />
          {{ t('readinessImport') }}
        </button>
        <button type="button" class="mts-btn" data-testid="readiness-archive" :disabled="exportBusy" @click="downloadArchive">
          <FileCode2 class="h-3.5 w-3.5" />
          {{ t('readinessArchive') }}
        </button>
        <button type="button" class="mts-btn" data-testid="readiness-acceptance-pack" :disabled="exportBusy" @click="downloadAcceptancePack">
          <Package class="h-3.5 w-3.5" />
          {{ t('readinessAcceptancePack') }}
        </button>
        <button class="mts-btn" :disabled="loadingDoctor" @click="loadDoctor">
          <RefreshCw class="h-3.5 w-3.5" :class="loadingDoctor ? 'animate-spin' : ''" />
          {{ t('refresh') }}
        </button>
        <button class="mts-btn" @click="go('/storage')">
          <ExternalLink class="h-3.5 w-3.5" />
          {{ t('storage') }}
        </button>
      </div>
    </div>

    <input
      ref="fileInput"
      type="file"
      accept="application/json,.json"
      class="hidden"
      data-testid="readiness-import-file"
      @change="onImportFile"
    />

    <div class="flex flex-wrap items-center gap-3 text-xs mts-muted">
      <label class="inline-flex items-center gap-1">
        <input v-model="importMerge" type="checkbox" data-testid="readiness-import-merge" />
        {{ t('readinessImportMerge') }}
      </label>
      <span>{{ t('readinessArchiveHint') }}</span>
    </div>

    <div id="export-preflight" class="mts-card p-4 scroll-mt-20" data-testid="readiness-export-preflight">
      <div class="mb-2 flex flex-wrap items-center justify-between gap-2">
        <h2 class="text-sm font-semibold">{{ t('readinessExportPreflight') }}</h2>
        <div class="flex flex-wrap items-center gap-2">
          <span class="text-xs mts-muted" data-testid="readiness-preflight-summary">
            ok={{ exportPreflight.okCount }} · warn={{ exportPreflight.warnCount }} · info={{ exportPreflight.infoCount }}
          </span>
          <button
            type="button"
            class="mts-btn !px-2 !py-0.5 text-[11px]"
            data-testid="readiness-copy-preflight"
            @click="copyPreflightSummary"
          >
            {{ t('readinessCopyPreflight') }}
          </button>
        </div>
      </div>
      <p class="mb-2 text-xs mts-muted">{{ t('readinessExportPreflightHint') }}</p>
      <ul class="space-y-1.5">
        <li
          v-for="item in exportPreflight.items"
          :key="item.id"
          class="flex items-start gap-2 rounded-md border border-slate-100 px-2 py-1.5 text-xs dark:border-slate-800"
          :data-testid="`preflight-item-${item.id}`"
        >
          <span
            class="mt-0.5 inline-block min-w-[2.5rem] rounded px-1 py-0.5 text-center text-[10px] font-semibold uppercase"
            :class="item.level === 'ok'
              ? 'bg-emerald-100 text-emerald-800 dark:bg-emerald-900/40 dark:text-emerald-200'
              : item.level === 'warn'
                ? 'bg-amber-100 text-amber-900 dark:bg-amber-900/40 dark:text-amber-100'
                : 'bg-slate-100 text-slate-700 dark:bg-slate-800 dark:text-slate-200'"
          >{{ item.level }}</span>
          <span class="min-w-0 flex-1 text-slate-700 dark:text-slate-200">{{ item.message }}</span>
          <button
            v-if="item.target"
            type="button"
            class="mts-btn shrink-0 !px-2 !py-0.5 text-[11px]"
            :data-testid="`preflight-jump-${item.id}`"
            @click="jumpPreflight(item.target)"
          >
            {{ t(item.actionKey === 'preflightJumpStorage' ? 'preflightJumpStorage' : 'preflightJumpLocal') }}
          </button>
        </li>
      </ul>
    </div>

    <div
      class="mts-card p-4"
      data-testid="readiness-next-steps"
    >
      <div class="mb-2 flex flex-wrap items-center justify-between gap-2">
        <h2 class="text-sm font-semibold">{{ t('readinessNextSteps') }}</h2>
        <span class="text-xs mts-muted">{{ t('readinessNextStepsHint') }}</span>
      </div>
      <ul class="space-y-1.5">
        <li
          v-for="step in readinessNextSteps"
          :key="step.id"
          class="flex items-start gap-2 rounded-md border border-slate-100 px-2 py-1.5 text-xs dark:border-slate-800"
          :data-testid="`next-step-${step.id}`"
        >
          <span class="min-w-0 flex-1 text-slate-700 dark:text-slate-200">{{ step.message }}</span>
          <button
            v-if="step.target"
            type="button"
            class="mts-btn shrink-0 !px-2 !py-0.5 text-[11px]"
            :data-testid="`next-jump-${step.id}`"
            @click="jumpPreflight(step.target)"
          >
            {{ t(step.actionKey === 'preflightJumpStorage' ? 'preflightJumpStorage' : 'preflightJumpLocal') }}
          </button>
        </li>
      </ul>
    </div>

    <ActionResultBanner
      v-if="doctorError && !doctor"
      kind="error"
      :message="doctorError"
      retryable
      data-testid="readiness-doctor-error"
      @retry="loadDoctor"
      @dismiss="doctorError = ''"
    />
    <PartialErrorBanner
      v-else-if="doctorError && doctor"
      :message="`${t('readinessDoctorRefreshFailed')}：${doctorError}`"
      test-id="readiness-doctor-refresh-error"
      @retry="loadDoctor"
      @dismiss="doctorError = ''"
    />
    <PartialErrorBanner
      v-if="versionError"
      :message="`${t('readinessVersionLoadFailed')}：${versionError}`"
      test-id="readiness-version-error"
      @retry="loadServerVersion"
      @dismiss="versionError = ''"
    />
    <div
      v-if="doctorError || versionError"
      class="flex flex-wrap items-center gap-2 rounded-lg border border-amber-200 bg-amber-50/80 px-3 py-2 dark:border-amber-900/50 dark:bg-amber-950/30"
      data-testid="readiness-partial-sections"
    >
      <span class="text-xs font-medium text-amber-900 dark:text-amber-100">{{ t('overviewPartialRetryHint') }}</span>
      <button
        v-if="doctorError"
        type="button"
        class="mts-btn text-xs"
        data-testid="readiness-partial-retry-doctor"
        :disabled="loadingDoctor"
        @click="loadDoctor"
      >{{ t('overviewPartialDoctor') }} · {{ loadingDoctor ? t('loading') : t('retry') }}</button>
      <button
        v-if="versionError"
        type="button"
        class="mts-btn text-xs"
        data-testid="readiness-partial-retry-version"
        :disabled="loadingVersion"
        @click="loadServerVersion"
      >{{ t('overviewPartialVersion') }} · {{ loadingVersion ? t('loading') : t('retry') }}</button>
    </div>
    <div id="readiness-action" class="scroll-mt-20" data-testid="readiness-action-anchor">
      <ActionResultBanner
        v-if="actionMsg"
        :kind="actionKind"
        :message="actionMsg"
        data-testid="readiness-action"
        @dismiss="actionMsg = ''"
      />
    </div>

    <div class="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
      <div class="mts-card p-4">
        <p class="text-xs mts-muted">{{ t('readinessScore') }}</p>
        <div
          v-if="adminOpBusy || readinessAdminLastLabel"
          class="mt-1 flex flex-wrap items-center gap-2"
          data-testid="readiness-admin-busy-row"
        >
          <span
            v-if="adminOpBusy"
            class="inline-flex rounded-full bg-sky-50 px-2 py-0.5 text-[11px] font-medium text-sky-900 dark:bg-sky-950/40 dark:text-sky-100"
            data-testid="readiness-admin-busy"
            :title="readinessAdminBusyTitle"
          >{{ readinessAdminBusyChipLabel }}</span>
          <AdminOpLastChip
            v-else-if="readinessAdminLastLabel"
            :label="readinessAdminLastLabel"
            :last-ok="adminOpBusySummary?.lastOk"
            :last-error="readinessAdminLastErrorDetail"
            test-id="readiness-admin-last"
            error-test-id="readiness-admin-last-error"
          />

          <button
            v-if="adminOpBusy"
            type="button"
            class="mts-btn text-[11px] !px-2 !py-0.5"
            data-testid="readiness-admin-busy-refresh"
            :disabled="readinessBusyRefreshing || adminOpBusyChecking"
            :aria-busy="readinessBusyRefreshing || adminOpBusyChecking ? 'true' : undefined"
            :title="t('adminOpBusyRefresh')"
            @click="refreshReadinessBusyOnly"
          >
            <RefreshCw
              class="h-3 w-3"
              :class="(readinessBusyRefreshing || adminOpBusyChecking) ? 'animate-spin' : ''"
            />
            {{ t('adminOpBusyRefresh') }}
          </button>
        </div>
        <p
          class="mt-1 text-3xl font-semibold"
          :class="scoreLevel === 'good' ? 'text-green-600' : scoreLevel === 'warn' ? 'text-amber-600' : 'text-red-600'"
          data-testid="readiness-score"
        >
          {{ readinessScore }}%
          <span class="ml-2 text-xs font-medium mts-muted" data-testid="readiness-score-level">{{ readinessLevelLabel(scoreLevel) }}</span>
        </p>
        <p class="mt-2 text-[11px] mts-muted">
          {{ t('readinessScoreBreakdown') }}:
          {{ t('readinessRequiredChecklist') }} {{ scoreBreakdown.checklist }}% ·
          {{ t('readinessEdgeHttps') }} {{ scoreBreakdown.edgeHttps }}% ·
          {{ t('readinessBackupSchedule') }} {{ scoreBreakdown.backupSchedule }}% ·
          {{ t('readinessScoreDoctor') }} {{ scoreBreakdown.doctor }}%
        </p>
        <p
          v-if="scoreBreakdown.reasons.length"
          class="mt-1 text-[11px] text-amber-700 dark:text-amber-200"
          data-testid="readiness-score-reasons"
        >
          {{ t('readinessScoreReasons') }}: {{ scoreReasonsLabel || scoreBreakdown.reasons.join(', ') }}
        </p>
      </div>
      <div class="mts-card p-4">
        <p class="text-xs mts-muted">{{ t('readinessRequiredChecklist') }}</p>
        <p class="mt-1 text-lg font-semibold">{{ requiredDoneCount }}/{{ requiredItems.length }}</p>
        <p class="text-[11px] mts-muted">
          {{ formatMessage(t('readinessAutoCoverage'), { done: productionCoverage.automated, total: productionCoverage.total }) }}
        </p>
      </div>
      <div class="mts-card p-4">
        <p class="text-xs mts-muted">{{ t('readinessEdgeHttps') }}</p>
        <p class="mt-1 text-lg font-semibold">{{ edgeStats.requiredDone }}/{{ edgeStats.requiredTotal }}</p>
      </div>
      <div class="mts-card p-4">
        <p class="text-xs mts-muted">{{ t('readinessBackupSchedule') }}</p>
        <p class="mt-1 text-lg font-semibold">{{ scheduleStats.requiredDone }}/{{ scheduleStats.requiredTotal }}</p>
      </div>
    </div>

    <div class="mts-card p-4">
      <div class="mb-3 flex flex-wrap items-center justify-between gap-2">
        <h2 class="flex items-center gap-2 text-sm font-semibold text-slate-800 dark:text-slate-100">
          <HardDrive class="h-4 w-4" />
          {{ t('readinessQuickActions') }}
        </h2>
      </div>
      <div class="flex flex-wrap gap-2">
        <button
          v-for="a in quickActions"
          :key="a.id"
          type="button"
          class="mts-btn"
          @click="go(a.path)"
        >
          <ExternalLink class="h-3.5 w-3.5" />
          {{ t(a.labelKey) }}
        </button>
      </div>
      <div id="deploy-kit" class="mt-4 space-y-3 scroll-mt-20" data-testid="readiness-deploy-kit">
        <div class="flex flex-wrap items-start justify-between gap-2">
          <div>
            <p class="mb-1 flex items-center gap-1 text-xs font-medium text-slate-700 dark:text-slate-200">
              <FileCode2 class="h-3.5 w-3.5" />
              {{ t('readinessDeployKit') }}
            </p>
            <p class="text-xs mts-muted">{{ t('readinessDeployKitHint') }}</p>
            <p class="mt-1 text-[11px] text-amber-700 dark:text-amber-200">{{ t('readinessDeployManualNote') }}</p>
            <div class="mt-3 space-y-1.5 rounded-lg border border-dashed border-slate-300 p-2 dark:border-slate-600" data-testid="deploy-kit-local-hints">
              <p class="text-[11px] font-medium text-slate-700 dark:text-slate-200">{{ t('readinessDeployKitLocalHints') }}</p>
              <p class="text-[11px] mts-muted">{{ t('readinessDeployKitHintsNote') }}</p>
              <label class="flex items-center gap-2 text-xs text-slate-700 dark:text-slate-200">
                <input
                  type="checkbox"
                  data-testid="deploy-kit-hint-reviewed"
                  :checked="!!state.deployKit?.reviewed"
                  @change="toggle('deployKit', 'reviewed', ($event.target as HTMLInputElement).checked)"
                />
                {{ t('readinessDeployKitReviewed') }}
              </label>
              <label class="flex items-center gap-2 text-xs text-slate-700 dark:text-slate-200">
                <input
                  type="checkbox"
                  data-testid="deploy-kit-hint-downloaded"
                  :checked="!!state.deployKit?.downloaded"
                  @change="toggle('deployKit', 'downloaded', ($event.target as HTMLInputElement).checked)"
                />
                {{ t('readinessDeployKitDownloadedLocal') }}
              </label>
              <label class="flex items-center gap-2 text-xs text-slate-700 dark:text-slate-200">
                <input
                  type="checkbox"
                  data-testid="deploy-kit-hint-copied"
                  :checked="!!state.deployKit?.copied"
                  @change="toggle('deployKit', 'copied', ($event.target as HTMLInputElement).checked)"
                />
                {{ t('readinessDeployKitCopied') }}
              </label>
            </div>
            <div
              class="mt-3 space-y-2 rounded-lg border border-amber-200 bg-amber-50/80 p-3 dark:border-amber-900 dark:bg-amber-950/30"
              data-testid="deploy-acceptance-boundary"
            >
              <p class="text-xs font-semibold text-amber-900 dark:text-amber-100">{{ t('readinessDeployAcceptanceTitle') }}</p>
              <p class="text-[11px] text-amber-900/90 dark:text-amber-100/90">{{ t('readinessDeployAcceptanceBody') }}</p>
              <p class="text-[11px] font-medium text-slate-700 dark:text-slate-200">{{ t('readinessDeployNextChecklist') }}</p>
              <ol class="list-decimal space-y-1 pl-4 text-[11px] text-slate-700 dark:text-slate-200">
                <li data-testid="deploy-accept-step-1">{{ t('readinessDeployStep1') }}</li>
                <li data-testid="deploy-accept-step-2">{{ t('readinessDeployStep2') }}</li>
                <li data-testid="deploy-accept-step-3">{{ t('readinessDeployStep3') }}</li>
              </ol>
              <div class="space-y-1" data-testid="deploy-runbook-links">
                <p class="text-[11px] font-medium text-slate-700 dark:text-slate-200">{{ t('readinessDeployRunbookTitle') }}</p>
                <p class="font-mono text-[11px] text-slate-600 dark:text-slate-300">
                  <span class="mts-muted">{{ t('readinessDeployRunbookProd') }}:</span>
                  docs/ops/dashboard-production-runbook.md
                </p>
                <p class="font-mono text-[11px] text-slate-600 dark:text-slate-300">
                  <span class="mts-muted">{{ t('readinessDeployRunbookBackup') }}:</span>
                  docs/ops/backup-orchestration.md
                </p>
              </div>
              <div
                id="deploy-runbook-drill"
                class="mt-3 space-y-2 scroll-mt-20 rounded-lg border border-slate-200 bg-white/70 p-3 dark:border-slate-700 dark:bg-slate-950/40"
                data-testid="deploy-runbook-drill"
              >
                <div class="flex flex-wrap items-start justify-between gap-2">
                  <div class="min-w-0">
                    <p class="text-xs font-semibold text-slate-800 dark:text-slate-100">{{ t('readinessDeployDrillTitle') }}</p>
                    <p class="mt-1 text-[11px] mts-muted">{{ t('readinessDeployDrillHint') }}</p>
                  </div>
                  <div class="flex flex-wrap gap-2">
                    <button
                      type="button"
                      class="mts-btn mts-focus-ring"
                      data-testid="deploy-drill-copy"
                      @click="copyDeployDrill"
                    >{{ t('readinessDeployDrillCopy') }}</button>
                    <button
                      type="button"
                      class="mts-btn mts-focus-ring"
                      data-testid="deploy-drill-download" :disabled="exportBusy"
                      @click="downloadDeployDrill"
                    >{{ t('readinessDeployDrillDownload') }}</button>
                  </div>
                </div>
                <div
                  v-for="area in deployDrillAreas"
                  :key="area"
                  class="rounded-md border border-slate-100 p-2 dark:border-slate-800"
                  :data-testid="`deploy-drill-area-${area}`"
                >
                  <p class="mb-1 text-[11px] font-medium text-slate-700 dark:text-slate-200">{{ areaLabel(area) }}</p>
                  <ol class="list-decimal space-y-2 pl-4 text-[11px] text-slate-700 dark:text-slate-200">
                    <li
                      v-for="step in drillStepsForArea(area)"
                      :key="step.id"
                      :data-testid="`deploy-drill-step-${step.id}`"
                    >
                      <span class="font-medium">{{ textForLocale(step.title, uiLocale) }}</span>
                      <p class="mts-muted">{{ t('readinessDeployDrillAction') }}: {{ textForLocale(step.action, uiLocale) }}</p>
                      <p class="mts-muted">{{ t('readinessDeployDrillEvidence') }}: {{ textForLocale(step.evidence, uiLocale) }}</p>
                      <p v-if="step.runbookPaths.length" class="font-mono text-[10px] mts-muted">
                        {{ t('readinessDeployDrillRunbooks') }}: {{ step.runbookPaths.join(uiLocale === 'en' ? ', ' : '、') }}
                      </p>
                      <p v-if="step.templateIds?.length" class="font-mono text-[10px] mts-muted">
                        {{ t('readinessDeployDrillTemplates') }}: {{ step.templateIds.join(uiLocale === 'en' ? ', ' : '、') }}
                      </p>
                    </li>
                  </ol>
                </div>
              </div>
            </div>
          </div>
          <button
            type="button"
            class="mts-btn"
            data-testid="readiness-deploy-kit-download" :disabled="exportBusy"
            @click="downloadDeployKit"
          >
            <Download class="h-3.5 w-3.5" />
            {{ t('readinessDeployKitDownload') }}
          </button>
        </div>
        <div
          v-for="tpl in deployTemplates"
          :key="tpl.id"
          :id="`deploy-tpl-${tpl.id}`"
          class="scroll-mt-20 rounded-lg border border-slate-200 p-3 dark:border-slate-700"
          :data-testid="`deploy-tpl-${tpl.id}`"
        >
          <div class="mb-2 flex flex-wrap items-start justify-between gap-2">
            <div class="min-w-0">
              <p class="text-xs font-medium text-slate-800 dark:text-slate-100">{{ formatDeployTemplateLabel(tpl, uiLocale) }}</p>
              <p class="text-[11px] mts-muted">{{ textForLocale(tpl.description, uiLocale) }} · <span class="font-mono">{{ tpl.filename }}</span></p>
            </div>
            <div class="flex flex-wrap gap-2">
              <button
                v-if="tpl.relatedSignoff"
                type="button"
                class="mts-btn"
                :data-testid="`deploy-jump-signoff-${tpl.id}`"
                @click="jumpRelatedSignoff(tpl.relatedSignoff)"
              >{{ t('readinessDeployRelatedSignoff') }}</button>
              <button
                type="button"
                class="mts-btn"
                :data-testid="`deploy-copy-${tpl.id}`"
                @click="copyDeployBody(tpl.body)"
              >{{ t('readinessCopy') }}</button>
            </div>
          </div>
          <pre class="max-h-40 overflow-auto rounded bg-slate-950 p-3 text-[11px] text-emerald-300">{{ tpl.body }}</pre>
        </div>
        <div>
          <p class="mb-1 text-xs font-medium text-slate-700 dark:text-slate-200">{{ t('readinessBackupScript') }}</p>
          <p class="mb-2 text-xs mts-muted">{{ t('readinessBackupScriptHint') }}</p>
          <pre class="overflow-x-auto rounded bg-slate-950 p-3 text-[11px] text-emerald-300">{{ backupScriptHint }}</pre>
        </div>
      </div>
    </div>

    <div id="signoff-notes" class="mts-panel scroll-mt-20" data-testid="readiness-signoff-notes">
      <div class="mb-2">
        <h2 class="text-sm font-semibold">{{ t('readinessSignoffTitle') }}</h2>
        <p class="mt-1 text-xs mts-muted">{{ t('readinessSignoffHint') }}</p>
        <p class="mt-1 text-[11px] text-amber-700 dark:text-amber-200">{{ t('readinessSignoffManualNote') }}</p>
        <div class="mt-2 rounded-lg border border-sky-100 bg-sky-50/70 px-2 py-1.5 text-[11px] text-sky-900 dark:border-sky-900 dark:bg-sky-950/30 dark:text-sky-100" data-testid="readiness-signoff-guide-banner">
          {{ t('readinessSignoffGuideBanner') }}
        </div>
        <div class="mt-2" data-testid="signoff-progress">
          <div class="mb-1 flex flex-wrap items-center justify-between gap-2 text-xs">
            <span class="mts-muted">{{ formatMessage(t('readinessSignoffProgress'), { percent: String(signoffProgress) }) }}</span>
            <button
              v-if="!signoffCompleteness.complete"
              type="button"
              class="mts-btn"
              data-testid="signoff-copy-missing"
              @click="copySignoffMissing"
            >{{ t('readinessSignoffCopyMissing') }}</button>
          </div>
          <div
            class="h-2 overflow-hidden rounded-full bg-slate-100 dark:bg-slate-800"
            role="progressbar"
            :aria-valuenow="signoffProgress"
            aria-valuemin="0"
            aria-valuemax="100"
            data-testid="signoff-progress-bar"
          >
            <div
              class="h-full rounded-full transition-[width] duration-200"
              :class="signoffCompleteness.complete ? 'bg-emerald-500' : 'bg-amber-500'"
              :style="{ width: `${signoffProgress}%` }"
              data-testid="signoff-progress-fill"
            />
          </div>
        </div>
        <p
          class="mt-2 text-xs"
          data-testid="signoff-completeness"
          :class="signoffCompleteness.complete ? 'text-emerald-700 dark:text-emerald-300' : 'text-amber-700 dark:text-amber-200'"
        >
          {{
            signoffCompleteness.complete
              ? t('readinessSignoffComplete')
              : formatMessage(t('readinessSignoffMissing'), {
                  missing: signoffMissingLabels.join(uiLocale === 'en' ? ', ' : '、'),
                  filled: String(signoffCompleteness.filledCount),
                  total: String(signoffCompleteness.total),
                })
          }}
        </p>
        <div v-if="signoffCompleteness.missing.length" class="mt-2 flex flex-wrap gap-2" data-testid="signoff-missing-jumps">
          <button
            v-for="field in signoffCompleteness.missing"
            :key="field"
            type="button"
            class="mts-btn"
            :data-testid="`signoff-jump-${field}`"
            @click="focusSignoffField(field)"
          >{{ t('readinessSignoffJump') }}: {{ signoffFieldLabel(field, uiLocale) }}</button>
        </div>
      </div>
      <div class="space-y-3">
        <label class="block text-xs text-slate-700 dark:text-slate-200">
          <span class="mb-1 flex items-center justify-between gap-2 font-medium">
            <span>{{ t('readinessSignoffEdge') }}</span>
            <span
              class="rounded-full px-2 py-0.5 text-[10px] font-normal"
              data-testid="signoff-field-status-edgeHttps"
              :class="isSignoffFieldFilled('edgeHttps') ? 'bg-emerald-50 text-emerald-800 dark:bg-emerald-950/40 dark:text-emerald-200' : 'bg-amber-50 text-amber-900 dark:bg-amber-950/40 dark:text-amber-100'"
            >{{ isSignoffFieldFilled('edgeHttps') ? t('readinessSignoffFieldDone') : t('readinessSignoffFieldTodo') }}</span>
          </span>
          <textarea
            id="signoff-field-edgeHttps"
            data-testid="signoff-edge-https"
            class="mts-input min-h-[4.5rem] w-full text-xs"
            :value="state.signoffNotes?.edgeHttps ?? ''"
            :placeholder="t('readinessSignoffEdgePh')"
            maxlength="2000"
            @input="saveSignoff('edgeHttps', ($event.target as HTMLTextAreaElement).value)"
          />
                    <details class="mt-2 rounded-lg border border-slate-100 bg-slate-50/80 px-2 py-1.5 dark:border-slate-800 dark:bg-slate-900/40" data-testid="signoff-guide-edgeHttps">
            <summary class="cursor-pointer text-[11px] font-medium text-slate-700 dark:text-slate-200" data-testid="signoff-guide-summary-edgeHttps">
              {{ t('readinessSignoffGuideToggle') }}
            </summary>
            <p class="mt-1 text-[11px] mts-muted" data-testid="signoff-guide-blurb-edgeHttps">{{ guideSummaryFor('edgeHttps') }}</p>
            <ol class="mt-1 list-decimal space-y-1 pl-4 text-[11px] text-slate-600 dark:text-slate-300">
              <li v-for="step in guideStepsFor('edgeHttps')" :key="step.id" :data-testid="`signoff-guide-step-edgeHttps-${step.id}`">
                <span class="font-medium">{{ step.title }}</span>
                <span class="mts-muted"> — {{ step.detail }}</span>
              </li>
            </ol>
            <button
              type="button"
              class="mts-btn mt-2 !px-2 !py-0.5 text-[10px]"
              data-testid="signoff-guide-fill-edgeHttps"
              @click="applyExampleToSignoff('edgeHttps')"
            >{{ t('readinessSignoffFillExample') }}</button>
            <p class="mt-1 text-[10px] text-amber-700 dark:text-amber-200">{{ t('readinessSignoffExampleNote') }}</p>
          </details>
<div class="mt-1 flex flex-wrap items-center gap-2" data-testid="signoff-related-edgeHttps">
            <span class="text-[10px] mts-muted">{{ t('readinessSignoffRelatedTemplates') }}:</span>
            <button
              v-for="tpl in relatedTemplatesFor('edgeHttps')"
              :key="tpl.id"
              type="button"
              class="mts-btn !px-2 !py-0.5 text-[10px]"
              :data-testid="`signoff-open-tpl-${tpl.id}`"
              @click="jumpDeployTemplate(tpl.id)"
            >{{ t('readinessSignoffJumpTemplate') }}: {{ formatDeployTemplateLabel(tpl, uiLocale) }}</button>
          </div>
        </label>
        <label class="block text-xs text-slate-700 dark:text-slate-200">
          <span class="mb-1 flex items-center justify-between gap-2 font-medium">
            <span>{{ t('readinessSignoffBackup') }}</span>
            <span
              class="rounded-full px-2 py-0.5 text-[10px] font-normal"
              data-testid="signoff-field-status-backupOffsite"
              :class="isSignoffFieldFilled('backupOffsite') ? 'bg-emerald-50 text-emerald-800 dark:bg-emerald-950/40 dark:text-emerald-200' : 'bg-amber-50 text-amber-900 dark:bg-amber-950/40 dark:text-amber-100'"
            >{{ isSignoffFieldFilled('backupOffsite') ? t('readinessSignoffFieldDone') : t('readinessSignoffFieldTodo') }}</span>
          </span>
          <textarea
            id="signoff-field-backupOffsite"
            data-testid="signoff-backup-offsite"
            class="mts-input min-h-[4.5rem] w-full text-xs"
            :value="state.signoffNotes?.backupOffsite ?? ''"
            :placeholder="t('readinessSignoffBackupPh')"
            maxlength="2000"
            @input="saveSignoff('backupOffsite', ($event.target as HTMLTextAreaElement).value)"
          />
                    <details class="mt-2 rounded-lg border border-slate-100 bg-slate-50/80 px-2 py-1.5 dark:border-slate-800 dark:bg-slate-900/40" data-testid="signoff-guide-backupOffsite">
            <summary class="cursor-pointer text-[11px] font-medium text-slate-700 dark:text-slate-200" data-testid="signoff-guide-summary-backupOffsite">
              {{ t('readinessSignoffGuideToggle') }}
            </summary>
            <p class="mt-1 text-[11px] mts-muted" data-testid="signoff-guide-blurb-backupOffsite">{{ guideSummaryFor('backupOffsite') }}</p>
            <ol class="mt-1 list-decimal space-y-1 pl-4 text-[11px] text-slate-600 dark:text-slate-300">
              <li v-for="step in guideStepsFor('backupOffsite')" :key="step.id" :data-testid="`signoff-guide-step-backupOffsite-${step.id}`">
                <span class="font-medium">{{ step.title }}</span>
                <span class="mts-muted"> — {{ step.detail }}</span>
              </li>
            </ol>
            <button
              type="button"
              class="mts-btn mt-2 !px-2 !py-0.5 text-[10px]"
              data-testid="signoff-guide-fill-backupOffsite"
              @click="applyExampleToSignoff('backupOffsite')"
            >{{ t('readinessSignoffFillExample') }}</button>
            <p class="mt-1 text-[10px] text-amber-700 dark:text-amber-200">{{ t('readinessSignoffExampleNote') }}</p>
          </details>
<div class="mt-1 flex flex-wrap items-center gap-2" data-testid="signoff-related-backupOffsite">
            <span class="text-[10px] mts-muted">{{ t('readinessSignoffRelatedTemplates') }}:</span>
            <button
              v-for="tpl in relatedTemplatesFor('backupOffsite').slice(0, 3)"
              :key="tpl.id"
              type="button"
              class="mts-btn !px-2 !py-0.5 text-[10px]"
              :data-testid="`signoff-open-tpl-${tpl.id}`"
              @click="jumpDeployTemplate(tpl.id)"
            >{{ t('readinessSignoffJumpTemplate') }}: {{ formatDeployTemplateLabel(tpl, uiLocale) }}</button>
          </div>
        </label>
        <label class="block text-xs text-slate-700 dark:text-slate-200">
          <span class="mb-1 flex items-center justify-between gap-2 font-medium">
            <span>{{ t('readinessSignoffAlert') }}</span>
            <span
              class="rounded-full px-2 py-0.5 text-[10px] font-normal"
              data-testid="signoff-field-status-backupAlert"
              :class="isSignoffFieldFilled('backupAlert') ? 'bg-emerald-50 text-emerald-800 dark:bg-emerald-950/40 dark:text-emerald-200' : 'bg-amber-50 text-amber-900 dark:bg-amber-950/40 dark:text-amber-100'"
            >{{ isSignoffFieldFilled('backupAlert') ? t('readinessSignoffFieldDone') : t('readinessSignoffFieldTodo') }}</span>
          </span>
          <textarea
            id="signoff-field-backupAlert"
            data-testid="signoff-backup-alert"
            class="mts-input min-h-[4.5rem] w-full text-xs"
            :value="state.signoffNotes?.backupAlert ?? ''"
            :placeholder="t('readinessSignoffAlertPh')"
            maxlength="2000"
            @input="saveSignoff('backupAlert', ($event.target as HTMLTextAreaElement).value)"
          />
                    <details class="mt-2 rounded-lg border border-slate-100 bg-slate-50/80 px-2 py-1.5 dark:border-slate-800 dark:bg-slate-900/40" data-testid="signoff-guide-backupAlert">
            <summary class="cursor-pointer text-[11px] font-medium text-slate-700 dark:text-slate-200" data-testid="signoff-guide-summary-backupAlert">
              {{ t('readinessSignoffGuideToggle') }}
            </summary>
            <p class="mt-1 text-[11px] mts-muted" data-testid="signoff-guide-blurb-backupAlert">{{ guideSummaryFor('backupAlert') }}</p>
            <ol class="mt-1 list-decimal space-y-1 pl-4 text-[11px] text-slate-600 dark:text-slate-300">
              <li v-for="step in guideStepsFor('backupAlert')" :key="step.id" :data-testid="`signoff-guide-step-backupAlert-${step.id}`">
                <span class="font-medium">{{ step.title }}</span>
                <span class="mts-muted"> — {{ step.detail }}</span>
              </li>
            </ol>
            <button
              type="button"
              class="mts-btn mt-2 !px-2 !py-0.5 text-[10px]"
              data-testid="signoff-guide-fill-backupAlert"
              @click="applyExampleToSignoff('backupAlert')"
            >{{ t('readinessSignoffFillExample') }}</button>
            <p class="mt-1 text-[10px] text-amber-700 dark:text-amber-200">{{ t('readinessSignoffExampleNote') }}</p>
          </details>
<div class="mt-1 flex flex-wrap items-center gap-2" data-testid="signoff-related-backupAlert">
            <span class="text-[10px] mts-muted">{{ t('readinessSignoffRelatedTemplates') }}:</span>
            <button
              v-for="tpl in relatedTemplatesFor('backupAlert')"
              :key="tpl.id"
              type="button"
              class="mts-btn !px-2 !py-0.5 text-[10px]"
              :data-testid="`signoff-open-tpl-${tpl.id}`"
              @click="jumpDeployTemplate(tpl.id)"
            >{{ t('readinessSignoffJumpTemplate') }}: {{ formatDeployTemplateLabel(tpl, uiLocale) }}</button>
          </div>
        </label>
      </div>
    </div>

    <div id="doctor-panel" class="mts-panel scroll-mt-20" data-testid="readiness-doctor-panel">
      <div class="mb-3 flex flex-wrap items-center justify-between gap-2">
        <h2 class="flex items-center gap-2 text-sm font-semibold">
          <ShieldCheck class="h-4 w-4" />
          {{ t('readinessDoctorTitle') }}
        </h2>
        <span class="text-xs mts-muted" data-testid="readiness-doctor-summary">
          {{ formatMessage(t('readinessDoctorSummary'), {
            ok: doctor?.ok == null ? t('emptyValue') : String(doctor.ok),
            warn: String(doctorWarns.length),
            tls: doctor?.http_tls_enabled == null ? t('emptyValue') : String(doctor.http_tls_enabled),
          }) }}
        </span>
      </div>
      <div v-if="doctor && (doctor.checks ?? []).length" class="overflow-hidden rounded-lg border border-slate-100 dark:border-slate-800" data-testid="readiness-doctor-checks">
        <div class="grid grid-cols-[minmax(5rem,0.6fr)_minmax(7rem,0.9fr)_minmax(8rem,1.3fr)] border-b border-slate-200 px-2 py-2 text-left text-[11px] uppercase mts-muted dark:border-slate-700">
          <span>{{ t('readinessDoctorColLevel') }}</span>
          <span>{{ t('readinessDoctorColCode') }}</span>
          <span>{{ t('readinessDoctorColMessage') }}</span>
        </div>
        <VirtualTable
          :items="doctor.checks ?? []"
          :row-height="DOC_ROW_HEIGHT"
          :height="Math.min(DOC_LIST_HEIGHT, Math.max(120, (doctor.checks ?? []).length * DOC_ROW_HEIGHT))"
          data-testid="readiness-doctor-virtual-list"
        >
          <template #default="{ item: c, index }">
            <div class="grid h-full grid-cols-[minmax(5rem,0.6fr)_minmax(7rem,0.9fr)_minmax(8rem,1.3fr)] items-center border-b border-slate-100 px-2 text-xs dark:border-slate-800" :data-testid="`readiness-doctor-row-${index}`">
              <span :class="healthStatusToneClass(c.level)">{{ formatDoctorLevel(c.level) }}</span>
              <span class="truncate font-mono" :title="c.code">{{ c.code }}</span>
              <span class="truncate mts-muted" :title="c.message">{{ c.message }}</span>
            </div>
          </template>
        </VirtualTable>
        <p class="border-t border-slate-100 px-3 py-1.5 text-[11px] mts-muted dark:border-slate-800" data-testid="readiness-doctor-virtual-hint">{{ t('readinessDoctorVirtualHint') }}</p>
      </div>
      <EmptyState
        v-else-if="!loadingDoctor"
        compact
        data-testid="readiness-doctor-empty"
        :title="doctorError ? t('overviewDoctorEmptyFailed') : t('readinessDoctorEmpty')"
        :description="doctorError || t('readinessDoctorEmptyDesc')"
      >
        <template #action>
          <button type="button" class="mts-btn-primary" data-testid="readiness-doctor-empty-retry" :disabled="loadingDoctor" @click="loadDoctor">{{ t('readinessDoctorRetry') }}</button>
        </template>
      </EmptyState>
      <p v-else class="text-xs mts-muted">{{ t('loading') }}</p>
      <p v-if="doctorOKs.length" class="mt-2 text-[11px] mts-muted">
        {{ formatMessage(t('readinessDoctorOkChecks'), { count: doctorOKs.length }) }}
      </p>
    </div>

    <div id="production-checklist" class="mts-card p-4 scroll-mt-20" data-testid="readiness-production-checklist">
      <div class="mb-2 flex flex-wrap items-center justify-between gap-2">
        <h2 class="text-sm font-semibold">{{ t('readinessProductionChecklist') }}</h2>
        <span class="text-xs mts-muted">{{ productionDone.length }}/{{ PRODUCTION_CHECKLIST.length }}</span>
      </div>
      <ol class="space-y-2">
        <li
          v-for="item in PRODUCTION_CHECKLIST"
          :key="item.id"
          class="flex items-start gap-2 rounded-lg border border-slate-100 px-3 py-2 text-sm dark:border-slate-800"
        >
          <input
            type="checkbox"
            class="mt-1"
            :data-testid="`readiness-prod-${item.id}`"
            :checked="!!state.production[item.id]"
            @change="toggle('production', item.id, ($event.target as HTMLInputElement).checked)"
          />
          <div class="min-w-0">
            <p class="font-medium text-slate-800 dark:text-slate-100">
              {{ textForLocale(item.title, uiLocale) }}
              <span class="ml-1 text-[11px] font-normal mts-muted">{{ item.severity === 'required' ? t('required') : t('recommended') }}</span>
              <span v-if="item.automated" class="ml-1 text-[11px] font-normal text-emerald-700 dark:text-emerald-300">{{ t('partialAuto') }}</span>
            </p>
            <p class="text-xs mts-muted">{{ textForLocale(item.detail, uiLocale) }}</p>
          </div>
        </li>
      </ol>
    </div>

    <div id="edge-https-checklist" class="mts-card p-4 scroll-mt-20" data-testid="readiness-edge-https-checklist">
      <div class="mb-2 flex flex-wrap items-center justify-between gap-2">
        <h2 class="text-sm font-semibold">{{ t('readinessEdgeHttps') }}</h2>
        <span class="text-xs mts-muted">
          {{ edgeStats.done }}/{{ edgeStats.total }} · {{ t('required') }} {{ edgeStats.requiredDone }}/{{ edgeStats.requiredTotal }}
        </span>
      </div>
      <ol class="space-y-2">
        <li
          v-for="step in EDGE_HTTPS_ACCEPTANCE_STEPS"
          :key="step.id"
          class="flex items-start gap-2 rounded-lg border border-slate-100 px-3 py-2 text-sm dark:border-slate-800"
        >
          <input
            type="checkbox"
            class="mt-1"
            :data-testid="`readiness-edge-${step.id}`"
            :checked="!!state.edgeHttps[step.id]"
            @change="toggle('edgeHttps', step.id, ($event.target as HTMLInputElement).checked)"
          />
          <div class="min-w-0">
            <p class="font-medium text-slate-800 dark:text-slate-100">
              {{ textForLocale(step.title, uiLocale) }}
              <span class="ml-1 text-[11px] font-normal mts-muted">{{ step.severity === 'required' ? t('required') : t('recommended') }}</span>
            </p>
            <p class="text-xs mts-muted">{{ textForLocale(step.detail, uiLocale) }}</p>
          </div>
        </li>
      </ol>
    </div>

    <div id="backup-schedule-checklist" class="mts-card p-4 scroll-mt-20" data-testid="readiness-backup-schedule-checklist">
      <div class="mb-2 flex flex-wrap items-center justify-between gap-2">
        <h2 class="text-sm font-semibold">{{ t('readinessBackupSchedule') }}</h2>
        <span class="text-xs mts-muted">
          {{ scheduleStats.done }}/{{ scheduleStats.total }} · {{ t('required') }} {{ scheduleStats.requiredDone }}/{{ scheduleStats.requiredTotal }}
        </span>
      </div>
      <p class="mb-3 text-xs mts-muted">{{ t('readinessBackupScheduleHint') }}</p>
      <ol class="space-y-2">
        <li
          v-for="step in BACKUP_SCHEDULE_STEPS"
          :key="step.id"
          class="flex items-start gap-2 rounded-lg border border-slate-100 px-3 py-2 text-sm dark:border-slate-800"
        >
          <input
            type="checkbox"
            class="mt-1"
            :data-testid="`readiness-sched-${step.id}`"
            :checked="!!state.backupSchedule[step.id]"
            @change="toggle('backupSchedule', step.id, ($event.target as HTMLInputElement).checked)"
          />
          <div class="min-w-0 flex-1">
            <p class="font-medium text-slate-800 dark:text-slate-100">
              {{ textForLocale(step.title, uiLocale) }}
              <span class="ml-1 text-[11px] font-normal mts-muted">{{ step.severity === 'required' ? t('required') : t('recommended') }}</span>
            </p>
            <p class="text-xs mts-muted">{{ textForLocale(step.detail, uiLocale) }}</p>
            <pre
              v-if="step.example"
              class="mt-1 overflow-x-auto rounded bg-slate-950 px-2 py-1 text-[11px] text-emerald-300"
            >{{ step.example }}</pre>
          </div>
        </li>
      </ol>
    </div>
  </div>
</template>
