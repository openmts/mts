<script setup lang="ts">
import { computed, onMounted, onBeforeUnmount, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { apiGet } from '@/api/client'
import { formatCaughtError } from '@/utils/apiError'
import { useAuth } from '@/composables/useAuth'
import { useNotify } from '@/composables/useNotify'
import { useI18n } from '@/composables/useI18n'
import { formatMessage } from '@/utils/formatMessage'
import { healthStatusLabel, healthStatusToneClass } from '@/utils/healthStatusLabel'
import { scheduleScrollToHash } from '@/utils/hashScroll'
import { buildExportPreflight, formatExportPreflightText } from '@/utils/exportPreflight'
import { buildOpsNextSteps } from '@/utils/opsNextSteps'
import { copyText } from '@/utils/clipboard'
import {
  DEPLOY_TEMPLATES,
  deployKitFilename,
  formatDeployKitMarkdown,
  formatDeployTemplateLabel,
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
import ActionResultBanner from '@/components/ActionResultBanner.vue'
import {
  PRODUCTION_CHECKLIST,
  automatedCoverage,
  requiredChecklist,
} from '@/utils/productionChecklist'
import { EDGE_HTTPS_ACCEPTANCE_STEPS, edgeHttpsProgress } from '@/utils/edgeHttpsAcceptance'
import { BACKUP_SCHEDULE_STEPS, backupScheduleProgress } from '@/utils/backupSchedule'
import {
  completedIds,
  loadReadinessState,
  setReadinessFlag,
  setSignoffNote,
  type ReadinessState,
  type SignoffNotes,
} from '@/utils/readinessState'
import {
  assessSignoffCompleteness,
  composeSignoffArchiveNote,
  confirmExportWithMissingSignoff,
  signoffFieldLabel,
} from '@/utils/signoffExport'
import { computeReadinessScore, readinessLevel } from '@/utils/readinessScore'
import {
  buildReadinessExport,
  downloadJSON,
  downloadText,
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
}

interface VersionResponse {
  version: string
  commit: string
  built_at: string
}

const { isAdmin, currentUser } = useAuth()
const { t, locale } = useI18n()
const uiLocale = computed<LocaleCode>(() => (locale.value === 'en' ? 'en' : 'zh'))

function formatDoctorLevel(level?: string) {
  return healthStatusLabel(level, uiLocale.value === 'en' ? 'en' : 'zh')
}
const router = useRouter()
const route = useRoute()
const { success, error: notifyError } = useNotify()

const state = ref<ReadinessState>(loadReadinessState())
const doctor = ref<DoctorResponse | null>(null)
const doctorError = ref('')
const loadingDoctor = ref(false)
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
    doctorLoaded: doctor.value != null && !doctorError.value,
    doctorOk: doctor.value?.ok,
    doctorWarnCount: doctorWarns.value.length,
    httpTlsEnabled: doctor.value == null ? null : !!doctor.value.http_tls_enabled,
  })
})
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
    doctorLoaded: doctor.value != null && !doctorError.value,
    doctorOk: doctor.value?.ok,
    doctorWarnCount: doctorWarns.value.length,
    httpTlsEnabled: doctor.value == null ? null : !!doctor.value.http_tls_enabled,
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

async function loadServerVersion() {
  try {
    serverVersion.value = await apiGet<VersionResponse>('/api/v1/admin/version')
  } catch {
    serverVersion.value = null
  }
}

async function loadDoctor() {
  loadingDoctor.value = true
  doctorError.value = ''
  try {
    doctor.value = await apiGet<DoctorResponse>('/api/v1/admin/doctor')
  } catch (e) {
    doctor.value = null
    doctorError.value = formatCaughtError(e)
  } finally {
    loadingDoctor.value = false
  }
}

function go(path: string) {
  void router.push(path)
}

function exportState() {
  const payload = buildReadinessExport(state.value)
  const stamp = new Date().toISOString().replace(/[:.]/g, '-').slice(0, 19)
  downloadJSON(`mts-readiness-${stamp}.json`, payload)
  flash('ok', t.value('readinessExportOk'))
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
    flash('ok', t.value('readinessImportOk'))
  } catch (e) {
    flash('error', `${t.value('readinessImportFail')}: ${e instanceof Error ? e.message : String(e)}`)
  }
}

