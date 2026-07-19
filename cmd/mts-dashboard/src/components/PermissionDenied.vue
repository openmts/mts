<script setup lang="ts">
defineProps<{ title?: string; detail?: string }>()
</script>

<template>
  <div class="flex min-h-48 flex-col items-center justify-center rounded-xl border border-dashed border-slate-300 bg-white p-8 text-center">
    <h3 class="text-sm font-semibold text-slate-800">{{ title || '权限不足' }}</h3>
    <p class="mt-2 max-w-md text-xs text-slate-500">{{ detail || '当前账号无权访问该页面。如需管理操作，请使用管理员账号登录。' }}</p>
  </div>
</template>
