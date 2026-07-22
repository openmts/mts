<script setup lang="ts">
import { computed, inject, onMounted, ref, type ComputedRef } from 'vue'
import { useRoute } from 'vue-router'
import { useHashScroll } from '@/composables/useHashScroll'
import { aboutFormToPrefill, parseAboutPrefill } from '@/utils/routePrefill'
import { apiGet } from '@/api/client'
import { formatCaughtError } from '@/utils/apiError'
import { useAuth } from '@/composables/useAuth'
import { useI18n } from '@/composables/useI18n'
import { useNotify } from '@/composables/useNotify'
import { useNotifyAdminBusy } from '@/composables/useNotifyAdminBusy'
import ActionResultBanner from '@/components/ActionResultBanner.vue'
import AdminOpLastChip from '@/components/AdminOpLastChip.vue'
import EmptyState from '@/components/EmptyState.vue'
import PartialErrorBanner from '@/components/PartialErrorBanner.vue'
import { clientBuildInfo } from '@/utils/buildInfo'
import { buildAboutExport, formatAboutExportPretty } from '@/utils/aboutExport'
import { stampFilename } from '@/utils/download'
import { useExportJob } from '@/composables/useExportJob'
import ExportJobBanner from '@/components/ExportJobBanner.vue'
import { copyText } from '@/utils/clipboard'
import { Download, Copy, Info } from 'lucide-vue-next'

interface VersionResponse {
  version: string
  commit: string
  built_at: string
}

const route = useRoute()
useHashScroll()
const { isAdmin, currentUser } = useAuth()
const adminOpBusySummary = inject<ComputedRef<{ lastSummary?: string; lastError?: string; lastOk?: boolean | null }> | undefined>('adminOpBusySummary', undefined)
const aboutAdminLastLabel = computed(() => (adminOpBusySummary?.value?.lastSummary || '').trim())
const aboutAdminLastErrorDetail = computed(() => {
  if (adminOpBusySummary?.value?.lastOk !== false) return ''
  return (adminOpBusySummary?.value?.lastError || '').trim()
})
const { t } = useI18n()
const { success, info, error: notifyError } = useNotify()
const { notifyMaybeAdminBusy } = useNotifyAdminBusy()
const {
  exportJob,
  exportBusy,
  cancelExport,
  resetExport,
  retryLastExport,
  canRetryExport,
  runJSONExport,
  runTextExport,
} = useExportJob()
const client = clientBuildInfo()
const server = ref<VersionResponse | null>(null)
const loadError = ref('')
const loading = ref(false)

async function loadVersion() {
  if (!isAdmin.value) {
    server.value = null
    return
  }
  loading.value = true
  try {
    server.value = await apiGet<VersionResponse>('/api/v1/admin/version')
    loadError.value = ''
  } catch (e) {
    const msg = formatCaughtError(e)
    if (server.value) {
      loadError.value = msg
      notifyMaybeAdminBusy(msg, e, { treatLocalBusy: true })
    } else {
      server.value = null
      loadError.value = msg
      notifyMaybeAdminBusy(msg, e)
    }
  } finally {
    loading.value = false
  }
}

async function exportAbout() {
  if (exportBusy.value) return
  const payload = buildAboutExport({
    client,
    server: server.value,
    user: currentUser.value || '',
  })
  const outcome = await runJSONExport({
    label: 'JSON',
    filename: stampFilename('mts-about', 'json'),
    total: 1,
    build: async ({ isCancelled, progress }) => {
      progress(0, 1)
      if (isCancelled()) return null
      progress(1, 1)
      return payload
    },
  })
  if (outcome === 'done') success(t.value('aboutExported'))
  else if (outcome === 'cancelled') info(t.value('exportCancelledToast'))
  else if (outcome === 'error') {
    const m = exportJob.value.error || t.value('failed')
    notifyError(m)
    notifyMaybeAdminBusy(m, { message: m }, { treatLocalBusy: true })
  }
}

async function copyAbout() {
  const res = await copyText(
    formatAboutExportPretty({
      client,
      server: server.value,
      user: currentUser.value || '',
    }),
  )
  if (res.ok) success(t.value('aboutCopied'))
  else notifyError(res.error || t.value('failed'))
}


