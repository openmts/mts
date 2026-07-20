<script setup lang="ts">
import { computed } from 'vue'
import {
  actionResultClass,
  actionResultLabel,
  type ActionResult,
  type ActionResultKind,
} from '@/utils/actionResult'
import { X } from 'lucide-vue-next'
import { useI18n } from '@/composables/useI18n'

const props = withDefaults(defineProps<{
  result?: ActionResult | null
  kind?: ActionResultKind
  message?: string
  dismissible?: boolean
}>(), {
  result: null,
  kind: 'info',
  message: '',
  dismissible: true,
})

const emit = defineEmits<{ dismiss: [] }>()
const { t } = useI18n()

const view = computed(() => {
  if (props.result?.message) return props.result
  if (props.message) {
    return {
      kind: props.kind,
      message: props.message,
      at: 0,
    } as ActionResult
  }
  return null
})

const cls = computed(() => (view.value ? actionResultClass(view.value.kind) : ''))
const label = computed(() => (view.value ? actionResultLabel(view.value.kind) : ''))
</script>

<template>
  <div v-if="view" class="flex items-start justify-between gap-3" :class="cls" role="status">
    <div class="min-w-0 flex-1">
      <div class="mb-0.5 text-[11px] font-semibold uppercase tracking-wide opacity-80">{{ label }}</div>
      <p class="whitespace-pre-wrap break-words text-sm">{{ view.message }}</p>
    </div>
    <button
      v-if="dismissible"
      type="button"
      class="shrink-0 rounded p-1 opacity-60 hover:bg-black/5 hover:opacity-100 dark:hover:bg-white/10"
      :title="t('dismiss')"
      @click="emit('dismiss')"
    >
      <X class="h-3.5 w-3.5" />
    </button>
  </div>
</template>
