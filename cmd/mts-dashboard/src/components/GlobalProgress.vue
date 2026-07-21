<script setup lang="ts">
import { useGlobalLoading } from '@/composables/useGlobalLoading'
import { useI18n } from '@/composables/useI18n'

const { busy, longBusy } = useGlobalLoading()
const { t } = useI18n()
</script>

<template>
  <div
    class="pointer-events-none fixed inset-x-0 top-0 z-[200]"
    data-testid="global-progress"
    role="progressbar"
    :aria-hidden="busy ? 'false' : 'true'"
    :aria-busy="busy ? 'true' : 'false'"
    :aria-valuetext="busy ? (longBusy ? t('globalProgressLong') : t('loading')) : undefined"
    :aria-label="longBusy ? t('globalProgressLong') : t('loading')"
  >
    <div class="h-0.5 overflow-hidden">
      <div
        v-show="busy"
        class="h-full w-full origin-left animate-pulse bg-sky-500/90 dark:bg-sky-400"
        data-testid="global-progress-bar"
        style="animation: mts-progress 1.1s ease-in-out infinite"
      />
    </div>
    <div
      v-show="longBusy"
      class="pointer-events-none flex justify-center px-2 pt-1"
      data-testid="global-progress-long"
    >
      <span
        class="rounded-full border border-sky-200/80 bg-sky-50/95 px-2.5 py-0.5 text-[11px] font-medium text-sky-900 shadow-sm dark:border-sky-900 dark:bg-sky-950/80 dark:text-sky-100"
      >{{ t('globalProgressLong') }}</span>
    </div>
  </div>
</template>

<style scoped>
@keyframes mts-progress {
  0% { transform: translateX(-100%) scaleX(0.3); }
  50% { transform: translateX(10%) scaleX(0.6); }
  100% { transform: translateX(100%) scaleX(0.3); }
}
</style>
