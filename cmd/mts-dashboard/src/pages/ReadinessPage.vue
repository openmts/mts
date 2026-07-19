<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { apiGet } from '@/api/client'
import { useAuth } from '@/composables/useAuth'
import { useI18n } from '@/composables/useI18n'
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
import { ClipboardCheck, ExternalLink, RefreshCw, ShieldCheck, HardDrive, FileCode2 } from 'lucide-vue-next'

interface DoctorCheck { level: string; code: string; message: string }
interface DoctorResponse {
  ok: boolean
  http_tls_enabled?: boolean
  checks?: DoctorCheck[]
  lines?: string[]
}

const { isAdmin } = useAuth()
const { t } = useI18n()
const router = useRouter()

const state = ref<ReadinessState>(loadReadinessState())
const doctor = ref<DoctorResponse | null>(null)
const doctorError = ref('')
const loadingDoctor = ref(false)

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

const readinessScore = computed(() => {
  const parts = [
    requiredItems.value.length === 0 ? 1 : requiredDoneCount.value / requiredItems.value.length,
    edgeStats.value.requiredTotal === 0 ? 1 : edgeStats.value.requiredDone / edgeStats.value.requiredTotal,
    scheduleStats.value.requiredTotal === 0
      ? 1
      : scheduleStats.value.requiredDone / scheduleStats.value.requiredTotal,
  ]
  return Math.round((parts.reduce((a, b) => a + b, 0) / parts.length) * 100)
})

function toggle(
  section: 'production' | 'edgeHttps' | 'backupSchedule',
  id: string,
  checked: boolean,
) {
  state.value = setReadinessFlag(section, id, checked)
}

async function loadDoctor() {
  loadingDoctor.value = true
  doctorError.value = ''
  try {
    doctor.value = await apiGet<DoctorResponse>('/api/v1/admin/doctor')
  } catch (e) {
    doctor.value = null
    doctorError.value = e instanceof Error ? e.message : 'doctor load failed'
  } finally {
    loadingDoctor.value = false
  }
}

function go(path: string) {
  void router.push(path)
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
  if (isAdmin.value) void loadDoctor()
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
      <div class="flex items-center gap-2">
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

    <ActionResultBanner
      v-if="doctorError"
      kind="error"
      :message="doctorError"
      @dismiss="doctorError = ''"
    />

    <div class="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
      <div class="mts-card p-4">
        <p class="text-xs mts-muted">{{ t('readinessScore') }}</p>
        <p class="mt-1 text-3xl font-semibold" :class="readinessScore >= 80 ? 'text-green-600' : readinessScore >= 50 ? 'text-amber-600' : 'text-red-600'">
          {{ readinessScore }}%
        </p>
      </div>
      <div class="mts-card p-4">
        <p class="text-xs mts-muted">{{ t('readinessRequiredChecklist') }}</p>
        <p class="mt-1 text-lg font-semibold">{{ requiredDoneCount }}/{{ requiredItems.length }}</p>
        <p class="text-[11px] mts-muted">
          自动覆盖 {{ productionCoverage.automated }}/{{ productionCoverage.total }}
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
        <div class="flex items-center gap-2">
          <ShieldCheck class="h-4 w-4 text-slate-500" />
          <h2 class="text-sm font-semibold">{{ t('doctorTitle') }}</h2>
        </div>
        <span class="text-xs mts-muted">
          HTTP TLS:
          <span :class="doctor?.http_tls_enabled ? 'text-green-600' : 'text-amber-600'">
            {{ doctor == null ? '—' : doctor.http_tls_enabled ? t('enabled') : t('disabled') }}
          </span>
          · ok={{ doctor?.ok ?? '—' }} · warn={{ doctorWarns.length }} · ok_rows={{ doctorOKs.length }}
        </span>
      </div>
      <p v-if="!doctor && !doctorError" class="text-sm mts-muted">—</p>
      <div v-else-if="doctor" class="overflow-x-auto">
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
            :checked="!!state.production[item.id]"
            @change="toggle('production', item.id, ($event.target as HTMLInputElement).checked)"
          />
          <div class="min-w-0">
            <p class="font-medium text-slate-800 dark:text-slate-100">
              {{ item.title }}
              <span class="ml-1 text-[11px] font-normal mts-muted">{{ item.severity === 'required' ? t('required') : t('recommended') }}</span>
              <span v-if="item.automated" class="ml-1 text-[11px] font-normal text-emerald-700 dark:text-emerald-300">{{ t('partialAuto') }}</span>
            </p>
            <p class="text-xs mts-muted">{{ item.detail }}</p>
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
            :checked="!!state.edgeHttps[step.id]"
            @change="toggle('edgeHttps', step.id, ($event.target as HTMLInputElement).checked)"
          />
          <div class="min-w-0">
            <p class="font-medium text-slate-800 dark:text-slate-100">
              {{ step.title }}
              <span class="ml-1 text-[11px] font-normal mts-muted">{{ step.severity === 'required' ? t('required') : t('recommended') }}</span>
            </p>
            <p class="text-xs mts-muted">{{ step.detail }}</p>
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
            :checked="!!state.backupSchedule[step.id]"
            @change="toggle('backupSchedule', step.id, ($event.target as HTMLInputElement).checked)"
          />
          <div class="min-w-0 flex-1">
            <p class="font-medium text-slate-800 dark:text-slate-100">
              {{ step.title }}
              <span class="ml-1 text-[11px] font-normal mts-muted">{{ step.severity === 'required' ? t('required') : t('recommended') }}</span>
            </p>
            <p class="text-xs mts-muted">{{ step.detail }}</p>
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
