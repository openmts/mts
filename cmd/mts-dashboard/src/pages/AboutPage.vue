<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { apiGet } from '@/api/client'
import { formatCaughtError } from '@/utils/apiError'
import { useAuth } from '@/composables/useAuth'
import { useI18n } from '@/composables/useI18n'
import ActionResultBanner from '@/components/ActionResultBanner.vue'
import { clientBuildInfo } from '@/utils/buildInfo'
import { Info } from 'lucide-vue-next'

interface VersionResponse {
  version: string
  commit: string
  built_at: string
}

const { isAdmin, currentUser } = useAuth()
const { t } = useI18n()
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
  loadError.value = ''
  try {
    server.value = await apiGet<VersionResponse>('/api/v1/admin/version')
  } catch (e) {
    server.value = null
    loadError.value = formatCaughtError(e)
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  void loadVersion()
})
</script>

<template>
  <div class="space-y-6">
    <div>
      <h1 class="mts-title flex items-center gap-2">
        <Info class="h-5 w-5" />
        {{ t('aboutTitle') }}
      </h1>
      <p class="text-xs mts-muted">{{ t('aboutDesc') }}</p>
    </div>

    <ActionResultBanner v-if="loadError" kind="error" :message="loadError" @dismiss="loadError = ''" />

    <div class="grid gap-4 md:grid-cols-2">
      <div class="mts-card p-4">
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
            <dt class="mts-muted">BASE_URL</dt>
            <dd class="font-mono">{{ client.base }}</dd>
          </div>
          <div class="flex justify-between gap-3">
            <dt class="mts-muted">VITE_API_BASE</dt>
            <dd class="font-mono">{{ client.apiBase || '—' }}</dd>
          </div>
          <div class="flex justify-between gap-3">
            <dt class="mts-muted">{{ t('user') }}</dt>
            <dd class="font-mono">{{ currentUser || '—' }}</dd>
          </div>
        </dl>
      </div>

      <div class="mts-card p-4">
        <div class="mb-3 flex items-center justify-between gap-2">
          <h2 class="text-sm font-semibold">{{ t('aboutServer') }}</h2>
          <button v-if="isAdmin" type="button" class="mts-btn" :disabled="loading" @click="loadVersion">
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
            <dt class="mts-muted">commit</dt>
            <dd class="font-mono break-all">{{ server.commit }}</dd>
          </div>
          <div class="flex justify-between gap-3">
            <dt class="mts-muted">built_at</dt>
            <dd class="font-mono">{{ server.built_at }}</dd>
          </div>
        </dl>
        <p v-else class="text-xs mts-muted">{{ loading ? t('loading') : '—' }}</p>
      </div>
    </div>
  </div>
</template>
