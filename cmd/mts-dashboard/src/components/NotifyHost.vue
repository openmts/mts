<script setup lang="ts">
import { useNotify } from '@/composables/useNotify'

const { items, dismiss } = useNotify()
</script>

<template>
  <div class="pointer-events-none fixed right-4 top-4 z-[100] flex w-80 max-w-[calc(100vw-2rem)] flex-col gap-2">
    <div
      v-for="n in items"
      :key="n.id"
      class="pointer-events-auto rounded-lg border px-3 py-2 text-sm shadow-lg"
      :class="{
        'border-emerald-200 bg-emerald-50 text-emerald-800': n.kind === 'success',
        'border-red-200 bg-red-50 text-red-800': n.kind === 'error',
        'border-slate-200 bg-white text-slate-700': n.kind === 'info',
      }"
    >
      <div class="flex items-start justify-between gap-2">
        <p class="flex-1 break-words">{{ n.message }}</p>
        <button class="text-xs opacity-60 hover:opacity-100" @click="dismiss(n.id)">关闭</button>
      </div>
    </div>
  </div>
</template>
