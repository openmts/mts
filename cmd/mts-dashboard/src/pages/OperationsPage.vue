<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount, watch } from 'vue'
import { useRoute } from 'vue-router'
import { apiGet, apiPost } from '@/api/client'
import { useAuth } from '@/composables/useAuth'
import { useI18n } from '@/composables/useI18n'
import PermissionDenied from '@/components/PermissionDenied.vue'
import ConfirmDialog from '@/components/ConfirmDialog.vue'
import ActionResultBanner from '@/components/ActionResultBanner.vue'
import EmptyState from '@/components/EmptyState.vue'
import { useNotify } from '@/composables/useNotify'
import { formatCaughtError } from '@/utils/apiError'
import { makeActionResult, type ActionResult } from '@/utils/actionResult'
import {
  appendOpsAction,
  buildOpsActionExport,
  clearOpsActionLog,
  loadOpsActionLog,
  saveOpsActionLog,
  type OpsActionEntry,
  type OpsActionKind,
} from '@/utils/opsActionLog'
import { downloadJSON, stampFilename } from '@/utils/download'
import { copyText } from '@/utils/clipboard'
import { buildMaintenanceErrorsExport, buildOpsStatsExport, formatOpsStatsPretty, maintenanceErrorsToText } from '@/utils/opsExport'
import { RefreshCw, DatabaseBackup, Layers, Timer, AlertTriangle, Download, Eraser, Copy } from 'lucide-vue-next'
import type { CompactionStats, MaintenanceStats } from '@/api/types'
import { scheduleScrollToHash } from '@/utils/hashScroll'

interface CompactionStatsResponse { stats: CompactionStats }
interface MaintenanceStatsResponse { stats: MaintenanceStats }
interface MaintenanceErrorsResponse { errors: string[] }

const route = useRoute()
const { isAdmin } = useAuth()
const { t } = useI18n()
const { success, error: notifyError, warn, info } = useNotify()
const loadError = ref('')
const actionResult = ref<ActionResult | null>(null)
const loading = ref(false)
const maintenanceStats = ref<MaintenanceStats | null>(null)
const compactionStats = ref<CompactionStats | null>(null)
const maintenanceErrors = ref<string[]>([])
const confirmKind = ref<'flush' | 'compact' | 'retention' | null>(null)
const confirmLoading = ref(false)
const actionLog = ref<OpsActionEntry[]>(loadOpsActionLog())

const confirmTitle = computed(() => ({
  flush: t.value('opsConfirmFlush'),
  compact: t.value('opsConfirmCompact'),
  retention: t.value('opsConfirmRetention'),
}))

const confirmMessage = computed(() => ({
  flush: t.value('opsMsgFlush'),
  compact: t.value('opsMsgCompact'),
  retention: t.value('opsMsgRetention'),
}))

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

async function loadStats() {
  if (!isAdmin.value) return
  loading.value = true
  loadError.value = ''
  try {
    const results = await Promise.allSettled([
      apiGet<MaintenanceStatsResponse>('/api/v1/admin/stats/maintenance'),
      apiGet<CompactionStatsResponse>('/api/v1/admin/stats/compaction'),
      apiGet<MaintenanceErrorsResponse>('/api/v1/admin/maintenance/errors'),
    ])
    if (results[0].status === 'fulfilled') maintenanceStats.value = results[0].value.stats ?? null
    if (results[1].status === 'fulfilled') compactionStats.value = results[1].value.stats
    if (results[2].status === 'fulfilled') maintenanceErrors.value = results[2].value.errors ?? []
    else maintenanceErrors.value = []
    const errs = results.filter((r) => r.status === 'rejected') as PromiseRejectedResult[]
    if (errs.length && results.every((r) => r.status === 'rejected')) {
      loadError.value = formatCaughtError(errs[0].reason)
    }
  } catch (e) {
    loadError.value = formatCaughtError(e)
  } finally {
    loading.value = false
  }
}

function openConfirm(kind: 'flush' | 'compact' | 'retention') {
  confirmKind.value = kind
}

async function runConfirmed() {
  if (!confirmKind.value) return
  const kind = confirmKind.value
  confirmLoading.value = true
  actionResult.value = null
  try {
    let msg = ''
    if (kind === 'flush') {
      await apiPost('/api/v1/admin/flush', {})
      msg = t.value('opsFlushDone')
    } else if (kind === 'compact') {
      await apiPost('/api/v1/admin/compact', {})
      msg = t.value('opsCompactDone')
    } else {
      await apiPost('/api/v1/admin/retention/apply', {})
      msg = t.value('opsRetentionDone')
    }
    actionResult.value = makeActionResult('ok', msg)
    success(msg)
    recordAction(kind, 'ok', msg)
    confirmKind.value = null
    await loadStats()
  } catch (e) {
    const msg = formatCaughtError(e)
    actionResult.value = makeActionResult('error', msg)
    notifyError(msg)
    recordAction(kind, 'error', msg)
  } finally {
    confirmLoading.value = false
  }
}


