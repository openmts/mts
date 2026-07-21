<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount, computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useHashScroll } from '@/composables/useHashScroll'
import { overviewFormToPrefill, parseOverviewPrefill } from '@/utils/routePrefill'
import { apiGet } from '@/api/client'
import { formatCaughtError } from '@/utils/apiError'
import { useAuth } from '@/composables/useAuth'
import { useI18n } from '@/composables/useI18n'
import { useServerReachability } from '@/composables/useServerReachability'
import { healthStatusLabel, healthStatusToneClass } from '@/utils/healthStatusLabel'
import ActionResultBanner from '@/components/ActionResultBanner.vue'
import type { HealthSnapshot, MaintenanceStats, CompactionStats } from '@/api/types'
import { clientBuildInfo } from '@/utils/buildInfo'
import { parseExpiresAt, sessionExpiryView } from '@/utils/sessionExpiry'
import { completedIds, loadReadinessState } from '@/utils/readinessState'
import { computeReadinessScore, readinessLevel } from '@/utils/readinessScore'
import { requiredChecklist } from '@/utils/productionChecklist'
import { edgeHttpsProgress } from '@/utils/edgeHttpsAcceptance'
import { backupScheduleProgress } from '@/utils/backupSchedule'
import {
  assessSignoffCompleteness,
  signoffFieldLabel,
  signoffProgressPercent,
} from '@/utils/signoffExport'
import { buildExportPreflight } from '@/utils/exportPreflight'
import { buildOpsNextSteps } from '@/utils/opsNextSteps'
import { formatMessage } from '@/utils/formatMessage'
import type { LocaleCode } from '@/utils/localizedText'
import { Activity, RefreshCw, Cpu, Layers, Wrench, AlertTriangle, ShieldCheck, ClipboardCheck, Info, Clock3, FileCode2, Download, Copy } from 'lucide-vue-next'
import { useNotify } from '@/composables/useNotify'
import { buildOverviewExport, formatOverviewExportPretty } from '@/utils/overviewExport'
import { stampFilename } from '@/utils/download'
import { useExportJob } from '@/composables/useExportJob'
import ExportJobBanner from '@/components/ExportJobBanner.vue'
import { copyText } from '@/utils/clipboard'
import VirtualTable from '@/components/VirtualTable.vue'

interface HealthResponse extends HealthSnapshot {}

const CHECK_ROW_HEIGHT = 40
const CHECK_LIST_HEIGHT = 280
const MAINT_ROW_HEIGHT = 36
const MAINT_LIST_HEIGHT = 240
interface StorageMemorySnapshot {
  heap_alloc_bytes?: number
  heap_inuse_bytes?: number
  sys_bytes?: number
  num_gc?: number
  [key: string]: unknown
}
interface StorageMemoryResponse { snapshot: StorageMemorySnapshot }
interface CompactionStatsResponse { stats: CompactionStats }
interface MaintenanceStatsResponse { stats: MaintenanceStats }
interface MaintenanceErrorsResponse { errors: string[] }
interface AdminHealthResponse { health?: HealthSnapshot; healthy?: boolean; ready?: boolean; reasons?: string[]; checks?: HealthSnapshot['checks'] }
interface DoctorCheck { level: string; code: string; message: string }
interface DoctorResponse { ok: boolean; http_tls_enabled?: boolean; checks?: DoctorCheck[]; lines?: string[] }

const { isAdmin, getTokenExpiresAt } = useAuth()
const router = useRouter()
const route = useRoute()
useHashScroll()
const { t, locale } = useI18n()
const { success, error: notifyError } = useNotify()
const {
  exportJob,
  exportBusy,
  cancelExport,
  resetExport,
  runJSONExport,
  runTextExport,
} = useExportJob()
const {
  kind: connectivityKind,
  showUnreachableBanner,
  checking: reachChecking,
  checkOnce: retryReadyz,
} = useServerReachability()
const uiLocale = computed<LocaleCode>(() => (locale.value === 'en' ? 'en' : 'zh'))
const healthy = ref<boolean | null>(null)
const ready = ref<boolean | null>(null)
const healthReasons = ref<string[]>([])
const healthChecks = ref<{ name: string; status: string; reason?: string }[]>([])
const maintenanceErrors = ref<string[]>([])
const memorySnapshot = ref<StorageMemorySnapshot | null>(null)
const compactionStats = ref<CompactionStats | null>(null)
const maintenanceStats = ref<MaintenanceStats | null>(null)
const doctorChecks = ref<DoctorCheck[]>([])
const doctorTLS = ref<boolean | null>(null)
const loadError = ref('')
const adminPartialError = ref('')
const loading = ref(false)
const lastRefreshed = ref('')
const autoRefresh = ref(false)
let timer: ReturnType<typeof setInterval> | null = null
const nowMs = ref(Date.now())
let clockTimer: ReturnType<typeof setInterval> | null = null
const serverVersion = ref<{ version: string; commit: string; built_at: string } | null>(null)
const clientInfo = clientBuildInfo()
const readinessTick = ref(0)

const localReadiness = computed(() => {
  readinessTick.value // depend for refresh after load
  return loadReadinessState()
})

