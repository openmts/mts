<script setup lang="ts">
import { computed, onErrorCaptured, ref } from 'vue'
import EmptyState from '@/components/EmptyState.vue'
import { useI18n } from '@/composables/useI18n'

const props = defineProps<{
  title?: string
  description?: string
}>()

const { t } = useI18n()

const titleText = computed(() => props.title || t.value('errorBoundaryTitle'))
const descText = computed(() => props.description || t.value('errorBoundaryDesc'))

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
  <div v-if="errMsg" class="mts-panel" role="alert" data-testid="error-boundary">
    <EmptyState :title="titleText" :description="descText">
      <template #icon>
        <span class="text-lg text-red-500" aria-hidden="true">!</span>
      </template>
      <template #action>
        <div class="space-y-2">
          <p class="rounded-lg bg-red-50 px-3 py-2 font-mono text-xs text-red-700 dark:bg-red-950/40 dark:text-red-200">{{ errMsg }}</p>
          <details v-if="errStack" class="text-left">
            <summary class="cursor-pointer text-xs text-slate-500">{{ t('errorBoundaryStack') }}</summary>
            <pre class="mt-1 max-h-40 overflow-auto rounded bg-slate-950 p-2 text-[10px] text-slate-300">{{ errStack }}</pre>
          </details>
          <div class="flex flex-wrap justify-center gap-2">
            <button type="button" class="mts-btn-primary" data-testid="error-boundary-retry" @click="reset">{{ t('errorBoundaryRetry') }}</button>
            <button type="button" class="mts-btn" data-testid="error-boundary-reload" @click="reloadPage">{{ t('errorBoundaryReload') }}</button>
          </div>
        </div>
      </template>
    </EmptyState>
  </div>
  <div v-else :key="renderKey">
    <slot />
  </div>
</template>
