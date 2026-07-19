<script setup lang="ts">
withDefaults(defineProps<{ rows?: number }>(), { rows: 4 })
</script>

<template>
  <div class="space-y-3" role="status" aria-live="polite" aria-busy="true">
    <div class="h-6 w-40 animate-pulse rounded bg-slate-200 dark:bg-slate-800" />
    <div class="h-4 w-72 max-w-full animate-pulse rounded bg-slate-100 dark:bg-slate-800/80" />
    <div
      v-for="i in rows"
      :key="i"
      class="h-16 animate-pulse rounded-xl border border-slate-100 bg-slate-50 dark:border-slate-800 dark:bg-slate-900/50"
    />
  </div>
</template>
