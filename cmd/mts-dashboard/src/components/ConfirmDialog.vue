<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue'
import { createFocusTrap, type FocusTrapHandle } from '@/utils/focusTrap'

const props = withDefaults(defineProps<{
  open: boolean
  title: string
  message: string
  confirmLabel?: string
  cancelLabel?: string
  danger?: boolean
  /** 若设置，用户必须输入完全匹配的字符串才能确认 */
  requireText?: string
  loading?: boolean
}>(), {
  confirmLabel: '确认',
  cancelLabel: '取消',
  danger: false,
  requireText: '',
  loading: false,
})

const emit = defineEmits<{
  'update:open': [boolean]
  confirm: []
  cancel: []
}>()

const input = ref('')
const panelRef = ref<HTMLElement | null>(null)
const inputRef = ref<HTMLInputElement | null>(null)
const primaryRef = ref<HTMLButtonElement | null>(null)
const titleId = `confirm-title-${Math.random().toString(36).slice(2, 8)}`
let trap: FocusTrapHandle | null = null

const canConfirm = computed(() => {
  if (props.loading) return false
  if (!props.requireText) return true
  return input.value === props.requireText
})

function releaseTrap() {
  trap?.release()
  trap = null
}

function close() {
  if (props.loading) return
  emit('update:open', false)
  emit('cancel')
  input.value = ''
}

function onConfirm() {
  if (!canConfirm.value) return
  emit('confirm')
}

function onKey(e: KeyboardEvent) {
  if (!props.open) return
  if (e.key === 'Escape') {
    e.preventDefault()
    close()
  }
}

watch(() => props.open, async (open) => {
  window.removeEventListener('keydown', onKey)
  if (!open) {
    input.value = ''
    releaseTrap()
    document.body.style.overflow = ''
    return
  }
  document.body.style.overflow = 'hidden'
  window.addEventListener('keydown', onKey)
  await nextTick()
  if (panelRef.value) {
    releaseTrap()
    trap = createFocusTrap(panelRef.value)
    if (props.requireText) inputRef.value?.focus()
    else primaryRef.value?.focus()
  }
}, { immediate: true })

onBeforeUnmount(() => {
  window.removeEventListener('keydown', onKey)
  releaseTrap()
  document.body.style.overflow = ''
})
</script>

<template>
  <div
    v-if="open"
    class="fixed inset-0 z-[60] flex items-center justify-center bg-black/40 p-3 sm:p-4"
    role="presentation"
    @click.self="close"
  >
    <div
      ref="panelRef"
      class="w-full max-w-md rounded-xl bg-white p-4 shadow-xl dark:bg-slate-900 sm:p-5"
      role="dialog"
      aria-modal="true"
      :aria-labelledby="titleId"
    >
      <h3 :id="titleId" class="text-sm font-semibold text-slate-800 dark:text-slate-100">{{ title }}</h3>
      <p class="mt-2 whitespace-pre-wrap text-sm text-slate-600 dark:text-slate-300">{{ message }}</p>
      <div v-if="requireText" class="mt-3">
        <label class="mb-1 block text-xs text-slate-500 dark:text-slate-400">
          请输入 <span class="font-mono text-slate-700 dark:text-slate-200">{{ requireText }}</span> 确认
        </label>
        <input
          ref="inputRef"
          v-model="input"
          class="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm focus:border-slate-500 focus:outline-none dark:border-slate-600 dark:bg-slate-800"
          @keyup.enter="onConfirm"
        />
      </div>
      <div class="mt-4 flex flex-wrap justify-end gap-2">
        <button
          type="button"
          class="rounded-lg border border-slate-200 px-3 py-1.5 text-sm text-slate-600 hover:bg-slate-50 disabled:opacity-50 dark:border-slate-700 dark:text-slate-300 dark:hover:bg-slate-800"
          :disabled="loading"
          @click="close"
        >{{ cancelLabel }}</button>
        <button
          ref="primaryRef"
          type="button"
          class="rounded-lg px-4 py-2 text-sm font-medium text-white disabled:opacity-50"
          :class="danger ? 'bg-red-600 hover:bg-red-500' : 'bg-slate-800 hover:bg-slate-700'"
          :disabled="!canConfirm"
          @click="onConfirm"
        >{{ loading ? '处理中…' : confirmLabel }}</button>
      </div>
    </div>
  </div>
</template>
