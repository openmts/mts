<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useI18n } from '@/composables/useI18n'
import { formatMessage } from '@/utils/formatMessage'
import { formatElapsedSeconds, isLongRunning } from '@/utils/inFlightStatus'
import { Square } from 'lucide-vue-next'

const props = withDefaults(
  defineProps<{
    active: boolean
    startedAtMs?: number | null
    kind?: 'query' | 'write' | 'delete' | 'ops' | 'storage' | 'admin' | 'login'
    timeoutHintMs?: number
    longThresholdMs?: number
  }>(),
  {
    startedAtMs: null,
    kind: 'query',
    timeoutHintMs: 30_000,
    longThresholdMs: 5_000,
  },
)

const emit = defineEmits<{ cancel: [] }>()
const { t } = useI18n()
const nowMs = ref(Date.now())
let timer: ReturnType<typeof setInterval> | null = null

function arm() {
  if (timer) clearInterval(timer)
  timer = null
  if (!props.active) return
  nowMs.value = Date.now()
  timer = setInterval(() => {
    nowMs.value = Date.now()
  }, 250)
}

watch(
  () => props.active,
  () => arm(),
  { immediate: true },
)

onMounted(() => arm())
onBeforeUnmount(() => {
  if (timer) clearInterval(timer)
})

const elapsedMs = computed(() => {
  if (!props.active || props.startedAtMs == null) return 0
  return Math.max(0, nowMs.value - props.startedAtMs)
})

const elapsedLabel = computed(() => formatElapsedSeconds(elapsedMs.value))
const longRun = computed(() => isLongRunning(elapsedMs.value, props.longThresholdMs))
const nearTimeout = computed(
  () => props.timeoutHintMs > 0 && elapsedMs.value >= Math.max(0, props.timeoutHintMs - 5_000),
)

const title = computed(() => {
  if (props.kind === 'write') return t.value('writeInFlightTitle')
  if (props.kind === 'delete') return t.value('deleteInFlightTitle')
  if (props.kind === 'ops') return t.value('opsInFlightTitle')
  if (props.kind === 'storage') return t.value('storageInFlightTitle')
  if (props.kind === 'admin') return t.value('adminInFlightTitle')
  if (props.kind === 'login') return t.value('loginInFlightTitle')
  return t.value('queryInFlightTitle')
})

const detail = computed(() => {
  const base = formatMessage(t.value('inFlightElapsed'), { elapsed: elapsedLabel.value })
  if (nearTimeout.value) {
    return `${base} · ${formatMessage(t.value('inFlightNearTimeout'), {
      seconds: Math.ceil(props.timeoutHintMs / 1000),
    })}`
  }
  if (longRun.value) {
    return `${base} · ${t.value('inFlightLongHint')}`
  }
  return base
})
</script>

<template>
  <div
    v-if="active"
    class="flex flex-wrap items-center justify-between gap-2 rounded-lg border border-sky-200 bg-sky-50 px-3 py-2 text-xs text-sky-900 dark:border-sky-900/50 dark:bg-sky-950/40 dark:text-sky-100"
    role="status"
    aria-live="polite"
    data-testid="in-flight-banner"
  >
    <div class="min-w-0 flex-1">
      <p class="font-medium" data-testid="in-flight-title">{{ title }}</p>
      <p class="mt-0.5 mts-muted text-sky-800/80 dark:text-sky-200/80" data-testid="in-flight-detail">{{ detail }}</p>
    </div>
    <button
      type="button"
      class="mts-btn text-xs"
      data-testid="in-flight-cancel"
      :aria-label="t('cancel')"
      :title="t('cancel')"
      @click="emit('cancel')"
    >
      <Square class="mr-1 inline h-3.5 w-3.5" aria-hidden="true" />{{ t('cancel') }}
    </button>
  </div>
</template>
