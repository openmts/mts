<script setup lang="ts">
import { useRouter } from 'vue-router'
import EmptyState from '@/components/EmptyState.vue'

const router = useRouter()
</script>

<template>
  <div class="flex min-h-[50vh] items-center justify-center">
    <EmptyState title="页面不存在" description="地址无效或资源已移除。请检查 URL，或返回概览继续使用。">
      <template #icon>
        <span class="text-lg font-semibold">404</span>
      </template>
      <template #action>
        <div class="flex flex-wrap items-center justify-center gap-2">
          <button
            type="button"
            class="rounded-lg bg-slate-800 px-4 py-2 text-sm text-white hover:bg-slate-700 dark:bg-slate-100 dark:text-slate-900 dark:hover:bg-white"
            @click="router.push({ name: 'Overview' })"
          >
            返回概览
          </button>
          <button
            type="button"
            class="rounded-lg border border-slate-300 px-4 py-2 text-sm text-slate-700 hover:bg-slate-50 dark:border-slate-600 dark:text-slate-200 dark:hover:bg-slate-800"
            @click="router.back()"
          >
            返回上一页
          </button>
        </div>
      </template>
    </EmptyState>
  </div>
</template>
