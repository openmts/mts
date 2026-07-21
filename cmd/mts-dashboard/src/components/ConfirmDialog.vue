<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue'
import { createFocusTrap, type FocusTrapHandle } from '@/utils/focusTrap'
import { useI18n } from '@/composables/useI18n'
import { formatMessage } from '@/utils/formatMessage'
import { mutationBlockedMessageKey } from '@/utils/mutationGuard'
import type { MessageKey } from '@/i18n/messages'

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
  /** 离线或会话 critical/expired 时禁用确认 */
  writeBlocked?: boolean
  blockReason?: 'none' | 'offline' | 'session' | null
  /** 离线场景 i18n key；session 时固定 sessionMutationBlocked */
  offlineMessageKey?: MessageKey
  /** 加载中是否允许取消（长请求 abort） */
  allowCancelWhileLoading?: boolean
}>(), {
  confirmLabel: '',
  cancelLabel: '',
  danger: false,
  requireText: '',
  loading: false,
  writeBlocked: false,
  blockReason: 'none',
  offlineMessageKey: 'offlineAdminBlocked',
  allowCancelWhileLoading: false,
})

const emit = defineEmits<{
  'update:open': [boolean]
  confirm: []
  cancel: []
}>()

const { t } = useI18n()
const input = ref('')
const inputRef = ref<HTMLInputElement | null>(null)
const primaryRef = ref<HTMLButtonElement | null>(null)
const panelRef = ref<HTMLElement | null>(null)
const uid = Math.random().toString(36).slice(2, 9)
const titleId = `confirm-title-${uid}`
const descId = `confirm-desc-${uid}`
let trap: FocusTrapHandle | null = null

const resolvedConfirm = computed(() => props.confirmLabel || t.value('confirm'))
const resolvedCancel = computed(() => props.cancelLabel || t.value('cancel'))
const requireHint = computed(() =>
  props.requireText ? formatMessage(t.value('typeToConfirm'), { text: props.requireText }) : '',
)

const blockedTitle = computed(() => {
  if (!props.writeBlocked) return undefined
  const key = mutationBlockedMessageKey(
    props.blockReason === 'session' || props.blockReason === 'offline' ? props.blockReason : 'offline',
    props.offlineMessageKey || 'offlineAdminBlocked',
  )
  return t.value(key)
})

const canConfirm = computed(() => {
  if (props.writeBlocked) return false
  if (props.loading) return false
  if (!props.requireText) return true
  return input.value === props.requireText
})

function releaseTrap() {
  trap?.release()
  trap = null
}

function close() {
  if (props.loading && !props.allowCancelWhileLoading) return
  emit('update:open', false)
  emit('cancel')
  input.value = ''
}

function onConfirm() {
  if (props.writeBlocked || !canConfirm.value) return
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
    data-testid="confirm-dialog-backdrop"
    @click.self="close"
  >
    <div
      ref="panelRef"
      class="w-full max-w-md rounded-xl bg-white p-4 shadow-xl dark:bg-slate-900 sm:p-5"
      role="dialog"
      aria-modal="true"
      :aria-labelledby="titleId"
      :aria-describedby="descId"
      data-testid="confirm-dialog"
    >
      <h3 :id="titleId" class="text-sm font-semibold text-slate-800 dark:text-slate-100">{{ title }}</h3>
      <p :id="descId" class="mt-2 whitespace-pre-wrap text-sm text-slate-600 dark:text-slate-300">{{ message }}</p>
      <p
        v-if="loading"
        class="mt-2 rounded-lg border border-sky-200 bg-sky-50 px-2 py-1.5 text-[11px] text-sky-900 dark:border-sky-900/50 dark:bg-sky-950/40 dark:text-sky-100"
        data-testid="confirm-dialog-loading"
        role="status"
      >{{ t('processing') }}{{ allowCancelWhileLoading ? ` · ${t('confirmCancelWhileLoading')}` : '' }}</p>
      <p
        v-if="writeBlocked && blockedTitle"
        class="mt-2 rounded-lg border border-amber-200 bg-amber-50 px-2 py-1.5 text-[11px] text-amber-900 dark:border-amber-900 dark:bg-amber-950/40 dark:text-amber-100"
        data-testid="confirm-dialog-blocked"
      >{{ blockedTitle }}</p>
      <div v-if="requireText" class="mt-3">
        <label class="mb-1 block text-xs text-slate-500 dark:text-slate-400" :for="`confirm-require-${uid}`">
          {{ requireHint }}
        </label>
        <input
          :id="`confirm-require-${uid}`"
          ref="inputRef"
          v-model="input"
          class="mts-input mts-focus-ring w-full"
          data-testid="confirm-dialog-input"
          autocomplete="off"
          @keyup.enter="onConfirm"
        />
      </div>
      <div class="mt-4 flex flex-wrap justify-end gap-2">
        <button
          type="button"
          class="mts-btn mts-focus-ring"
          data-testid="confirm-dialog-cancel"
          :disabled="loading && !allowCancelWhileLoading"
          @click="close"
        >{{ resolvedCancel }}</button>
        <button
          ref="primaryRef"
          type="button"
          class="mts-focus-ring rounded-lg px-4 py-2 text-sm font-medium text-white disabled:opacity-50"
          :class="danger ? 'bg-red-600 hover:bg-red-500' : 'bg-slate-800 hover:bg-slate-700 dark:bg-slate-100 dark:text-slate-900 dark:hover:bg-white'"
          data-testid="confirm-dialog-confirm"
          :disabled="!canConfirm"
          :title="blockedTitle"
          :aria-busy="loading ? 'true' : undefined"
          @click="onConfirm"
        >{{ loading ? t('processing') : resolvedConfirm }}</button>
      </div>
    </div>
  </div>
</template>
