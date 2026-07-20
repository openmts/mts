<script setup lang="ts">
import { useRouter } from 'vue-router'
import EmptyState from '@/components/EmptyState.vue'
import { useI18n } from '@/composables/useI18n'

const router = useRouter()
const { t } = useI18n()
</script>

<template>
  <div class="flex min-h-[50vh] items-center justify-center" data-testid="not-found-page">
    <EmptyState :title="t('notFoundTitle')" :description="t('notFoundDesc')">
      <template #icon>
        <span class="text-lg font-semibold">404</span>
      </template>
      <template #action>
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
        </div>
      </template>
    </EmptyState>
  </div>
</template>
