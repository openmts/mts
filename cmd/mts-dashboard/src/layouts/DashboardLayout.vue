<script setup lang="ts">
import { onMounted, onBeforeUnmount, provide, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import SidebarNav from '@/components/SidebarNav.vue'
import TopBar from '@/components/TopBar.vue'
import PageSkeleton from '@/components/PageSkeleton.vue'
import CommandPalette from '@/components/CommandPalette.vue'
import ShortcutsHelp from '@/components/ShortcutsHelp.vue'
import { useI18n } from '@/composables/useI18n'
import type { MessageKey } from '@/i18n/messages'
import { resolveRouteTitleKey } from '@/utils/pageTitle'
import {
  isEditableTarget,
  matchShortcutHelpOpen,
} from '@/utils/keyboardShortcuts'
import {
  loadRecentRoutes,
  recordRecentRoute,
  type RecentRouteEntry,
} from '@/utils/recentRoutes'

const { t } = useI18n()
const route = useRoute()
const router = useRouter()
const sidebarOpen = ref(false)
const commandPaletteRef = ref<InstanceType<typeof CommandPalette> | null>(null)
const shortcutsOpen = ref(false)
const recent = ref<RecentRouteEntry[]>(loadRecentRoutes())

function toggleSidebar() { sidebarOpen.value = !sidebarOpen.value }
function closeSidebar() { sidebarOpen.value = false }
function openCommandPalette() { commandPaletteRef.value?.openPalette() }
function openShortcuts() { shortcutsOpen.value = true }

function recentLabel(entry: RecentRouteEntry): string {
  const key = resolveRouteTitleKey(entry.name || null)
  if (key) return t.value(key as MessageKey)
  return entry.path
}

function goRecent(path: string) {
  void router.push(path)
}

function onGlobalKey(e: KeyboardEvent) {
  if (shortcutsOpen.value && e.key === 'Escape') {
    e.preventDefault()
    shortcutsOpen.value = false
    return
  }
  if (matchShortcutHelpOpen(e, isEditableTarget(e.target))) {
    e.preventDefault()
    shortcutsOpen.value = !shortcutsOpen.value
  }
}

watch(
  () => [route.fullPath, route.name] as const,
  ([path, name]) => {
    recent.value = recordRecentRoute(path, name)
  },
  { immediate: true },
)

onMounted(() => {
  window.addEventListener('keydown', onGlobalKey)
})
onBeforeUnmount(() => {
  window.removeEventListener('keydown', onGlobalKey)
})

provide('toggleSidebar', toggleSidebar)
provide('closeSidebar', closeSidebar)
provide('openCommandPalette', openCommandPalette)
provide('openShortcutsHelp', openShortcuts)
</script>

<template>
  <div class="flex h-screen bg-slate-50 dark:bg-slate-950">
    <a
      href="#main-content"
      class="sr-only focus:not-sr-only focus:absolute focus:left-3 focus:top-3 focus:z-[200] focus:rounded-lg focus:bg-white focus:px-3 focus:py-2 focus:text-sm focus:font-medium focus:text-slate-900 focus:shadow dark:focus:bg-slate-800 dark:focus:text-slate-100"
      data-testid="skip-to-main"
    >
      {{ t('skipToMain') }}
    </a>
    <SidebarNav :visible="sidebarOpen" @close="closeSidebar" />
    <div class="flex flex-1 flex-col overflow-hidden">
      <TopBar
        @toggle-sidebar="toggleSidebar"
        @open-command-palette="openCommandPalette"
        @open-shortcuts="openShortcuts"
      />
      <div
        v-if="recent.length"
        class="flex flex-wrap items-center gap-2 border-b border-slate-200 bg-white px-3 py-1.5 dark:border-slate-700 dark:bg-slate-900 sm:px-6"
        data-testid="recent-routes"
      >
        <span class="text-[11px] mts-muted">{{ t('recentRoutes') }}</span>
        <button
          v-for="r in recent.slice(0, 6)"
          :key="r.path + r.at"
          type="button"
          class="rounded-full border border-slate-200 px-2 py-0.5 text-[11px] text-slate-600 hover:bg-slate-50 dark:border-slate-700 dark:text-slate-300 dark:hover:bg-slate-800"
          :data-testid="`recent-route-${r.path}`"
          @click="goRecent(r.path)"
        >
          {{ recentLabel(r) }}
        </button>
      </div>
      <main id="main-content" class="flex-1 overflow-auto p-4 sm:p-6" tabindex="-1">
        <RouterView v-slot="{ Component }">
          <Suspense>
            <component :is="Component" />
            <template #fallback>
              <PageSkeleton />
            </template>
          </Suspense>
        </RouterView>
      </main>
    </div>
    <CommandPalette ref="commandPaletteRef" />
    <ShortcutsHelp v-model:open="shortcutsOpen" />
  </div>
</template>
