<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { useAuth } from '@/composables/useAuth'
import { useI18n } from '@/composables/useI18n'
import type { MessageKey } from '@/i18n/messages'
import { createFocusTrap, type FocusTrapHandle } from '@/utils/focusTrap'
import {
  COMMAND_NAV_ITEMS,
  filterCommandItems,
  matchCommandPaletteClose,
  matchCommandPaletteOpen,
  visibleCommandItems,
  type CommandNavItem,
} from '@/utils/commandPalette'
import { Search, Command } from 'lucide-vue-next'

const open = ref(false)
const query = ref('')
const activeIndex = ref(0)
const panelRef = ref<HTMLElement | null>(null)
const inputRef = ref<HTMLInputElement | null>(null)
let trap: FocusTrapHandle | null = null

const router = useRouter()
const { isAdmin } = useAuth()
const { t } = useI18n()

const items = computed(() => {
  const base = visibleCommandItems(COMMAND_NAV_ITEMS, isAdmin.value)
  return filterCommandItems(base, query.value, (key) => t.value(key as MessageKey) || key)
})

function openPalette() {
  open.value = true
  query.value = ''
  activeIndex.value = 0
  void nextTick(() => {
    if (panelRef.value) {
      trap?.release()
      trap = createFocusTrap(panelRef.value)
      trap.focusFirst()
    }
    inputRef.value?.focus()
    inputRef.value?.select()
  })
}

function closePalette() {
  open.value = false
  query.value = ''
  activeIndex.value = 0
  trap?.release()
  trap = null
}

function go(item: CommandNavItem) {
  closePalette()
  void router.push(item.path)
}

function onGlobalKey(e: KeyboardEvent) {
  if (matchCommandPaletteOpen(e, false)) {
    e.preventDefault()
    if (open.value) closePalette()
    else openPalette()
    return
  }
  if (!open.value) return
  if (matchCommandPaletteClose(e)) {
    e.preventDefault()
    closePalette()
    return
  }
  if (e.key === 'ArrowDown') {
    e.preventDefault()
    if (!items.value.length) return
    activeIndex.value = (activeIndex.value + 1) % items.value.length
    return
  }
  if (e.key === 'ArrowUp') {
    e.preventDefault()
    if (!items.value.length) return
    activeIndex.value = (activeIndex.value - 1 + items.value.length) % items.value.length
    return
  }
  if (e.key === 'Enter') {
    e.preventDefault()
    const item = items.value[activeIndex.value]
    if (item) go(item)
  }
}

watch(items, (list) => {
  if (activeIndex.value >= list.length) activeIndex.value = 0
})

onMounted(() => {
  window.addEventListener('keydown', onGlobalKey)
})
onBeforeUnmount(() => {
  window.removeEventListener('keydown', onGlobalKey)
  trap?.release()
})

defineExpose({ openPalette, closePalette, open })
</script>

<template>
  <div>
    <!-- 供 TopBar 触发 -->
    <button
      type="button"
      class="hidden"
      data-testid="command-palette-open"
      aria-hidden="true"
      tabindex="-1"
      @click="openPalette"
    />

    <div
      v-if="open"
      class="fixed inset-0 z-[120] flex items-start justify-center bg-black/40 p-4 pt-[12vh]"
      data-testid="command-palette"
      @click.self="closePalette"
    >
      <div
        ref="panelRef"
        role="dialog"
        aria-modal="true"
        :aria-label="t('commandPaletteTitle')"
        class="w-full max-w-lg overflow-hidden rounded-xl border border-slate-200 bg-white shadow-2xl dark:border-slate-700 dark:bg-slate-900"
      >
        <div class="flex items-center gap-2 border-b border-slate-100 px-3 py-2 dark:border-slate-800">
          <Search class="h-4 w-4 mts-muted" />
          <input
            ref="inputRef"
            v-model="query"
            type="search"
            class="min-w-0 flex-1 border-0 bg-transparent py-2 text-sm outline-none placeholder:text-slate-400"
            :placeholder="t('commandPalettePlaceholder')"
            data-testid="command-palette-input"
            autocomplete="off"
          />
          <kbd class="hidden rounded border border-slate-200 px-1.5 py-0.5 text-[10px] mts-muted sm:inline dark:border-slate-700">
            Esc
          </kbd>
        </div>
        <ul class="max-h-80 overflow-auto p-1" role="listbox">
          <li v-if="!items.length" class="px-3 py-6 text-center text-sm mts-muted">
            {{ t('commandPaletteEmpty') }}
          </li>
          <li
            v-for="(item, idx) in items"
            :key="item.id"
            role="option"
            :aria-selected="idx === activeIndex"
            class="flex cursor-pointer items-center justify-between gap-2 rounded-lg px-3 py-2 text-sm"
            :class="idx === activeIndex
              ? 'bg-slate-900 text-white dark:bg-slate-100 dark:text-slate-900'
              : 'text-slate-700 hover:bg-slate-100 dark:text-slate-200 dark:hover:bg-slate-800'"
            :data-testid="`command-item-${item.id}`"
            @mouseenter="activeIndex = idx"
            @click="go(item)"
          >
            <span class="font-medium">{{ t(item.labelKey as MessageKey) }}</span>
            <span
              class="font-mono text-[11px]"
              :class="idx === activeIndex ? 'opacity-80' : 'mts-muted'"
            >{{ item.path }}</span>
          </li>
        </ul>
        <div class="flex items-center gap-2 border-t border-slate-100 px-3 py-2 text-[11px] mts-muted dark:border-slate-800">
          <Command class="h-3 w-3" />
          <span>{{ t('commandPaletteHint') }}</span>
        </div>
      </div>
    </div>
  </div>
</template>