const localReadinessScore = computed(() => {
  const state = localReadiness.value
  const requiredItems = requiredChecklist()
  const requiredDone = requiredItems.filter((i) => !!state.production[i.id]).length
  const requiredRatio = requiredItems.length === 0 ? 1 : requiredDone / requiredItems.length
  const edgeDone = completedIds(state.edgeHttps)
  const edgeStats = edgeHttpsProgress(edgeDone)
  const edgeRatio =
    edgeStats.requiredTotal === 0 ? 1 : edgeStats.requiredDone / edgeStats.requiredTotal
  const scheduleDone = completedIds(state.backupSchedule)
  const scheduleStats = backupScheduleProgress(scheduleDone)
  const scheduleRatio =
    scheduleStats.requiredTotal === 0 ? 1 : scheduleStats.requiredDone / scheduleStats.requiredTotal
  const doctorWarnCount = doctorChecks.value.filter((c) => c.level === 'warn').length
  const doctorLoaded = showAdminPanels.value && doctorTLS.value != null
  const doctorOk = doctorLoaded ? doctorWarnCount === 0 : undefined
  return computeReadinessScore({
    requiredChecklistRatio: requiredRatio,
    edgeHttpsRequiredRatio: edgeRatio,
    backupScheduleRequiredRatio: scheduleRatio,
    doctorLoaded,
    doctorOk,
    doctorWarnCount,
    httpTlsEnabled: doctorTLS.value,
  })
})

const localReadinessLevel = computed(() => readinessLevel(localReadinessScore.value.total))

const signoffCompleteness = computed(() => assessSignoffCompleteness(localReadiness.value.signoffNotes))
const signoffMissingLabels = computed(() =>
  signoffCompleteness.value.missing.map((f) => signoffFieldLabel(f, uiLocale.value)),
)
const signoffProgress = computed(() => signoffProgressPercent(localReadiness.value.signoffNotes))

const overviewPreflight = computed(() => {
  const state = localReadiness.value
  const requiredItems = requiredChecklist()
  const requiredDone = requiredItems.filter((i) => !!state.production[i.id]).length
  const requiredRatio = requiredItems.length === 0 ? 1 : requiredDone / requiredItems.length
  const edgeDone = completedIds(state.edgeHttps)
  const edgeStats = edgeHttpsProgress(edgeDone)
  const edgeRatio =
    edgeStats.requiredTotal === 0 ? 1 : edgeStats.requiredDone / edgeStats.requiredTotal
  const scheduleDone = completedIds(state.backupSchedule)
  const scheduleStats = backupScheduleProgress(scheduleDone)
  const scheduleRatio =
    scheduleStats.requiredTotal === 0 ? 1 : scheduleStats.requiredDone / scheduleStats.requiredTotal
  const doctorWarnCount = doctorChecks.value.filter((c) => c.level === 'warn').length
  const doctorLoaded = showAdminPanels.value && doctorTLS.value != null
  return buildExportPreflight({
    locale: uiLocale.value,
    requiredChecklistRatio: requiredRatio,
    edgeHttpsRequiredRatio: edgeRatio,
    backupScheduleRequiredRatio: scheduleRatio,
    doctorLoaded,
    doctorOk: doctorLoaded ? doctorWarnCount === 0 : undefined,
    doctorWarnCount,
    httpTlsEnabled: doctorTLS.value,
    signoffNotes: state.signoffNotes,
    deployKitReviewed: !!state.deployKit?.reviewed,
  })
})

const overviewNextSteps = computed(() =>
  buildOpsNextSteps({
    locale: uiLocale.value,
    preflight: overviewPreflight.value,
    signoffNotes: localReadiness.value.signoffNotes,
    limit: 4,
  }),
)

function readinessLevelLabel(level: string): string {
  if (level === 'good') return t.value('readinessLevelGood')
  if (level === 'warn') return t.value('readinessLevelWarn')
  return t.value('readinessLevelBad')
}

function jumpNextStep(target?: string) {
  if (!target) return
  if (target.startsWith('/')) {
    void router.push(target)
    return
  }
  void router.push({ path: '/ops/readiness', hash: target.startsWith('#') ? target : `#${target}` })
}

function goSignoffNotes() {
  router.push({ path: '/ops/readiness', hash: '#signoff-notes' })
}

function goExportPreflight() {
  router.push({ path: '/ops/readiness', hash: '#export-preflight' })
}

const sessionSummary = computed(() => {
  const exp = parseExpiresAt(getTokenExpiresAt())
  return sessionExpiryView(exp, nowMs.value, undefined, undefined, locale.value === 'en' ? 'en' : 'zh')
})

function memoryLabel(key: string): string {
  switch (key) {
    case 'heap_alloc_bytes':
      return t.value('memHeapAlloc')
    case 'heap_inuse_bytes':
      return t.value('memHeapInuse')
    case 'sys_bytes':
      return t.value('memSys')
    case 'num_gc':
      return t.value('memNumGC')
    default:
      return key
  }
}

function formatBytes(bytes: number): string {

  if (bytes >= 1 << 30) return (bytes / (1 << 30)).toFixed(1) + ' GB'
  if (bytes >= 1 << 20) return (bytes / (1 << 20)).toFixed(1) + ' MB'
  if (bytes >= 1 << 10) return (bytes / (1 << 10)).toFixed(1) + ' KB'
  return bytes + ' B'
}

function applyHealth(h: HealthSnapshot) {
  healthy.value = h.healthy
  ready.value = h.ready
  healthReasons.value = h.reasons ?? []
  healthChecks.value = h.checks ?? []
}

