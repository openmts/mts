<script setup lang="ts">
import { computed } from 'vue'
import { useRouter } from 'vue-router'
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
  /** 显示重试按钮（加载失败等） */
  retryable?: boolean
  retryLabel?: string
  /** 可选快捷动作（如跳转运维状态条） */
  actionLabel?: string
  actionPath?: string
}>(), {
  result: null,
  kind: 'info',
  message: '',
  dismissible: true,
  retryable: false,
  retryLabel: '',
  actionLabel: '',
  actionPath: '',
})

const emit = defineEmits<{ dismiss: []; retry: [] }>()
const { t, locale } = useI18n()
const router = useRouter()

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
const label = computed(() =>
  view.value
    ? actionResultLabel(view.value.kind, locale.value === 'en' ? 'en' : 'zh')
    : '',
)
const liveRole = computed(() => (view.value?.kind === 'error' ? 'alert' : 'status'))
const showAction = computed(() => Boolean((props.actionPath || '').trim() && (props.actionLabel || '').trim()))

function goAction() {
  const path = (props.actionPath || '').trim()
  if (!path) return
  void router.push(path)
}
</script>

<template>
  <div
    v-if="view"
    class="flex items-start justify-between gap-3"
    :class="cls"
    :role="liveRole"
    :aria-live="view.kind === 'error' ? 'assertive' : 'polite'"
  >
    <div class="min-w-0 flex-1">
      <div class="mb-0.5 text-[11px] font-semibold uppercase tracking-wide opacity-80">{{ label }}</div>
      <p class="whitespace-pre-wrap break-words text-sm">{{ view.message }}</p>
    </div>
    <div class="flex shrink-0 items-center gap-1">
      <button
        v-if="showAction"
        type="button"
        class="mts-btn text-xs"
        data-testid="action-result-action"
        @click="goAction"
      >{{ actionLabel }}</button>
      <button
        v-if="retryable"
        type="button"
        class="mts-btn text-xs"
        data-testid="action-result-retry"
        @click="emit('retry')"
      >{{ retryLabel || t('retry') }}</button>
      <button
        v-if="dismissible"
        type="button"
        class="mts-focus-ring rounded p-1 opacity-60 hover:bg-black/5 hover:opacity-100 dark:hover:bg-white/10"
        :title="t('dismiss')"
        :aria-label="t('dismiss')"
        data-testid="action-result-dismiss"
        @click="emit('dismiss')"
      >
        <X class="h-3.5 w-3.5" />
      </button>
    </div>
  </div>
</template>
