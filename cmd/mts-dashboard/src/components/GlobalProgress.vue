<script setup lang="ts">
import { useGlobalLoading } from '@/composables/useGlobalLoading'

const { busy } = useGlobalLoading()
</script>

<template>
  <div
    class="pointer-events-none fixed inset-x-0 top-0 z-[200] h-0.5 overflow-hidden"
    aria-hidden="true"
  >
    <div
      v-show="busy"
      class="h-full w-full origin-left animate-pulse bg-sky-500/90 dark:bg-sky-400"
      style="animation: mts-progress 1.1s ease-in-out infinite"
    />
  </div>
</template>

<style scoped>
@keyframes mts-progress {
  0% { transform: translateX(-100%) scaleX(0.3); }
  50% { transform: translateX(10%) scaleX(0.6); }
  100% { transform: translateX(100%) scaleX(0.3); }
}
</style>