async function loadOverview() {
  loading.value = true
  loadError.value = ''
  adminPartialError.value = ''
  try {
    const healthData = await apiGet<HealthResponse>('/healthz')
    applyHealth(healthData)

    if (isAdmin.value) {
      const results = await Promise.allSettled([
        apiGet<StorageMemoryResponse>('/api/v1/admin/stats/storage-memory'),
        apiGet<CompactionStatsResponse>('/api/v1/admin/stats/compaction'),
        apiGet<MaintenanceStatsResponse>('/api/v1/admin/stats/maintenance'),
        apiGet<MaintenanceErrorsResponse>('/api/v1/admin/maintenance/errors'),
        apiGet<AdminHealthResponse | HealthSnapshot>('/api/v1/admin/health'),
        apiGet<DoctorResponse>('/api/v1/admin/doctor'),
        apiGet<{ version: string; commit: string; built_at: string }>('/api/v1/admin/version'),
      ])
      const errs: string[] = []
      if (results[0].status === 'fulfilled') memorySnapshot.value = results[0].value.snapshot
      else {
        memorySnapshot.value = null
        errs.push(formatCaughtError(results[0].reason))
      }
      if (results[1].status === 'fulfilled') compactionStats.value = results[1].value.stats
      else {
        compactionStats.value = null
        errs.push(formatCaughtError(results[1].reason))
      }
      if (results[2].status === 'fulfilled') maintenanceStats.value = results[2].value.stats ?? null
      else {
        maintenanceStats.value = null
        errs.push(formatCaughtError(results[2].reason))
      }
      if (results[3].status === 'fulfilled') maintenanceErrors.value = results[3].value.errors ?? []
      else {
        maintenanceErrors.value = []
        errs.push(formatCaughtError(results[3].reason))
      }
      if (results[4].status === 'fulfilled') {
        const raw = results[4].value as AdminHealthResponse
        const h = (raw.health ?? raw) as HealthSnapshot
        if (h && typeof h.healthy === 'boolean') {
          // admin health 更完整时覆盖 checks
          if (h.checks?.length) healthChecks.value = h.checks
          if (h.reasons?.length) healthReasons.value = h.reasons
        }
      }
      if (results[5].status === 'fulfilled') {
        doctorChecks.value = results[5].value.checks ?? []
        doctorTLS.value = !!results[5].value.http_tls_enabled
      } else {
        doctorChecks.value = []
        doctorTLS.value = null
        errs.push(formatCaughtError(results[5].reason))
      }
      if (results[6].status === 'fulfilled') {
        serverVersion.value = results[6].value
      } else {
        serverVersion.value = null
        errs.push(formatCaughtError(results[6].reason))
      }
      if (errs.length) adminPartialError.value = errs.join('；')
    } else {
      memorySnapshot.value = null
      compactionStats.value = null
      maintenanceStats.value = null
      maintenanceErrors.value = []
      doctorChecks.value = []
      doctorTLS.value = null
      serverVersion.value = null
    }
    lastRefreshed.value = new Date().toLocaleTimeString()
    readinessTick.value += 1
  } catch (e) {
    loadError.value = formatCaughtError(e)
  } finally {
    loading.value = false
  }
}

function toggleAutoRefresh() {
  autoRefresh.value = !autoRefresh.value
  if (timer) {
    clearInterval(timer)
    timer = null
  }
  if (autoRefresh.value) {
    timer = setInterval(() => { void loadOverview() }, 10000)
  }
}


