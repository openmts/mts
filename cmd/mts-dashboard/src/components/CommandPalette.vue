<script setup lang="ts">
import { computed, inject, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { useAuth } from '@/composables/useAuth'
import { useI18n } from '@/composables/useI18n'
import type { MessageKey } from '@/i18n/messages'
import { createFocusTrap, type FocusTrapHandle } from '@/utils/focusTrap'
import {
  allVisibleCommandItems,
  filterCommandItems,
  isCommandAction,
  matchCommandPaletteClose,
  matchCommandPaletteOpen,
  recentCommandItems,
  type CommandActionId,
  type CommandNavItem,
} from '@/utils/commandPalette'
import { loadRecentRoutes } from '@/utils/recentRoutes'
import { resolveRouteTitleKey } from '@/utils/pageTitle'
import { useTheme } from '@/composables/useTheme'
import { useDensity } from '@/composables/useDensity'
import { useNotify } from '@/composables/useNotify'
import { Search, Command, History, Zap } from 'lucide-vue-next'

const open = ref(false)
const query = ref('')
const activeIndex = ref(0)
const panelRef = ref<HTMLElement | null>(null)
const inputRef = ref<HTMLInputElement | null>(null)
let trap: FocusTrapHandle | null = null

const router = useRouter()
const { isAdmin } = useAuth()
const { t, toggleLocale } = useI18n()
const { toggleTheme } = useTheme()
const { toggleDensity } = useDensity()
const { success } = useNotify()

const focusSidebarFilter = inject<(() => void) | undefined>('focusSidebarFilter', undefined)
const openNotifyHistory = inject<(() => void) | undefined>('openNotifyHistory', undefined)
const openShortcutsHelp = inject<(() => void) | undefined>('openShortcutsHelp', undefined)
const toggleSidebarCollapse = inject<(() => void) | undefined>('toggleSidebarCollapse', undefined)
const scrollMainToTop = inject<(() => void) | undefined>('scrollMainToTop', undefined)

const items = computed(() => {
  const base = allVisibleCommandItems(isAdmin.value)
  return filterCommandItems(base, query.value, (key) => t.value(key as MessageKey) || key)
})

const recentItems = computed(() => {
  if (query.value.trim()) return []
  return recentCommandItems(loadRecentRoutes(), 5)
})

function recentLabel(name: string, path: string): string {
  const key = resolveRouteTitleKey(name || null)
  if (key) return t.value(key)
  return path
}

function goRecent(path: string) {
  closePalette()
  void router.push(path)
}

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

function runAction(action: CommandActionId) {
  switch (action) {
    case 'toggle-theme':
      toggleTheme()
      success(t.value('cmdActionThemeToggled'))
      break
    case 'toggle-locale':
      toggleLocale()
      success(t.value('cmdActionLocaleToggled'))
      break
    case 'toggle-density':
      toggleDensity()
      success(t.value('cmdActionDensityToggled'))
      break
    case 'focus-sidebar-filter':
      focusSidebarFilter?.()
      break
    case 'open-notify-history':
      openNotifyHistory?.()
      break
    case 'open-shortcuts':
      openShortcutsHelp?.()
      break
    case 'toggle-sidebar-collapse':
      toggleSidebarCollapse?.()
      success(t.value('cmdActionSidebarCollapseToggled'))
      break
    case 'scroll-main-to-top':
      scrollMainToTop?.()
      break
    default:
      break
  }
}

function go(item: CommandNavItem) {
  closePalette()
  if (isCommandAction(item) && item.action) {
    runAction(item.action)
    return
  }
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
            class="mts-focus-ring min-w-0 flex-1 border-0 bg-transparent py-2 text-sm outline-none placeholder:text-slate-400"
            :placeholder="t('commandPalettePlaceholder')"
            data-testid="command-palette-input"
            role="combobox"
            aria-autocomplete="list"
            aria-controls="command-palette-listbox"
            :aria-expanded="open ? 'true' : 'false'"
            :aria-activedescendant="items[activeIndex] ? `command-option-${items[activeIndex].id}` : undefined"
            autocomplete="off"
          />
          <kbd class="hidden rounded border border-slate-200 px-1.5 py-0.5 text-[10px] mts-muted sm:inline dark:border-slate-700">
            Esc
          </kbd>
        </div>
        <ul id="command-palette-listbox" class="max-h-80 overflow-auto p-1" role="listbox" data-testid="command-palette-listbox">
          <li
            v-if="recentItems.length"
            class="px-2 pb-1 pt-2 text-[10px] font-semibold uppercase tracking-wide mts-muted"
            data-testid="command-palette-recent-label"
          >
            <span class="inline-flex items-center gap-1">
              <History class="h-3 w-3" aria-hidden="true" />
              {{ t('commandPaletteRecent') }}
            </span>
          </li>
          <li
            v-for="r in recentItems"
            :key="r.id"
            role="option"
            class="flex cursor-pointer items-center justify-between gap-2 rounded-lg px-3 py-2 text-sm text-slate-700 hover:bg-slate-100 dark:text-slate-200 dark:hover:bg-slate-800"
            :data-testid="`command-recent-${r.path}`"
            :data-pinned="r.pinned ? 'true' : 'false'"
            @click="goRecent(r.path)"
          >
            <span class="inline-flex items-center gap-1 font-medium">
              <span v-if="r.pinned" class="text-sky-600 dark:text-sky-300" aria-hidden="true">★</span>
              {{ recentLabel(r.name, r.path) }}
            </span>
            <span class="font-mono text-[11px] mts-muted">{{ r.path }}</span>
          </li>
          <li v-if="recentItems.length" class="my-1 border-t border-slate-100 dark:border-slate-800" aria-hidden="true" />
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
            :id="`command-option-${item.id}`"
            :data-testid="`command-item-${item.id}`"
            @mouseenter="activeIndex = idx"
            @click="go(item)"
          >
            <span class="inline-flex min-w-0 items-center gap-1.5 font-medium">
              <Zap v-if="isCommandAction(item)" class="h-3.5 w-3.5 shrink-0 opacity-80" aria-hidden="true" />
              <span class="truncate">{{ t(item.labelKey as MessageKey) }}</span>
            </span>
            <span
              class="shrink-0 font-mono text-[11px]"
              :class="idx === activeIndex ? 'opacity-80' : 'mts-muted'"
            >{{ isCommandAction(item) ? t('commandPaletteActionBadge') : item.path }}</span>
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