function exportStats() {
  downloadJSON(
    stampFilename('mts-ops-stats', 'json'),
    buildOpsStatsExport({
      maintenance: maintenanceStats.value,
      compaction: compactionStats.value,
      maintenance_errors: maintenanceErrors.value,
    }),
  )
  success(t.value('opsStatsExported'))
}

async function copyStats() {
  const res = await copyText(
    formatOpsStatsPretty({
      maintenance: maintenanceStats.value,
      compaction: compactionStats.value,
      maintenance_errors: maintenanceErrors.value,
    }),
  )
  if (res.ok) success(t.value('opsStatsCopied'))
  else notifyError(res.error || t.value('failed'))
}

function exportMaintErrors() {
  if (!maintenanceErrors.value.length) {
    warn(t.value('opsMaintErrorsEmpty'))
    return
  }
  downloadJSON(
    stampFilename('mts-ops-maintenance-errors', 'json'),
    buildMaintenanceErrorsExport(maintenanceErrors.value),
  )
  success(t.value('opsMaintErrorsExported'))
}

async function copyMaintErrors() {
  if (!maintenanceErrors.value.length) {
    warn(t.value('opsMaintErrorsEmpty'))
    return
  }
  const res = await copyText(maintenanceErrorsToText(maintenanceErrors.value))
  if (res.ok) success(t.value('opsMaintErrorsCopied'))
  else notifyError(res.error || t.value('failed'))
}

function exportActionLog() {
  if (!actionLog.value.length) {
    warn(t.value('opsLogExportEmpty'))
    return
  }
  downloadJSON(stampFilename('mts-ops-actions', 'json'), buildOpsActionExport(actionLog.value))
  success(t.value('opsLogExportOk'))
}

function clearLog() {
  actionLog.value = []
  clearOpsActionLog()
  info(t.value('opsLogCleared'))
}

function formatAt(at: number): string {
  try {
    return new Date(at).toLocaleString()
  } catch {
    return String(at)
  }
}

function scrollToCurrentHash() {
  scheduleScrollToHash(typeof window !== 'undefined' ? window.location.hash : route.hash, typeof document !== 'undefined' ? document : null)
}