function currentOverviewSection(): string {
  const h = (route.hash || (typeof window !== 'undefined' ? window.location.hash : '') || '').replace(/^#/, '')
  const known = new Set([
    'overview-summary',
    'overview-readiness',
    'overview-health',
    'overview-health-checks',
    'overview-doctor',
    'overview-workspace',
    'overview-maint',
  ])
  if (known.has(h)) return h
  const pre = parseOverviewPrefill(route.query as Record<string, unknown>, route.hash)
  return pre.section || 'overview-summary'
}

async function copyOverviewShareLink() {
  const path = overviewFormToPrefill({ section: currentOverviewSection() })
  const url = `${window.location.origin}${path}`
  const res = await copyText(url)
  if (res.ok) success(t.value('overviewShareCopied'))
  else notifyError(res.error || t.value('failed'))
}

function goReadiness() {
  void router.push('/ops/readiness')
}

function goDeployKit() {
  void router.push({ path: '/ops/readiness', hash: '#deploy-kit' })
}


onMounted(() => {
  void loadOverview()
  clockTimer = setInterval(() => { nowMs.value = Date.now() }, 15_000)
})
onBeforeUnmount(() => {
  if (timer) clearInterval(timer)
  if (clockTimer) clearInterval(clockTimer)
})


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
const connectivityTone = computed(() => {
  switch (connectivityKind.value) {
    case 'ok':
      return 'text-green-600 dark:text-green-400'
    case 'unreachable':
      return 'text-red-600 dark:text-red-400'
    case 'offline':
      return 'text-amber-700 dark:text-amber-300'
    default:
      return 'mts-muted'
  }
})
function formatHealthStatus(status?: string) {
  return healthStatusLabel(status, locale.value === 'en' ? 'en' : 'zh')
}

const showAdminPanels = computed(() => isAdmin.value)

function buildOverviewSnapshotPayload() {
  return buildOverviewExport({
    connectivity: connectivityKind.value,
    healthy: healthy.value,
    ready: ready.value,
    health_reasons: healthReasons.value,
    health_checks: healthChecks.value,
    maintenance_errors: maintenanceErrors.value,
    memory: memorySnapshot.value as object | null,
    compaction: compactionStats.value as object | null,
    maintenance: maintenanceStats.value as object | null,
    doctor_tls: doctorTLS.value,
    doctor_checks: doctorChecks.value,
    readiness_total: localReadinessScore.value.total,
    readiness_level: localReadinessLevel.value,
    server_version: serverVersion.value,
    client: clientInfo as unknown as object,
    last_refreshed: lastRefreshed.value,
  })
}

async function exportOverview() {
  if (exportBusy.value) return
  const payload = buildOverviewSnapshotPayload()
  const outcome = await runJSONExport({
    label: 'JSON',
    filename: stampFilename('mts-overview', 'json'),
    total: 1,
    build: async ({ isCancelled, progress }) => {
      progress(0, 1)
      if (isCancelled()) return null
      progress(1, 1)
      return payload
    },
  })
  if (outcome === 'done') success(t.value('overviewExported'))
  else if (outcome === 'cancelled') success(t.value('exportCancelledToast'))
  else if (outcome === 'error') notifyError(exportJob.value.error || t.value('failed'))
}

async function copyOverview() {
  const res = await copyText(formatOverviewExportPretty({
    connectivity: connectivityKind.value,
    healthy: healthy.value,
    ready: ready.value,
    health_reasons: healthReasons.value,
    health_checks: healthChecks.value,
    maintenance_errors: maintenanceErrors.value,
    memory: memorySnapshot.value as object | null,
    compaction: compactionStats.value as object | null,
    maintenance: maintenanceStats.value as object | null,
    doctor_tls: doctorTLS.value,
    doctor_checks: doctorChecks.value,
    readiness_total: localReadinessScore.value.total,
    readiness_level: localReadinessLevel.value,
    server_version: serverVersion.value,
    client: clientInfo as unknown as object,
    last_refreshed: lastRefreshed.value,
  }))
  if (res.ok) success(t.value('overviewCopied'))
  else notifyError(res.error || t.value('failed'))
}
</script>

<template>
  <div class="space-y-6" data-testid="overview-page">
    <div class="flex flex-wrap items-center justify-between gap-3">
      <div>
        <h1 class="mts-title">{{ t('overviewTitle') }}</h1>
        <p class="text-xs mts-muted">{{ t('overviewDesc') }}</p>
      </div>
      <div class="flex items-center gap-2">
        <span v-if="lastRefreshed" class="text-xs mts-muted">{{ t('refreshedAt') }} {{ lastRefreshed }}</span>
        <button class="mts-btn" @click="toggleAutoRefresh">
          <RefreshCw class="h-3.5 w-3.5" :class="autoRefresh ? 'animate-spin' : ''" />
          {{ autoRefresh ? t('autoRefreshing') : t('autoRefresh') }}
        </button>
        <button
          v-if="showAdminPanels"
          type="button"
          class="mts-btn"
          @click="goReadiness"
        >
          <ClipboardCheck class="h-3.5 w-3.5" />
          {{ t('readiness') }}
        </button>
        <button class="mts-btn-primary" :disabled="loading" @click="loadOverview">
          <RefreshCw class="h-3.5 w-3.5" /> {{ t('refresh') }}
        </button>
      </div>
    </div>

    <ActionResultBanner
      v-if="loadError"
      kind="error"
      :message="loadError"
      @dismiss="loadError = ''"
    />
    <ActionResultBanner
      v-else-if="adminPartialError"
      kind="warn"
      :message="`${t('partialAdminStats')}：${adminPartialError}`"
      :dismissible="false"
    />

    <div id="overview-summary" class="mts-card scroll-mt-20 p-4" data-testid="overview-summary">
      <div class="flex flex-wrap items-center gap-x-6 gap-y-2 text-sm">
        <div class="flex items-center gap-2">
          <Clock3 class="h-4 w-4 mts-muted" />
          <span class="mts-muted">{{ t('sessionExpiry') }}</span>
          <span
            class="rounded-full px-2 py-0.5 text-[11px] font-medium"
            :class="{
              'bg-emerald-50 text-emerald-800 dark:bg-emerald-950/40 dark:text-emerald-200': sessionSummary.urgency === 'ok',
              'bg-amber-100 text-amber-800 dark:bg-amber-950/50 dark:text-amber-200': sessionSummary.urgency === 'warn',
              'bg-red-100 text-red-700 dark:bg-red-950/50 dark:text-red-200': sessionSummary.urgency === 'critical' || sessionSummary.urgency === 'expired',
              'bg-slate-100 text-slate-600 dark:bg-slate-800 dark:text-slate-300': sessionSummary.urgency === 'unknown',
            }"
          >{{ sessionSummary.label || t('sessionUnknown') }}</span>
        </div>
        <div class="flex items-center gap-2">
          <Info class="h-4 w-4 mts-muted" />
          <span class="mts-muted">{{ t('aboutClient') }}</span>
          <span class="font-mono text-xs">{{ clientInfo.name }} {{ clientInfo.version }}</span>
        </div>
        <div v-if="showAdminPanels" class="flex items-center gap-2">
          <span class="mts-muted">{{ t('aboutServer') }}</span>
          <span class="font-mono text-xs">{{ serverVersion?.version || t('emptyValue') }}</span>
          <span v-if="serverVersion?.commit" class="font-mono text-[11px] mts-muted">{{ serverVersion.commit.slice(0, 8) }}</span>
        </div>
        <ExportJobBanner class="w-full basis-full" :job="exportJob" @cancel="cancelExport" @dismiss="resetExport" />
        <button type="button" class="mts-btn" data-testid="overview-export-json" :disabled="exportBusy" @click="exportOverview">
          <Download class="h-3.5 w-3.5" /> {{ t('overviewExportJSON') }}
        </button>
        <button type="button" class="mts-btn" data-testid="overview-copy-snapshot" @click="copyOverview">
          <Copy class="h-3.5 w-3.5" /> {{ t('overviewCopySnapshot') }}
        </button>
        <button type="button" class="mts-btn" data-testid="overview-share-link" @click="copyOverviewShareLink">
          {{ t('overviewShareLink') }}
        </button>
        <button type="button" class="mts-btn ml-auto" data-testid="overview-about" @click="router.push('/about')">
          <Info class="h-3.5 w-3.5" />
          {{ t('about') }}
        </button>
      </div>
    </div>

    <div
      v-if="showAdminPanels"
      id="overview-readiness"
      data-testid="overview-readiness-score"
      class="mts-card flex flex-wrap items-center justify-between gap-3 scroll-mt-20 p-4"
    >
      <div class="min-w-0">
        <p class="text-xs mts-muted">{{ t('readinessScore') }}</p>
        <p class="mt-1 text-2xl font-semibold tabular-nums" data-testid="overview-readiness-total">
          {{ localReadinessScore.total }}%
          <span class="ml-2 text-xs font-medium mts-muted">{{ readinessLevelLabel(localReadinessLevel) }}</span>
        </p>
        <p class="mt-1 text-[11px] mts-muted">
          {{ t('readinessScoreBreakdown') }}:
          {{ t('readinessRequiredChecklist') }} {{ localReadinessScore.checklist }}% ·
          {{ t('readinessEdgeHttps') }} {{ localReadinessScore.edgeHttps }}% ·
          {{ t('readinessBackupSchedule') }} {{ localReadinessScore.backupSchedule }}% ·
          {{ t('readinessScoreDoctor') }} {{ localReadinessScore.doctor }}%
        </p>
        <p class="mt-1 text-[11px] mts-muted">{{ t('overviewDeployKitHint') }}</p>
      </div>
      <div class="flex flex-wrap items-center gap-2">
        <button type="button" class="mts-btn-primary" data-testid="overview-go-readiness" @click="goReadiness">
          <ClipboardCheck class="h-3.5 w-3.5" />
          {{ t('readiness') }}
        </button>
        <button type="button" class="mts-btn" data-testid="overview-go-deploy-kit" :title="t('overviewDeployKitHint')" @click="goDeployKit">
          <FileCode2 class="h-3.5 w-3.5" />
          {{ t('overviewGoDeployKit') }}
        </button>
        <button type="button" class="mts-btn" data-testid="overview-go-signoff" :title="t('overviewSignoffHint')" @click="goSignoffNotes">
          <ClipboardCheck class="h-3.5 w-3.5" />
          {{ t('overviewGoSignoff') }}
        </button>
      </div>
      <div class="basis-full space-y-1.5" data-testid="overview-signoff-panel">
        <div class="flex flex-wrap items-center justify-between gap-2 text-[11px]">
          <span class="mts-muted" data-testid="overview-signoff-progress-label">
            {{ formatMessage(t('readinessSignoffProgress'), { percent: String(signoffProgress) }) }}
          </span>
          <span class="mts-muted">{{ t('overviewSignoffHint') }}</span>
        </div>
        <div
          class="h-1.5 overflow-hidden rounded-full bg-slate-100 dark:bg-slate-800"
          role="progressbar"
          :aria-valuenow="signoffProgress"
          aria-valuemin="0"
          aria-valuemax="100"
          data-testid="overview-signoff-progress-bar"
        >
          <div
            class="h-full rounded-full transition-[width] duration-200"
            :class="signoffCompleteness.complete ? 'bg-emerald-500' : 'bg-amber-500'"
            :style="{ width: `${signoffProgress}%` }"
            data-testid="overview-signoff-progress-fill"
          />
        </div>
        <p
          class="text-[11px]"
          data-testid="overview-signoff-completeness"
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
        <div v-if="signoffCompleteness.missing.length" class="flex flex-wrap gap-2" data-testid="overview-signoff-missing-jumps">
          <button
            v-for="field in signoffCompleteness.missing"
            :key="field"
            type="button"
            class="mts-btn !px-2 !py-0.5 text-[10px]"
            :data-testid="`overview-signoff-jump-${field}`"
            @click="router.push({ path: '/ops/readiness', hash: `#signoff-field-${field}` })"
          >{{ t('readinessSignoffJump') }}: {{ signoffFieldLabel(field, uiLocale) }}</button>
        </div>
      </div>
      <div class="basis-full flex flex-wrap items-center gap-2 text-[11px]" data-testid="overview-export-preflight">
        <span
          :class="overviewPreflight.warnCount > 0 ? 'text-amber-700 dark:text-amber-200' : 'text-emerald-700 dark:text-emerald-300'"
        >
          {{
            formatMessage(t('overviewPreflightSummary'), {
              warn: String(overviewPreflight.warnCount),
              info: String(overviewPreflight.infoCount),
              ok: String(overviewPreflight.okCount),
            })
          }}
        </span>
        <button type="button" class="mts-btn !px-2 !py-0.5 text-[11px]" data-testid="overview-go-preflight" @click="goExportPreflight">
          {{ t('overviewGoPreflight') }}
        </button>
        <span class="mts-muted">{{ t('overviewPreflightHint') }}</span>
      </div>
      <div
        v-if="showAdminPanels"
        class="basis-full rounded-lg border border-dashed border-slate-300 p-3 dark:border-slate-600"
        data-testid="overview-next-steps"
      >
        <div class="mb-1 flex flex-wrap items-center justify-between gap-2">
          <p class="text-xs font-medium text-slate-700 dark:text-slate-200">{{ t('readinessNextSteps') }}</p>
          <p class="text-[11px] mts-muted">{{ t('readinessNextStepsHint') }}</p>
        </div>
        <ul class="space-y-1.5">
          <li
            v-for="step in overviewNextSteps"
            :key="step.id"
            class="flex items-start gap-2 text-xs text-slate-700 dark:text-slate-200"
            :data-testid="`overview-next-step-${step.id}`"
          >
            <span class="min-w-0 flex-1">{{ step.message }}</span>
            <button
              v-if="step.target"
              type="button"
              class="mts-btn shrink-0 !px-2 !py-0.5 text-[11px]"
              :data-testid="`overview-next-jump-${step.id}`"
              @click="jumpNextStep(step.target)"
            >
              {{ t(step.actionKey === 'preflightJumpStorage' ? 'preflightJumpStorage' : 'preflightJumpLocal') }}
            </button>
          </li>
        </ul>
      </div>
    </div>

    <div id="overview-health" class="grid scroll-mt-20 gap-4 sm:grid-cols-2 lg:grid-cols-4" data-testid="overview-health-grid">
      <div class="mts-card p-5" data-testid="overview-connectivity">
        <div class="mb-2 flex items-center justify-between gap-2">
          <div class="flex items-center gap-2 mts-muted">
            <Activity class="h-4 w-4" aria-hidden="true" />
            <span class="text-xs">{{ t('connectivityTitle') }}</span>
          </div>
          <button
            v-if="connectivityKind === 'unreachable' || connectivityKind === 'unknown'"
            type="button"
            class="mts-btn mts-focus-ring"
            data-testid="overview-connectivity-retry"
            :disabled="reachChecking"
            @click="retryReadyz"
          >
            {{ t('serverUnreachableRetry') }}
          </button>
        </div>
        <p class="text-2xl font-semibold" :class="connectivityTone" data-testid="overview-connectivity-kind">
          {{ connectivityLabel }}
        </p>
        <p class="mt-1 text-[11px] mts-muted" data-testid="overview-connectivity-hint">{{ connectivityHint }}</p>
        <p
          v-if="showUnreachableBanner"
          class="mt-2 text-[11px] text-red-700 dark:text-red-300"
          data-testid="overview-connectivity-banner-sync"
        >
          {{ t('connectivityHintUnreachable') }}
        </p>
      </div>
      <div class="mts-card p-5">
        <div class="mb-2 flex items-center gap-2 mts-muted"><Activity class="h-4 w-4" aria-hidden="true" /><span class="text-xs">{{ t('healthy') }}</span></div>
        <p class="text-2xl font-semibold" :class="healthy ? 'text-green-600' : healthy === false ? 'text-red-600' : 'mts-muted'">
          {{ healthy === null ? t('emptyValue') : healthy ? t('healthy') : t('unhealthy') }}
        </p>
      </div>
      <div class="mts-card p-5">
        <div class="mb-2 flex items-center gap-2 mts-muted"><Activity class="h-4 w-4" aria-hidden="true" /><span class="text-xs">{{ t('ready') }}</span></div>
        <p class="text-2xl font-semibold" :class="ready ? 'text-green-600' : ready === false ? 'text-amber-600' : 'mts-muted'">
          {{ ready === null ? t('emptyValue') : ready ? t('ready') : t('notReady') }}
        </p>
      </div>
      <div class="mts-card p-5">
        <div class="mb-2 flex items-center gap-2 mts-muted"><AlertTriangle class="h-4 w-4" aria-hidden="true" /><span class="text-xs">{{ t('reasons') }}</span></div>
        <p v-if="!healthReasons.length" class="text-sm mts-muted">{{ t('emptyValue') }}</p>
        <ul v-else class="list-disc space-y-1 pl-5 text-sm text-slate-700 dark:text-slate-200">
          <li v-for="(r, i) in healthReasons" :key="i">{{ r }}</li>
        </ul>
      </div>
    </div>

    <div v-if="healthChecks.length" id="overview-health-checks" class="mts-panel scroll-mt-20" data-testid="overview-health-checks">
      <h2 class="mb-3 text-sm font-semibold text-slate-800 dark:text-slate-100">{{ t('healthChecks') }}</h2>
      <div class="overflow-hidden rounded-lg border border-slate-100 dark:border-slate-800">
        <div class="grid grid-cols-[minmax(7rem,1fr)_minmax(5rem,0.6fr)_minmax(8rem,1.2fr)] border-b border-slate-200 px-2 py-2 text-left text-[11px] uppercase mts-muted dark:border-slate-700">
          <span>{{ t('healthCheckColName') }}</span>
          <span>{{ t('healthCheckColStatus') }}</span>
          <span>{{ t('healthCheckColReason') }}</span>
        </div>
        <VirtualTable
          :items="healthChecks"
          :row-height="CHECK_ROW_HEIGHT"
          :height="Math.min(CHECK_LIST_HEIGHT, Math.max(120, healthChecks.length * CHECK_ROW_HEIGHT))"
          data-testid="overview-health-virtual-list"
        >
          <template #default="{ item: c, index }">
            <div class="grid h-full grid-cols-[minmax(7rem,1fr)_minmax(5rem,0.6fr)_minmax(8rem,1.2fr)] items-center border-b border-slate-100 px-2 text-xs dark:border-slate-800" :data-testid="`overview-health-row-${index}`">
              <span class="truncate font-mono" :title="c.name">{{ c.name }}</span>
              <span :class="healthStatusToneClass(c.status)">{{ formatHealthStatus(c.status) }}</span>
              <span class="truncate mts-muted" :title="c.reason || ''">{{ c.reason || t('emptyValue') }}</span>
            </div>
          </template>
        </VirtualTable>
        <p class="border-t border-slate-100 px-3 py-1.5 text-[11px] mts-muted dark:border-slate-800" data-testid="overview-health-virtual-hint">{{ t('overviewChecksVirtualHint') }}</p>
      </div>
    </div>


        <div v-if="showAdminPanels && doctorChecks.length" id="overview-doctor" class="mts-panel scroll-mt-20" data-testid="overview-doctor-checks">
      <div class="mb-3 flex flex-wrap items-center justify-between gap-2">
        <div class="flex items-center gap-2">
          <ShieldCheck class="h-4 w-4 text-slate-500" />
          <h2 class="text-sm font-semibold text-slate-800 dark:text-slate-100">{{ t('doctorTitle') }}</h2>
          <span class="text-xs mts-muted">({{ doctorChecks.length }})</span>
        </div>
        <span class="text-xs mts-muted">
          {{ t('doctorHttpTls') }}:
          <span :class="doctorTLS ? 'text-green-600' : 'text-amber-600'">
            {{ doctorTLS === null ? t('emptyValue') : doctorTLS ? t('enabled') : t('disabled') }}
          </span>
        </span>
      </div>
      <p class="mb-3 text-xs mts-muted">{{ t('doctorDesc') }}</p>
      <div class="overflow-hidden rounded-lg border border-slate-100 dark:border-slate-800">
        <div class="grid grid-cols-[minmax(5rem,0.6fr)_minmax(7rem,0.9fr)_minmax(8rem,1.3fr)] border-b border-slate-200 px-2 py-2 text-left text-[11px] uppercase mts-muted dark:border-slate-700">
          <span>{{ t('readinessDoctorColLevel') }}</span>
          <span>{{ t('readinessDoctorColCode') }}</span>
          <span>{{ t('readinessDoctorColMessage') }}</span>
        </div>
        <VirtualTable
          :items="doctorChecks"
          :row-height="CHECK_ROW_HEIGHT"
          :height="Math.min(CHECK_LIST_HEIGHT, Math.max(120, doctorChecks.length * CHECK_ROW_HEIGHT))"
          data-testid="overview-doctor-virtual-list"
        >
          <template #default="{ item: c, index }">
            <div class="grid h-full grid-cols-[minmax(5rem,0.6fr)_minmax(7rem,0.9fr)_minmax(8rem,1.3fr)] items-center border-b border-slate-100 px-2 text-xs dark:border-slate-800" :data-testid="`overview-doctor-row-${index}`">
              <span :class="healthStatusToneClass(c.level)">{{ formatHealthStatus(c.level) }}</span>
              <span class="truncate font-mono" :title="c.code">{{ c.code }}</span>
              <span class="truncate mts-muted" :title="c.message">{{ c.message }}</span>
            </div>
          </template>
        </VirtualTable>
        <p class="border-t border-slate-100 px-3 py-1.5 text-[11px] mts-muted dark:border-slate-800" data-testid="overview-doctor-virtual-hint">{{ t('overviewChecksVirtualHint') }}</p>
      </div>
    </div>

    <div
      v-if="!showAdminPanels"
      id="overview-workspace"
      data-testid="overview-workspace-panel"
      class="mts-card space-y-3 scroll-mt-20 bg-slate-50 p-4 text-sm text-slate-600 dark:bg-slate-900/60 dark:text-slate-300"
    >
      <p data-testid="overview-nonadmin-hint">{{ t('nonAdminOverview') }}</p>
      <p class="text-xs mts-muted">{{ t('nonAdminWorkspaceHint') }}</p>
      <div class="flex flex-wrap gap-2" data-testid="overview-workspace-links">
        <button type="button" class="mts-btn-primary" data-testid="overview-go-query" @click="router.push('/query')">
          {{ t('query') }}
        </button>
        <button type="button" class="mts-btn" data-testid="overview-go-write" @click="router.push('/write')">
          {{ t('write') }}
        </button>
        <button type="button" class="mts-btn" data-testid="overview-go-databases" @click="router.push('/databases')">
          {{ t('databases') }}
        </button>
        <button type="button" class="mts-btn" data-testid="overview-go-access" @click="router.push('/access')">
          {{ t('accessMatrix') }}
        </button>
        <button type="button" class="mts-btn" data-testid="overview-go-audit" @click="router.push('/audit')">
          {{ t('audit') }}
        </button>
        <button type="button" class="mts-btn" data-testid="overview-go-account" @click="router.push('/account')">
          {{ t('account') }}
        </button>
      </div>
    </div>

    <template v-if="showAdminPanels">
      <div class="mts-panel">
        <div class="mb-3 flex items-center gap-2">
          <AlertTriangle class="h-4 w-4 text-slate-500" />
          <h2 class="text-sm font-semibold text-slate-800 dark:text-slate-100">{{ t('maintenanceErrors') }}</h2>
          <span class="text-xs mts-muted">({{ maintenanceErrors.length }})</span>
        </div>
        <p v-if="!maintenanceErrors.length" class="text-sm mts-muted" data-testid="overview-maint-empty">{{ t('noMaintenanceErrors') }}</p>
        <div v-else class="overflow-hidden rounded-lg border border-red-100 dark:border-red-900/40" id="overview-maint" data-testid="overview-maint-errors">
          <VirtualTable
            :items="maintenanceErrors"
            :row-height="MAINT_ROW_HEIGHT"
            :height="Math.min(MAINT_LIST_HEIGHT, Math.max(108, maintenanceErrors.length * MAINT_ROW_HEIGHT))"
            data-testid="overview-maint-virtual-list"
          >
            <template #default="{ item: e, index }">
              <div class="flex h-full items-center border-b border-red-50 px-2 font-mono text-xs text-red-700 dark:border-red-950/40 dark:text-red-200" :data-testid="`overview-maint-row-${index}`">
                <span class="truncate" :title="e">{{ e }}</span>
              </div>
            </template>
          </VirtualTable>
          <p class="border-t border-red-50 px-3 py-1.5 text-[11px] mts-muted dark:border-red-950/40" data-testid="overview-maint-virtual-hint">{{ t('overviewMaintVirtualHint') }}</p>
        </div>
      </div>

      <div class="mts-panel">
        <div class="mb-4 flex items-center gap-2">
          <Cpu class="h-4 w-4 text-slate-500" />
          <h2 class="text-sm font-semibold text-slate-800 dark:text-slate-100">{{ t('memoryStats') }}</h2>
        </div>
        <div v-if="!memorySnapshot" class="text-sm mts-muted">{{ t('emptyValue') }}</div>
        <div v-else class="grid grid-cols-2 gap-3 text-xs sm:grid-cols-4">
          <div class="rounded bg-slate-50 px-3 py-2 dark:bg-slate-800/50" v-for="(v, k) in memorySnapshot" :key="String(k)">
            <span class="mts-muted">{{ memoryLabel(String(k)) }}</span>
            <p class="font-semibold text-slate-800 dark:text-slate-100">{{ typeof v === 'number' && String(k).includes('bytes') ? formatBytes(v) : v }}</p>
          </div>
        </div>
      </div>

      <div class="mts-panel">
        <div class="mb-4 flex items-center gap-2">
          <Layers class="h-4 w-4 text-slate-500" />
          <h2 class="text-sm font-semibold text-slate-800 dark:text-slate-100">{{ t('compactionStats') }}</h2>
        </div>
        <div v-if="!compactionStats" class="text-sm mts-muted">{{ t('emptyValue') }}</div>
        <div v-else class="grid grid-cols-2 gap-4 sm:grid-cols-3 lg:grid-cols-5">
          <div><span class="text-xs mts-muted">{{ t('opsStatTotal') }}</span><p class="text-sm font-medium">{{ compactionStats.total }}</p></div>
          <div><span class="text-xs mts-muted">{{ t('opsStatSuccess') }}</span><p class="text-sm font-medium text-green-600">{{ compactionStats.success }}</p></div>
          <div><span class="text-xs mts-muted">{{ t('opsStatFailure') }}</span><p class="text-sm font-medium text-red-600">{{ compactionStats.failure }}</p></div>
          <div><span class="text-xs mts-muted">{{ t('opsStatBacklog') }}</span><p class="text-sm font-medium text-yellow-600">{{ compactionStats.backlog }}</p></div>
          <div><span class="text-xs mts-muted">{{ t('opsStatActive') }}</span><p class="text-sm font-medium text-blue-600">{{ compactionStats.active }}</p></div>
        </div>
        <p v-if="compactionStats?.last_error" class="mt-3 text-xs text-red-600 dark:text-red-300">{{ compactionStats.last_error }}</p>
      </div>

      <div class="mts-panel">
        <div class="mb-4 flex items-center gap-2">
          <Wrench class="h-4 w-4 text-slate-500" />
          <h2 class="text-sm font-semibold text-slate-800 dark:text-slate-100">{{ t('maintenanceStats') }}</h2>
        </div>
        <div v-if="!maintenanceStats" class="text-sm mts-muted">{{ t('emptyValue') }}</div>
        <div v-else class="grid grid-cols-2 gap-3 text-xs sm:grid-cols-3 lg:grid-cols-4">
          <div class="rounded bg-slate-50 px-3 py-2 dark:bg-slate-800/50">{{ t('opsStatCompactActive') }}: <span class="font-semibold">{{ maintenanceStats.compaction_active }}</span></div>
          <div class="rounded bg-slate-50 px-3 py-2 dark:bg-slate-800/50">{{ t('opsStatCompactBacklog') }}: <span class="font-semibold">{{ maintenanceStats.compaction_backlog }}</span></div>
          <div class="rounded bg-slate-50 px-3 py-2 dark:bg-slate-800/50">{{ t('opsStatCompactSkipped') }}: <span class="font-semibold">{{ maintenanceStats.compaction_skipped }}</span></div>
          <div class="rounded bg-slate-50 px-3 py-2 dark:bg-slate-800/50">{{ t('opsStatCompactFailure') }}: <span class="font-semibold">{{ maintenanceStats.compaction_failure }}</span></div>
          <div class="rounded bg-slate-50 px-3 py-2 dark:bg-slate-800/50">{{ t('opsStatDownsampleActive') }}: <span class="font-semibold">{{ maintenanceStats.downsample_active }}</span></div>
          <div class="rounded bg-slate-50 px-3 py-2 dark:bg-slate-800/50">{{ t('opsStatDownsampleInflight') }}: <span class="font-semibold">{{ maintenanceStats.downsample_inflight }}</span></div>
          <div class="rounded bg-slate-50 px-3 py-2 dark:bg-slate-800/50">{{ t('opsStatDownsampleFailure') }}: <span class="font-semibold">{{ maintenanceStats.downsample_failure }}</span></div>
          <div class="rounded bg-slate-50 px-3 py-2 dark:bg-slate-800/50">{{ t('opsStatErrors') }}: <span class="font-semibold">{{ maintenanceStats.maintenance_error_count }}</span></div>
        </div>
        <p v-if="maintenanceStats?.compaction_last_skip" class="mt-3 text-xs text-amber-700 dark:text-amber-200">{{ maintenanceStats.compaction_last_skip }}</p>
      </div>
    </template>
  </div>
</template>
