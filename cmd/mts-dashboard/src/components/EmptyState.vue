<script setup lang="ts">
withDefaults(defineProps<{
  title: string
  description?: string
  compact?: boolean
}>(), {
  description: '',
  compact: false,
})
</script>

<template>
  <div
    class="flex flex-col items-center justify-center text-center"
    :class="compact ? 'px-3 py-6' : 'px-4 py-10'"
    role="status"
  >
    <div
      class="mb-2 flex items-center justify-center rounded-full bg-slate-100 text-slate-400 dark:bg-slate-800 dark:text-slate-500"
      :class="compact ? 'h-8 w-8' : 'h-12 w-12'"
    >
      <slot name="icon">
        <span class="text-lg">∅</span>
      </slot>
    </div>
    <p class="text-sm font-medium text-slate-700 dark:text-slate-200">{{ title }}</p>
    <p v-if="description" class="mt-1 max-w-md text-xs text-slate-500 dark:text-slate-400">{{ description }}</p>
    <div v-if="$slots.action" class="mt-3">
      <slot name="action" />
    </div>
  </div>
</template>
