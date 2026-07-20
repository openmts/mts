<script setup lang="ts">
import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import EmptyState from '@/components/EmptyState.vue'
import { useI18n } from '@/composables/useI18n'
import { formatRedirectLabel } from '@/utils/redirect'
import { loadRecentRoutes } from '@/utils/recentRoutes'

const router = useRouter()
const route = useRoute()
const { t } = useI18n()

const attemptedPath = computed(() => {
  const full = route.fullPath || ''
  // nested catch-all under layout may be relative; prefer fullPath
  return full && full !== '/' ? full : String(route.params.pathMatch || '')
})

const attemptedLabel = computed(() =>
  attemptedPath.value ? formatRedirectLabel(attemptedPath.value, 120) : '',
)

const recent = computed(() => {
  if (typeof sessionStorage === 'undefined') return []
  return loadRecentRoutes(sessionStorage)
    .filter((x) => x.path && x.path !== attemptedPath.value)
    .slice(0, 5)
})

function goRecent(path: string) {
  void router.push(path)
}
</script>

<template>
  <div class="flex min-h-[50vh] items-center justify-center" data-testid="not-found-page">
    <EmptyState :title="t('notFoundTitle')" :description="t('notFoundDesc')">
      <template #icon>
        <span class="text-lg font-semibold">404</span>
      </template>
      <template #action>
        <div class="flex w-full max-w-md flex-col items-center gap-3">
          <p
            v-if="attemptedLabel"
            class="w-full rounded-lg border border-slate-200 bg-slate-50 px-3 py-2 text-left text-xs text-slate-700 dark:border-slate-700 dark:bg-slate-800/60 dark:text-slate-200"
            data-testid="not-found-path"
          >
            {{ t('notFoundPathLabel') }}
            <span class="mt-0.5 block break-all font-mono text-[11px]">{{ attemptedLabel }}</span>
          </p>
          <div class="flex flex-wrap items-center justify-center gap-2">
            <button
              type="button"
              class="rounded-lg bg-slate-800 px-4 py-2 text-sm text-white hover:bg-slate-700 dark:bg-slate-100 dark:text-slate-900 dark:hover:bg-white"
              data-testid="not-found-go-overview"
              @click="router.push({ name: 'Overview' })"
            >
              {{ t('notFoundGoOverview') }}
            </button>
            <button
              type="button"
              class="rounded-lg border border-slate-300 px-4 py-2 text-sm text-slate-700 hover:bg-slate-50 dark:border-slate-600 dark:text-slate-200 dark:hover:bg-slate-800"
              data-testid="not-found-go-back"
              @click="router.back()"
            >
              {{ t('notFoundGoBack') }}
            </button>
            <button
              type="button"
              class="rounded-lg border border-slate-300 px-4 py-2 text-sm text-slate-700 hover:bg-slate-50 dark:border-slate-600 dark:text-slate-200 dark:hover:bg-slate-800"
              data-testid="not-found-go-query"
              @click="router.push({ name: 'Query' })"
            >
              {{ t('query') }}
            </button>
          </div>
          <div v-if="recent.length" class="w-full space-y-1.5" data-testid="not-found-recent">
            <p class="text-center text-[11px] mts-muted">{{ t('notFoundRecentTitle') }}</p>
            <div class="flex flex-wrap justify-center gap-1.5">
              <button
                v-for="item in recent"
                :key="item.path"
                type="button"
                class="max-w-full truncate rounded-md border border-slate-200 px-2 py-1 font-mono text-[11px] text-slate-700 hover:bg-slate-50 dark:border-slate-600 dark:text-slate-200 dark:hover:bg-slate-800"
                :data-testid="`not-found-recent-${item.path.replaceAll('/', '_') || 'root'}`"
                :title="item.path"
                @click="goRecent(item.path)"
              >
                {{ item.name || item.path }}
              </button>
            </div>
          </div>
        </div>
      </template>
    </EmptyState>
  </div>
</template>
