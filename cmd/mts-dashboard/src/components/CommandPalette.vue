<script setup lang="ts">
import { computed, inject, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { useAuth } from '@/composables/useAuth'
import { useI18n } from '@/composables/useI18n'
import type { MessageKey } from '@/i18n/messages'
import { createFocusTrap, type FocusTrapHandle } from '@/utils/focusTrap'
import {
  allVisibleCommandItems,
  commandItemIndexMap,
  commandListKeyFromEvent,
  applyEmptyQueryNavCollapse,
  filterCommandItems,
  flattenCommandGroups,
  groupCommandItems,
  isCommandAction,
  matchCommandPaletteClose,
  matchCommandPaletteOpen,
  moveCommandActiveIndex,
  recentCommandItems,
  type CommandActionId,
  type CommandNavItem,
} from '@/utils/commandPalette'
import { loadRecentRoutes } from '@/utils/recentRoutes'
import { resolveRouteTitleKey } from '@/utils/pageTitle'
import { useTheme } from '@/composables/useTheme'
import { useDensity } from '@/composables/useDensity'
import { useNotify } from '@/composables/useNotify'
import { formatMessage } from '@/utils/formatMessage'
import { copyText } from '@/utils/clipboard'
import {
  clickShareLinkButton,
  pickShareLinkButton,
  resolveShareDeepLinkAction,
  stripSensitiveUrlParams,
} from '@/utils/shareDeepLink'
import { pathOpensShortcutsHelp } from '@/utils/shortcutsPrefill'
import { Search, Command, History, Zap } from 'lucide-vue-next'

const open = ref(false)
const query = ref('')
const navExpanded = ref(false)
const activeIndex = ref(0)
const panelRef = ref<HTMLElement | null>(null)
const inputRef = ref<HTMLInputElement | null>(null)
let trap: FocusTrapHandle | null = null

const router = useRouter()
const { isAdmin } = useAuth()
const { t, toggleLocale } = useI18n()
const { toggleTheme } = useTheme()
const { density, toggleDensity } = useDensity()
const { success, error: notifyError } = useNotify()

const focusSidebarFilter = inject<(() => void) | undefined>('focusSidebarFilter', undefined)
const openNotifyHistory = inject<(() => void) | undefined>('openNotifyHistory', undefined)
const openShortcutsHelp = inject<(() => void) | undefined>('openShortcutsHelp', undefined)
const toggleSidebarCollapse = inject<(() => void) | undefined>('toggleSidebarCollapse', undefined)
const scrollMainToTop = inject<(() => void) | undefined>('scrollMainToTop', undefined)

const filteredItems = computed(() => {
  const base = allVisibleCommandItems(isAdmin.value)
  return filterCommandItems(base, query.value, (key) => t.value(key as MessageKey) || key)
})

const emptyQuery = computed(() => !query.value.trim())

const collapsedView = computed(() => {
  const groups = groupCommandItems(filteredItems.value)
  if (!emptyQuery.value) {
    return { groups, navHiddenCount: 0, navDeepLinkCount: 0 }
  }
  return applyEmptyQueryNavCollapse(groups, navExpanded.value)
})

const itemGroups = computed(() => collapsedView.value.groups)
const navHiddenCount = computed(() => collapsedView.value.navHiddenCount)
const navDeepLinkCount = computed(() => collapsedView.value.navDeepLinkCount)

/** 键盘/选中索引用的扁平列表：导航在前、动作在后 */
const items = computed(() => flattenCommandGroups(itemGroups.value))
const matchedCount = computed(() => items.value.length)

function groupCountLabel(count: number): string {
  return String(count)
}

function expandNavLabel(): string {
  const n = navHiddenCount.value || navDeepLinkCount.value
  return formatMessage(t.value('commandPaletteNavExpand'), { count: n })
}

const itemIndexById = computed(() => commandItemIndexMap(items.value))

function flatIndexOf(item: CommandNavItem): number {
  return itemIndexById.value.get(item.id) ?? -1
}

function scrollActiveOptionIntoView() {
  void nextTick(() => {
    const item = items.value[activeIndex.value]
    if (!item || typeof document === 'undefined') return
    const el = document.getElementById(`command-option-${item.id}`)
    el?.scrollIntoView({ block: 'nearest' })
  })
}

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
  navExpanded.value = false
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
  navExpanded.value = false
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
    case 'copy-page-url': {
      const href = typeof window !== 'undefined' ? stripSensitiveUrlParams(window.location.href) : ''
      void copyText(href).then((r) => {
        if (r.ok) success(t.value('cmdActionUrlCopied'))
        else notifyError(t.value('cmdActionUrlCopyFailed'))
      })
      break
    }
    case 'click-share-deep-link': {
      const href = typeof window !== 'undefined' ? window.location.href : ''
      const root = typeof document !== 'undefined' ? document : null
      const decision = resolveShareDeepLinkAction({ root, href })
      if (decision.kind === 'clicked') {
        const btn = pickShareLinkButton(root)
        if (btn) {
          // 页内 share 按钮自行 toast，避免重复提示
          clickShareLinkButton(btn)
        } else {
          notifyError(t.value('cmdActionShareDeepLinkFailed'))
        }
        break
      }
      if (decision.kind === 'fallback-url') {
        void copyText(decision.href).then((r) => {
          if (r.ok) success(t.value('cmdActionShareDeepLinkFallback'))
          else notifyError(t.value('cmdActionUrlCopyFailed'))
        })
        break
      }
      notifyError(t.value('cmdActionShareDeepLinkFailed'))
      break
    }
    case 'focus-main': {
      if (typeof document !== 'undefined') {
        const el = document.getElementById('main-content') as HTMLElement | null
        el?.focus()
      }
      break
    }
    case 'reload-page':
      success(t.value('cmdActionReloading'))
      if (typeof window !== 'undefined') window.location.reload()
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
  // 同路径 router.push 不触发 watch：深链目标仍强制打开面板
  if (pathOpensShortcutsHelp(item.path)) {
    openShortcutsHelp?.()
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
  const listKey = commandListKeyFromEvent(e)
  if (listKey !== 'none') {
    e.preventDefault()
    if (!items.value.length) return
    activeIndex.value = moveCommandActiveIndex(activeIndex.value, items.value.length, listKey)
    scrollActiveOptionIntoView()
    return
  }
  if (e.key === 'Enter') {
    e.preventDefault()
    const item = items.value[activeIndex.value]
    if (item) go(item)
  }
}

watch(query, () => {
  if (!query.value.trim()) navExpanded.value = false
  activeIndex.value = 0
})

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
        :data-density="density"
        data-testid="command-palette-panel"
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
        <ul
          id="command-palette-listbox"
          class="max-h-[min(28rem,55vh)] overflow-auto p-1"
          role="listbox"
          data-testid="command-palette-listbox"
          :data-density="density"
        >
          <li
            v-if="recentItems.length"
            class="sticky top-0 z-10 bg-white/95 px-2 pb-1 pt-1.5 text-[10px] font-semibold uppercase tracking-wide mts-muted backdrop-blur dark:bg-slate-900/95"
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
            class="flex cursor-pointer items-center justify-between gap-2 rounded-md px-2.5 text-slate-700 hover:bg-slate-100 dark:text-slate-200 dark:hover:bg-slate-800"
            :class="density === 'compact' ? 'py-1 text-xs' : 'py-1.5 text-sm'"
            :data-testid="`command-recent-${r.path}`"
            :data-pinned="r.pinned ? 'true' : 'false'"
            @click="goRecent(r.path)"
          >
            <span class="inline-flex min-w-0 items-center gap-1 font-medium">
              <span v-if="r.pinned" class="text-sky-600 dark:text-sky-300" aria-hidden="true">★</span>
              <span class="truncate">{{ recentLabel(r.name, r.path) }}</span>
            </span>
            <span class="shrink-0 font-mono text-[10px] mts-muted">{{ r.path }}</span>
          </li>
          <li v-if="recentItems.length" class="my-0.5 border-t border-slate-100 dark:border-slate-800" aria-hidden="true" />
          <li v-if="!items.length" class="px-3 py-5 text-center text-sm mts-muted" data-testid="command-palette-empty">
            {{ t('commandPaletteEmpty') }}
          </li>
          <template v-for="group in itemGroups" :key="group.id">
            <li
              class="sticky top-0 z-10 flex items-center justify-between gap-2 bg-white/95 px-2 pb-0.5 pt-1.5 text-[10px] font-semibold uppercase tracking-wide mts-muted backdrop-blur dark:bg-slate-900/95"
              role="presentation"
              :data-testid="`command-palette-group-${group.id}`"
            >
              <span class="inline-flex items-center gap-1.5">
                <span
                  class="inline-block h-1.5 w-1.5 rounded-full"
                  :class="group.id === 'action' ? 'bg-amber-400' : 'bg-sky-400'"
                  aria-hidden="true"
                />
                {{ t(group.labelKey as MessageKey) }}
              </span>
              <span
                class="rounded-full bg-slate-100 px-1.5 py-0.5 font-mono text-[10px] normal-case tracking-normal text-slate-500 dark:bg-slate-800 dark:text-slate-400"
                :data-testid="`command-palette-group-${group.id}-count`"
              >{{ groupCountLabel(group.items.length) }}</span>
            </li>
            <li
              v-for="item in group.items"
              :key="item.id"
              role="option"
              :aria-selected="flatIndexOf(item) === activeIndex"
              class="flex cursor-pointer items-center justify-between gap-2 rounded-md px-2.5"
              :class="[
                density === 'compact' ? 'py-1 text-xs' : 'py-1.5 text-sm',
                flatIndexOf(item) === activeIndex
                  ? 'bg-slate-900 text-white dark:bg-slate-100 dark:text-slate-900'
                  : 'text-slate-700 hover:bg-slate-100 dark:text-slate-200 dark:hover:bg-slate-800',
              ]"
              :id="`command-option-${item.id}`"
              :data-testid="`command-item-${item.id}`"
              @mouseenter="activeIndex = flatIndexOf(item)"
              @click="go(item)"
            >
              <span class="inline-flex min-w-0 items-center gap-1.5 font-medium">
                <Zap v-if="isCommandAction(item)" class="h-3.5 w-3.5 shrink-0 opacity-80" aria-hidden="true" />
                <span class="truncate">{{ t(item.labelKey as MessageKey) }}</span>
              </span>
              <span
                class="shrink-0 font-mono text-[10px]"
                :class="flatIndexOf(item) === activeIndex ? 'opacity-80' : 'mts-muted'"
              >{{ isCommandAction(item) ? t('commandPaletteActionBadge') : item.path }}</span>
            </li>
            <li
              v-if="group.id === 'nav' && emptyQuery && navDeepLinkCount > 0"
              class="px-2 py-1"
              role="presentation"
            >
              <button
                type="button"
                class="mts-focus-ring w-full rounded-lg border border-dashed border-slate-200 px-3 py-1.5 text-left text-xs text-slate-600 hover:bg-slate-50 dark:border-slate-700 dark:text-slate-300 dark:hover:bg-slate-800"
                data-testid="command-palette-nav-expand"
                @click="navExpanded = !navExpanded; activeIndex = 0"
              >
                {{ navExpanded ? t('commandPaletteNavCollapse') : expandNavLabel() }}
              </button>
            </li>
          </template>
        </ul>
        <div class="flex flex-wrap items-center gap-x-2 gap-y-1 border-t border-slate-100 px-3 py-1.5 text-[11px] mts-muted dark:border-slate-800">
          <Command class="h-3 w-3 shrink-0" />
          <span class="min-w-0 flex-1">{{ t('commandPaletteHint') }}</span>
          <span
            class="shrink-0 rounded-full bg-slate-100 px-2 py-0.5 font-mono text-[10px] text-slate-600 dark:bg-slate-800 dark:text-slate-300"
            data-testid="command-palette-result-count"
          >{{ formatMessage(t('commandPaletteResultCount'), { count: matchedCount }) }}</span>
        </div>
      </div>
    </div>
  </div>
</template>
