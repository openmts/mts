<script setup lang="ts">
import { nextTick, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from '@/composables/useI18n'
import type { MessageKey } from '@/i18n/messages'
import { createFocusTrap, type FocusTrapHandle } from '@/utils/focusTrap'
import { DASHBOARD_SHORTCUTS } from '@/utils/keyboardShortcuts'
import { Keyboard, X } from 'lucide-vue-next'

const open = defineModel<boolean>('open', { default: false })
const { t } = useI18n()
const panelRef = ref<HTMLElement | null>(null)
let trap: FocusTrapHandle | null = null

function close() {
  open.value = false
}

watch(open, async (v) => {
  trap?.release()
  trap = null
  if (!v) {
    document.body.style.overflow = ''
    return
  }
  document.body.style.overflow = 'hidden'
  await nextTick()
  if (panelRef.value) {
    trap = createFocusTrap(panelRef.value)
    trap.focusFirst()
  }
})

onBeforeUnmount(() => {
  trap?.release()
  trap = null
  document.body.style.overflow = ''
})
</script>

<template>
  <div
    v-if="open"
    class="fixed inset-0 z-[130] flex items-center justify-center bg-black/40 p-4"
    data-testid="shortcuts-help"
    @click.self="close"
    @keydown.esc.prevent="close"
  >
    <div
      ref="panelRef"
      class="w-full max-w-md rounded-xl border border-slate-200 bg-white p-5 shadow-xl dark:border-slate-700 dark:bg-slate-900"
      role="dialog"
      aria-modal="true"
      :aria-label="t('shortcutHelpTitle')"
    >
      <div class="mb-3 flex items-center justify-between gap-2">
        <h2 class="flex items-center gap-2 text-sm font-semibold text-slate-800 dark:text-slate-100">
          <Keyboard class="h-4 w-4" />
          {{ t('shortcutHelpTitle') }}
        </h2>
        <button type="button" class="rounded p-1 text-slate-400 hover:bg-slate-100 dark:hover:bg-slate-800" data-testid="shortcuts-help-close" @click="close">
          <X class="h-4 w-4" />
        </button>
      </div>
      <p class="mb-3 text-xs mts-muted">{{ t('shortcutHelpDesc') }}</p>
      <ul class="space-y-2">
        <li
          v-for="s in DASHBOARD_SHORTCUTS"
          :key="s.id"
          class="flex items-center justify-between gap-3 rounded-lg border border-slate-100 px-3 py-2 text-sm dark:border-slate-800"
        >
          <span class="text-slate-700 dark:text-slate-200">{{ t(s.labelKey as MessageKey) }}</span>
          <kbd class="rounded bg-slate-100 px-2 py-0.5 font-mono text-[11px] text-slate-700 dark:bg-slate-800 dark:text-slate-200">{{ s.keys }}</kbd>
        </li>
      </ul>
    </div>
  </div>
</template>
