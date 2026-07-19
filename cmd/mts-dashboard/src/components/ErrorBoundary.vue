<script setup lang="ts">
import { onErrorCaptured, ref } from 'vue'
import EmptyState from '@/components/EmptyState.vue'

withDefaults(defineProps<{
  title?: string
  description?: string
}>(), {
  title: '页面渲染异常',
  description: '组件运行时出错。可尝试重试渲染或刷新页面；若持续出现请查看控制台日志。',
})

const errMsg = ref('')
const errStack = ref('')
const renderKey = ref(0)

onErrorCaptured((err) => {
  errMsg.value = err instanceof Error ? err.message : String(err)
  errStack.value = err instanceof Error && err.stack ? err.stack : ''
  return false
})

function reset() {
  errMsg.value = ''
  errStack.value = ''
  renderKey.value += 1
}

function reloadPage() {
  window.location.reload()
}

defineExpose({ reset })
</script>

<template>
  <div v-if="errMsg" class="mts-panel">
    <EmptyState :title="title" :description="description">
      <template #icon>
        <span class="text-lg text-red-500">!</span>
      </template>
      <template #action>
        <div class="space-y-2">
          <p class="rounded-lg bg-red-50 px-3 py-2 font-mono text-xs text-red-700 dark:bg-red-950/40 dark:text-red-200">{{ errMsg }}</p>
          <details v-if="errStack" class="text-left">
            <summary class="cursor-pointer text-xs text-slate-500">堆栈</summary>
            <pre class="mt-1 max-h-40 overflow-auto rounded bg-slate-950 p-2 text-[10px] text-slate-300">{{ errStack }}</pre>
          </details>
          <div class="flex flex-wrap justify-center gap-2">
            <button type="button" class="mts-btn-primary" @click="reset">重试渲染</button>
            <button type="button" class="mts-btn" @click="reloadPage">刷新页面</button>
          </div>
        </div>
      </template>
    </EmptyState>
  </div>
  <div v-else :key="renderKey">
    <slot />
  </div>
</template>