function doctorArchiveSummary() {
  return {
    loaded: doctor.value != null && !doctorError.value,
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

function downloadArchive() {
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
  downloadJSON(names.json, archive)
  downloadText(names.md, formatReadinessArchiveMarkdown(archive), 'text/markdown')
  const preflightToast = formatMessage(t.value('readinessExportPreflightToast'), {
    warn: exportPreflight.value.warnCount,
    info: exportPreflight.value.infoCount,
    ok: exportPreflight.value.okCount,
  })
  if (!signoffCompleteness.value.complete) {
    flash('info', `${t.value('readinessArchiveOkWithGaps')} · ${preflightToast}`)
    success(`${t.value('readinessArchiveOkWithGaps')} · ${preflightToast}`)
  } else {
    flash('ok', `${t.value('readinessArchiveOk')} · ${preflightToast}`)
    success(`${t.value('readinessArchiveOk')} · ${preflightToast}`)
  }
}

function downloadAcceptancePack() {
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
  downloadJSON(names.json, pack)
  downloadText(names.md, formatAcceptancePackMarkdown(pack), 'text/markdown')
  const preflightToast = formatMessage(t.value('readinessExportPreflightToast'), {
    warn: exportPreflight.value.warnCount,
    info: exportPreflight.value.infoCount,
    ok: exportPreflight.value.okCount,
  })
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

function downloadDeployKit() {
  const md = formatDeployKitMarkdown(uiLocale.value)
  downloadText(deployKitFilename(), md, 'text/markdown')
  markDeployKitHint('downloaded')
  markDeployKitHint('reviewed')
  success(t.value('readinessDeployKitDownloaded'))
  flash('ok', t.value('readinessDeployKitDownloaded'))
}

const deployDrillSteps = DEPLOY_DRILL_STEPS
const deployDrillAreas: DeployDrillArea[] = ['edge_https', 'scheduler', 'offsite_backup']

function drillStepsForArea(area: DeployDrillArea) {
  return deployDrillSteps.filter((s) => s.area === area)
}

function areaLabel(area: DeployDrillArea) {
  return formatDeployDrillAreaLabel(area, uiLocale.value)
}

function downloadDeployDrill() {
  const md = formatDeployRunbookDrillMarkdown(uiLocale.value)
  downloadText(deployRunbookDrillFilename(), md, 'text/markdown')
  markDeployKitHint('reviewed')
  success(t.value('readinessDeployDrillDownloaded'))
  flash('ok', t.value('readinessDeployDrillDownloaded'))
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
</script>

<template>
  <PermissionDenied v-if="!isAdmin" />
  <div v-else class="space-y-6">
    <div class="flex flex-wrap items-start justify-between gap-3">
      <div>
        <h1 class="mts-title flex items-center gap-2">
          <ClipboardCheck class="h-5 w-5" />
          {{ t('readinessTitle') }}
        </h1>
        <p class="text-xs mts-muted">{{ t('readinessDesc') }}</p>
        <p v-if="state.updatedAt" class="mt-1 text-[11px] mts-muted">
          {{ t('readinessUpdatedAt') }} {{ new Date(state.updatedAt).toLocaleString() }}
        </p>
      </div>
      <div class="flex flex-wrap items-center gap-2">
        <button type="button" class="mts-btn" data-testid="readiness-export" @click="exportState">
          <Download class="h-3.5 w-3.5" />
          {{ t('readinessExport') }}
        </button>
        <button type="button" class="mts-btn" data-testid="readiness-import" @click="openImport">
          <Upload class="h-3.5 w-3.5" />
          {{ t('readinessImport') }}
        </button>
        <button type="button" class="mts-btn" data-testid="readiness-archive" @click="downloadArchive">
          <FileCode2 class="h-3.5 w-3.5" />
          {{ t('readinessArchive') }}
        </button>
        <button type="button" class="mts-btn" data-testid="readiness-acceptance-pack" @click="downloadAcceptancePack">
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
      v-if="doctorError"
      kind="error"
      :message="doctorError"
      @dismiss="doctorError = ''"
    />
    <ActionResultBanner
      v-if="actionMsg"
      :kind="actionKind"
      :message="actionMsg"
      data-testid="readiness-action"
      @dismiss="actionMsg = ''"
    />

    <div class="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
      <div class="mts-card p-4">
        <p class="text-xs mts-muted">{{ t('readinessScore') }}</p>
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
        <p v-if="scoreBreakdown.reasons.length" class="mt-1 text-[11px] text-amber-700 dark:text-amber-200">
          {{ t('readinessScoreReasons') }}: {{ scoreBreakdown.reasons.join(', ') }}
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
                class="mt-3 space-y-2 rounded-lg border border-slate-200 bg-white/70 p-3 dark:border-slate-700 dark:bg-slate-950/40"
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
                      data-testid="deploy-drill-download"
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
            data-testid="readiness-deploy-kit-download"
            @click="downloadDeployKit"
          >
            <Download class="h-3.5 w-3.5" />
            {{ t('readinessDeployKitDownload') }}
          </button>
        </div>
        <div
          v-for="tpl in deployTemplates"
          :key="tpl.id"
          class="rounded-lg border border-slate-200 p-3 dark:border-slate-700"
          :data-testid="`deploy-tpl-${tpl.id}`"
        >
          <div class="mb-2 flex flex-wrap items-start justify-between gap-2">
            <div class="min-w-0">
              <p class="text-xs font-medium text-slate-800 dark:text-slate-100">{{ formatDeployTemplateLabel(tpl, uiLocale) }}</p>
              <p class="text-[11px] mts-muted">{{ textForLocale(tpl.description, uiLocale) }} · <span class="font-mono">{{ tpl.filename }}</span></p>
            </div>
            <button
              type="button"
              class="mts-btn"
              :data-testid="`deploy-copy-${tpl.id}`"
              @click="copyDeployBody(tpl.body)"
            >{{ t('readinessCopy') }}</button>
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
      </div>
      <div class="space-y-3">
        <label class="block text-xs text-slate-700 dark:text-slate-200">
          <span class="mb-1 block font-medium">{{ t('readinessSignoffEdge') }}</span>
          <textarea
            data-testid="signoff-edge-https"
            class="mts-input min-h-[4.5rem] w-full text-xs"
            :value="state.signoffNotes?.edgeHttps ?? ''"
            :placeholder="t('readinessSignoffEdgePh')"
            maxlength="2000"
            @change="saveSignoff('edgeHttps', ($event.target as HTMLTextAreaElement).value)"
          />
        </label>
        <label class="block text-xs text-slate-700 dark:text-slate-200">
          <span class="mb-1 block font-medium">{{ t('readinessSignoffBackup') }}</span>
          <textarea
            data-testid="signoff-backup-offsite"
            class="mts-input min-h-[4.5rem] w-full text-xs"
            :value="state.signoffNotes?.backupOffsite ?? ''"
            :placeholder="t('readinessSignoffBackupPh')"
            maxlength="2000"
            @change="saveSignoff('backupOffsite', ($event.target as HTMLTextAreaElement).value)"
          />
        </label>
        <label class="block text-xs text-slate-700 dark:text-slate-200">
          <span class="mb-1 block font-medium">{{ t('readinessSignoffAlert') }}</span>
          <textarea
            data-testid="signoff-backup-alert"
            class="mts-input min-h-[4.5rem] w-full text-xs"
            :value="state.signoffNotes?.backupAlert ?? ''"
            :placeholder="t('readinessSignoffAlertPh')"
            maxlength="2000"
            @change="saveSignoff('backupAlert', ($event.target as HTMLTextAreaElement).value)"
          />
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
      <div v-if="doctor" class="overflow-x-auto">
        <table class="w-full text-sm">
          <thead>
            <tr class="border-b border-slate-200 text-left text-[11px] uppercase mts-muted dark:border-slate-700">
              <th class="px-2 py-2">{{ t('readinessDoctorColLevel') }}</th>
              <th class="px-2 py-2">{{ t('readinessDoctorColCode') }}</th>
              <th class="px-2 py-2">{{ t('readinessDoctorColMessage') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="(c, i) in doctor.checks ?? []" :key="i" class="border-b border-slate-100 dark:border-slate-800">
              <td class="px-2 py-2 text-xs" :class="healthStatusToneClass(c.level)">{{ formatDoctorLevel(c.level) }}</td>
              <td class="px-2 py-2 font-mono text-xs">{{ c.code }}</td>
              <td class="px-2 py-2 text-xs mts-muted">{{ c.message }}</td>
            </tr>
          </tbody>
        </table>
      </div>
      <p v-else class="text-xs mts-muted">{{ loadingDoctor ? t('loading') : t('emptyValue') }}</p>
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
