<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { apiGet } from '@/api/client'
import { formatCaughtError } from '@/utils/apiError'
import { useAuth } from '@/composables/useAuth'
import { useI18n } from '@/composables/useI18n'
import { formatMessage } from '@/utils/formatMessage'
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
  type ReadinessState,
} from '@/utils/readinessState'
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
const router = useRouter()

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

function flash(kind: 'ok' | 'error' | 'info', message: string) {
  actionKind.value = kind
  actionMsg.value = message
}

function toggle(
  section: 'production' | 'edgeHttps' | 'backupSchedule',
  id: string,
  checked: boolean,
) {
  state.value = setReadinessFlag(section, id, checked)
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

function downloadArchive() {
  const archive = buildReadinessArchive({
    operator: currentUser.value || 'admin',
    state: state.value,
    score: scoreBreakdown.value,
    doctor: doctorArchiveSummary(),
    locale: uiLocale.value,
  })
  const names = archiveFilenames()
  downloadJSON(names.json, archive)
  downloadText(names.md, formatReadinessArchiveMarkdown(archive), 'text/markdown')
  flash('ok', t.value('readinessArchiveOk'))
}

function downloadAcceptancePack() {
  const archive = buildReadinessArchive({
    operator: currentUser.value || 'admin',
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
    locale: uiLocale.value,
  })
  const names = acceptancePackFilenames()
  downloadJSON(names.json, pack)
  downloadText(names.md, formatAcceptancePackMarkdown(pack), 'text/markdown')
  flash('ok', t.value('readinessAcceptancePackOk'))
}

const quickActions = [
  { id: 'data-restore', labelKey: 'readinessGoDataRestore', path: '/storage#data-restore' },
  { id: 'backup-drill', labelKey: 'readinessGoBackupDrill', path: '/storage#backup-drill' },
  { id: 'edge-https', labelKey: 'readinessGoEdgeHttps', path: '/storage#edge-https' },
] as const

const backupScriptHint = `export MTS_BASE_URL='https://mts.example.com'
export MTS_ADMIN_TOKEN='***'
export MTS_BACKUP_REMOTE='backup@host:/var/backups/mts'
./scripts/mts-backup.sh --dry-run
./scripts/mts-backup.sh`

onMounted(() => {
  if (isAdmin.value) {
    void loadDoctor()
    void loadServerVersion()
  }
})
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
        </p>
        <p class="mt-2 text-[11px] mts-muted">
          {{ t('readinessScoreBreakdown') }}:
          {{ t('readinessRequiredChecklist') }} {{ scoreBreakdown.checklist }}% ·
          {{ t('readinessEdgeHttps') }} {{ scoreBreakdown.edgeHttps }}% ·
          {{ t('readinessBackupSchedule') }} {{ scoreBreakdown.backupSchedule }}% ·
          Doctor {{ scoreBreakdown.doctor }}%
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
      <div class="mt-4">
        <p class="mb-1 flex items-center gap-1 text-xs font-medium text-slate-700 dark:text-slate-200">
          <FileCode2 class="h-3.5 w-3.5" />
          {{ t('readinessBackupScript') }}
        </p>
        <p class="mb-2 text-xs mts-muted">{{ t('readinessBackupScriptHint') }}</p>
        <pre class="overflow-x-auto rounded bg-slate-950 p-3 text-[11px] text-emerald-300">{{ backupScriptHint }}</pre>
      </div>
    </div>

    <div class="mts-panel">
      <div class="mb-3 flex flex-wrap items-center justify-between gap-2">
        <h2 class="flex items-center gap-2 text-sm font-semibold">
          <ShieldCheck class="h-4 w-4" />
          Doctor
        </h2>
        <span class="text-xs mts-muted">
          ok={{ doctor?.ok ?? '—' }} · warn={{ doctorWarns.length }} · tls={{ doctor?.http_tls_enabled ?? '—' }}
        </span>
      </div>
      <div v-if="doctor" class="overflow-x-auto">
        <table class="w-full text-sm">
          <thead>
            <tr class="border-b border-slate-200 text-left text-[11px] uppercase mts-muted dark:border-slate-700">
              <th class="px-2 py-2">Level</th>
              <th class="px-2 py-2">Code</th>
              <th class="px-2 py-2">Message</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="(c, i) in doctor.checks ?? []" :key="i" class="border-b border-slate-100 dark:border-slate-800">
              <td class="px-2 py-2 text-xs" :class="c.level === 'ok' ? 'text-green-600' : 'text-amber-600'">{{ c.level }}</td>
              <td class="px-2 py-2 font-mono text-xs">{{ c.code }}</td>
              <td class="px-2 py-2 text-xs mts-muted">{{ c.message }}</td>
            </tr>
          </tbody>
        </table>
      </div>
      <p v-else class="text-xs mts-muted">{{ loadingDoctor ? t('loading') : '—' }}</p>
      <p v-if="doctorOKs.length" class="mt-2 text-[11px] mts-muted">ok checks: {{ doctorOKs.length }}</p>
    </div>

    <div class="mts-card p-4">
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

    <div class="mts-card p-4">
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

    <div class="mts-card p-4">
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
