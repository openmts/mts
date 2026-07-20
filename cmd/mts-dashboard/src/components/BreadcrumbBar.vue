<script setup lang="ts">
import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from '@/composables/useI18n'
import { useNotify } from '@/composables/useNotify'
import type { MessageKey } from '@/i18n/messages'
import { buildBreadcrumbs } from '@/utils/breadcrumbs'
import { copyText } from '@/utils/clipboard'
import { ChevronRight, Copy } from 'lucide-vue-next'

const route = useRoute()
const router = useRouter()
const { t } = useI18n()
const { success, error: notifyError } = useNotify()

const crumbs = computed(() => buildBreadcrumbs(route.path))
const currentPath = computed(() => route.fullPath || route.path || '/')

function labelFor(key: string): string {
  return t.value(key as MessageKey)
}

function go(path: string) {
  void router.push(path)
}

async function copyPath() {
  const res = await copyText(currentPath.value)
  if (res.ok) success(t.value('breadcrumbPathCopied'))
  else notifyError(res.error || t.value('failed'))
}
</script>

<template>
  <nav
    class="no-print flex flex-wrap items-center gap-1 border-b border-slate-200 bg-white px-3 py-1.5 text-[11px] dark:border-slate-700 dark:bg-slate-900 sm:px-6"
    :aria-label="t('breadcrumbNav')"
    data-testid="breadcrumb-bar"
  >
    <div class="flex min-w-0 flex-1 flex-wrap items-center gap-1">
      <template v-for="(c, i) in crumbs" :key="c.path + i">
        <ChevronRight v-if="i > 0" class="h-3 w-3 shrink-0 text-slate-300 dark:text-slate-600" aria-hidden="true" />
        <button
          v-if="i < crumbs.length - 1"
          type="button"
          class="rounded px-1 py-0.5 text-slate-500 hover:bg-slate-100 hover:text-slate-800 dark:text-slate-400 dark:hover:bg-slate-800 dark:hover:text-slate-100"
          :data-testid="`breadcrumb-link-${i}`"
          @click="go(c.path)"
        >
          {{ labelFor(c.labelKey) }}
        </button>
        <span
          v-else
          class="rounded px-1 py-0.5 font-medium text-slate-800 dark:text-slate-100"
          aria-current="page"
          data-testid="breadcrumb-current"
        >
          {{ labelFor(c.labelKey) }}
        </span>
      </template>
    </div>
    <button
      type="button"
      class="mts-focus-ring ml-auto inline-flex shrink-0 items-center gap-1 rounded px-1.5 py-0.5 text-slate-500 hover:bg-slate-100 hover:text-slate-800 dark:text-slate-400 dark:hover:bg-slate-800 dark:hover:text-slate-100"
      data-testid="breadcrumb-copy-path"
      :title="t('breadcrumbCopyPath')"
      :aria-label="t('breadcrumbCopyPath')"
      @click="copyPath"
    >
      <Copy class="h-3 w-3" aria-hidden="true" />
      <span class="hidden sm:inline">{{ t('breadcrumbCopyPath') }}</span>
    </button>
  </nav>
</template>