function currentAboutSection(): string {
  const h = (route.hash || (typeof window !== 'undefined' ? window.location.hash : '') || '').replace(/^#/, '')
  if (h === 'about-client' || h === 'about-server') return h
  const pre = parseAboutPrefill(route.query as Record<string, unknown>, route.hash)
  return pre.section || 'about-client'
}

async function copyAboutShareLink() {
  const path = aboutFormToPrefill({ section: currentAboutSection() })
  const url = `${window.location.origin}${path}`
  const res = await copyText(url)
  if (res.ok) success(t.value('aboutShareCopied'))
  else notifyError(res.error || t.value('failed'))
}

onMounted(() => {
  void loadVersion()
})
</script>

<template>
  <div class="space-y-6" data-testid="about-page">
    <div class="flex flex-wrap items-start justify-between gap-3">
      <div>
        <h1 class="mts-title flex items-center gap-2">
          <Info class="h-5 w-5" />
          {{ t('aboutTitle') }}
        </h1>
        <div class="mt-1 flex flex-wrap items-center gap-2">
          <p class="text-xs mts-muted">{{ t('aboutDesc') }}</p>
          <AdminOpLastChip
            v-if="isAdmin && aboutAdminLastLabel"
            :label="aboutAdminLastLabel"
            :last-ok="adminOpBusySummary?.lastOk"
            :last-error="aboutAdminLastErrorDetail"
            test-id="about-admin-last"
            error-test-id="about-admin-last-error"
          />
        </div>
      </div>
      <div class="flex flex-wrap gap-2">
        <ExportJobBanner class="w-full basis-full" :job="exportJob" :retryable="canRetryExport" @cancel="cancelExport" @retry="retryLastExport" @dismiss="resetExport" />
        <button type="button" class="mts-btn" data-testid="about-export-json" :disabled="exportBusy" @click="exportAbout">
          <Download class="h-3.5 w-3.5" /> {{ t('aboutExportJSON') }}
        </button>
        <button type="button" class="mts-btn" data-testid="about-copy" @click="copyAbout">
          <Copy class="h-3.5 w-3.5" /> {{ t('aboutCopy') }}
        </button>
        <button type="button" class="mts-btn" data-testid="about-share-link" @click="copyAboutShareLink">
          {{ t('aboutShareLink') }}
        </button>
      </div>
    </div>

    <ActionResultBanner
      v-if="loadError && !server"
      kind="error"
      :message="loadError"
      retryable
      data-testid="about-load-error"
      @retry="loadVersion"
      @dismiss="loadError = ''"
    />
    <PartialErrorBanner
      v-else-if="loadError && server"
      :message="`${t('aboutVersionRefreshFailed')}：${loadError}`"
      test-id="about-refresh-error"
      @retry="loadVersion"
      @dismiss="loadError = ''"
    />

    <div class="grid gap-4 md:grid-cols-2">
      <div id="about-client" class="mts-card scroll-mt-20 p-4" data-testid="about-client">
        <h2 class="mb-3 text-sm font-semibold">{{ t('aboutClient') }}</h2>
        <dl class="space-y-2 text-sm">
          <div class="flex justify-between gap-3">
            <dt class="mts-muted">{{ t('aboutName') }}</dt>
            <dd class="font-mono">{{ client.name }}</dd>
          </div>
          <div class="flex justify-between gap-3">
            <dt class="mts-muted">{{ t('aboutVersion') }}</dt>
            <dd class="font-mono">{{ client.version }}</dd>
          </div>
          <div class="flex justify-between gap-3">
            <dt class="mts-muted">{{ t('aboutMode') }}</dt>
            <dd class="font-mono">{{ client.mode }}</dd>
          </div>
          <div class="flex justify-between gap-3">
            <dt class="mts-muted">{{ t('aboutBaseUrl') }}</dt>
            <dd class="font-mono">{{ client.base }}</dd>
          </div>
          <div class="flex justify-between gap-3">
            <dt class="mts-muted">{{ t('aboutApiBase') }}</dt>
            <dd class="font-mono">{{ client.apiBase || t('emptyValue') }}</dd>
          </div>
          <div class="flex justify-between gap-3">
            <dt class="mts-muted">{{ t('user') }}</dt>
            <dd class="font-mono">{{ currentUser || t('emptyValue') }}</dd>
          </div>
        </dl>
      </div>

      <div id="about-server" class="mts-card scroll-mt-20 p-4" data-testid="about-server">
        <div class="mb-3 flex items-center justify-between gap-2">
          <h2 class="text-sm font-semibold">{{ t('aboutServer') }}</h2>
          <button
            v-if="isAdmin"
            type="button"
            class="mts-btn"
            data-testid="about-server-refresh"
            :disabled="loading"
            :aria-busy="loading ? 'true' : undefined"
            @click="loadVersion"
          >
            {{ loading ? t('loading') : t('refresh') }}
          </button>
        </div>
        <p v-if="!isAdmin" class="text-xs mts-muted">{{ t('aboutServerAdminOnly') }}</p>
        <dl v-else-if="server" class="space-y-2 text-sm">
          <div class="flex justify-between gap-3">
            <dt class="mts-muted">{{ t('aboutVersion') }}</dt>
            <dd class="font-mono">{{ server.version }}</dd>
          </div>
          <div class="flex justify-between gap-3">
            <dt class="mts-muted">{{ t('aboutCommit') }}</dt>
            <dd class="font-mono break-all">{{ server.commit }}</dd>
          </div>
          <div class="flex justify-between gap-3">
            <dt class="mts-muted">{{ t('aboutBuiltAt') }}</dt>
            <dd class="font-mono">{{ server.built_at }}</dd>
          </div>
        </dl>
        <EmptyState
          v-else
          compact
          data-testid="about-server-empty"
          :title="loading ? t('loading') : t('aboutServerEmptyTitle')"
          :description="loading ? '' : t('aboutServerEmptyDesc')"
        >
          <template v-if="!loading" #action>
            <button
              type="button"
              class="mts-btn-primary text-xs"
              data-testid="about-server-empty-retry"
              :disabled="loading"
              @click="loadVersion"
            >{{ t('overviewSectionRetry') }}</button>
          </template>
        </EmptyState>
      </div>
    </div>
  </div>
</template>