onMounted(() => {
  void loadStats()
  scrollToCurrentHash()
  if (typeof window !== 'undefined') window.addEventListener('hashchange', scrollToCurrentHash)
})
onBeforeUnmount(() => {
  if (typeof window !== 'undefined') window.removeEventListener('hashchange', scrollToCurrentHash)
})
watch(() => route.hash, () => scrollToCurrentHash())
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
        <button type="button" class="mts-btn" :disabled="loading" data-testid="ops-refresh" @click="loadStats">
          <RefreshCw class="h-3.5 w-3.5" /> {{ t('refresh') }}
        </button>
        <button type="button" class="mts-btn" data-testid="ops-export-stats" @click="exportStats">
          <Download class="h-3.5 w-3.5" /> {{ t('opsExportStats') }}
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
      @dismiss="loadError = ''"
    />
    <ActionResultBanner
      :result="actionResult"
      @dismiss="actionResult = null"
    />

    <div id="ops-maint-errors" class="mts-panel scroll-mt-20" data-testid="ops-maint-errors-panel">
      <div class="mb-2 flex flex-wrap items-center justify-between gap-2">
        <div class="flex items-center gap-2 text-sm font-semibold text-slate-800 dark:text-slate-100">
          <AlertTriangle class="h-4 w-4" /> {{ t('maintenanceErrors') }} ({{ maintenanceErrors.length }})
        </div>
        <div class="flex flex-wrap gap-2">
          <button type="button" class="mts-btn" data-testid="ops-export-maint-errors" :disabled="!maintenanceErrors.length" @click="exportMaintErrors">
            <Download class="h-3.5 w-3.5" /> {{ t('opsExportMaintErrors') }}
          </button>
          <button type="button" class="mts-btn" data-testid="ops-copy-maint-errors" :disabled="!maintenanceErrors.length" @click="copyMaintErrors">
            <Copy class="h-3.5 w-3.5" /> {{ t('opsCopyMaintErrors') }}
          </button>
        </div>
      </div>
      <EmptyState
        v-if="!maintenanceErrors.length"
        compact
        :title="t('noMaintenanceErrors')"
        :description="t('opsNoMaintErrorsDesc')"
      />
      <ul v-else class="max-h-40 space-y-1 overflow-auto text-xs text-red-700 dark:text-red-200" data-testid="ops-maint-errors">
        <li v-for="(e, i) in maintenanceErrors" :key="i" class="rounded bg-red-50 px-2 py-1 dark:bg-red-950/40">{{ e }}</li>
      </ul>
    </div>

    <div id="ops-actions" class="grid gap-4 scroll-mt-20 sm:grid-cols-3">
      <button type="button" id="ops-flush" class="mts-card p-5 text-left hover:border-slate-300 dark:hover:border-slate-500" data-testid="ops-flush" @click="openConfirm('flush')">
        <DatabaseBackup class="mb-2 h-5 w-5 mts-muted" />
        <p class="text-sm font-semibold text-slate-800 dark:text-slate-100">{{ t('opsActionFlush') }}</p>
        <p class="mt-1 text-xs mts-muted">{{ t('opsFlushHint') }}</p>
      </button>
      <button type="button" class="mts-card p-5 text-left hover:border-slate-300 dark:hover:border-slate-500" id="ops-compact" data-testid="ops-compact" @click="openConfirm('compact')">
        <Layers class="mb-2 h-5 w-5 mts-muted" />
        <p class="text-sm font-semibold text-slate-800 dark:text-slate-100">{{ t('opsActionCompact') }}</p>
        <p class="mt-1 text-xs mts-muted">{{ t('opsCompactHint') }}</p>
      </button>
      <button type="button" class="mts-card p-5 text-left hover:border-slate-300 dark:hover:border-slate-500" id="ops-retention" data-testid="ops-retention" @click="openConfirm('retention')">
        <Timer class="mb-2 h-5 w-5 mts-muted" />
        <p class="text-sm font-semibold text-slate-800 dark:text-slate-100">{{ t('opsActionRetention') }}</p>
        <p class="mt-1 text-xs mts-muted">{{ t('opsRetentionHint') }}</p>
      </button>
    </div>

    <div class="grid gap-4 lg:grid-cols-2">
      <div class="mts-card p-5" data-testid="ops-maint-stats">
        <h2 class="mb-3 text-sm font-semibold text-slate-800 dark:text-slate-100">{{ t('maintenanceStats') }}</h2>
        <EmptyState v-if="!maintenanceStats" compact :title="t('opsNoMaintStats')" :description="t('opsStatsEmptyHint')" />
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
        <EmptyState v-if="!compactionStats" compact :title="t('opsNoCompactStats')" :description="t('opsStatsEmptyHint')" />
        <dl v-else class="grid grid-cols-2 gap-2 text-xs text-slate-600 dark:text-slate-300">
          <div>{{ t('opsStatTotal') }}: <b>{{ compactionStats.total }}</b></div>
          <div>{{ t('opsStatSuccess') }}: <b>{{ compactionStats.success }}</b></div>
          <div>{{ t('opsStatFailure') }}: <b>{{ compactionStats.failure }}</b></div>
          <div>{{ t('opsStatBacklog') }}: <b>{{ compactionStats.backlog }}</b></div>
          <div>{{ t('opsStatActive') }}: <b>{{ compactionStats.active }}</b></div>
        </dl>
        <p v-if="compactionStats?.last_error" class="mt-2 text-xs text-red-600 dark:text-red-300">{{ compactionStats.last_error }}</p>
      </div>
    </div>

    <div id="ops-action-log" class="mts-card scroll-mt-20 p-4" data-testid="ops-action-log">
      <div class="mb-3 flex flex-wrap items-center justify-between gap-2">
        <h2 class="text-sm font-semibold">{{ t('opsActionLog') }}</h2>
        <div class="flex flex-wrap gap-2">
          <button type="button" class="mts-btn" data-testid="ops-export-log" @click="exportActionLog">
            <Download class="h-3.5 w-3.5" />
            {{ t('opsExportLog') }}
          </button>
          <button type="button" class="mts-btn" data-testid="ops-clear-log" :disabled="!actionLog.length" @click="clearLog">
            <Eraser class="h-3.5 w-3.5" />
            {{ t('opsClearLog') }}
          </button>
        </div>
      </div>
      <p class="mb-2 text-xs mts-muted">{{ t('opsActionLogHint') }}</p>
      <EmptyState
        v-if="!actionLog.length"
        compact
        :title="t('opsLogEmpty')"
        :description="t('opsLogEmptyDesc')"
      />
      <ul v-else class="max-h-56 space-y-1 overflow-auto text-xs">
        <li
          v-for="item in actionLog"
          :key="item.id"
          class="flex flex-wrap items-start justify-between gap-2 rounded border border-slate-100 px-2 py-1.5 dark:border-slate-800"
        >
          <div class="min-w-0">
            <span
              class="mr-2 rounded px-1.5 py-0.5 font-medium"
              :class="item.status === 'ok'
                ? 'bg-emerald-50 text-emerald-800 dark:bg-emerald-950/40 dark:text-emerald-200'
                : 'bg-red-50 text-red-700 dark:bg-red-950/40 dark:text-red-200'"
            >{{ item.kind }} · {{ item.status }}</span>
            <span class="text-slate-700 dark:text-slate-200">{{ item.message }}</span>
          </div>
          <span class="shrink-0 mts-muted">{{ formatAt(item.at) }}</span>
        </li>
      </ul>
    </div>

    <ConfirmDialog
      :open="!!confirmKind"
      :title="confirmKind ? confirmTitle[confirmKind] : ''"
      :message="confirmKind ? confirmMessage[confirmKind] : ''"
      :confirm-label="t('opsExecute')"
      danger
      :loading="confirmLoading"
      @update:open="(v) => { if (!v) confirmKind = null }"
      @confirm="runConfirmed"
    />
  </div>
</template>
