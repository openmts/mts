<script setup lang="ts">
import { useRouter } from 'vue-router'

const router = useRouter()
</script>

<template>
  <div class="flex min-h-[50vh] flex-col items-center justify-center gap-3 text-center">
    <h2 class="text-lg font-semibold text-slate-800 dark:text-slate-100">页面不存在</h2>
    <p class="text-sm text-slate-500 dark:text-slate-400 dark:text-slate-500">请检查地址，或返回仪表盘。</p>
    <button class="rounded-lg bg-slate-800 px-4 py-2 text-sm text-white hover:bg-slate-700" @click="router.push({ name: 'Overview' })">
      返回仪表盘
    </button>
  </div>
</template>
