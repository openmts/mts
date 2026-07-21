<script setup lang="ts">
import { computed, onMounted, onBeforeUnmount, provide, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import SidebarNav from '@/components/SidebarNav.vue'
import TopBar from '@/components/TopBar.vue'
import PageSkeleton from '@/components/PageSkeleton.vue'
import CommandPalette from '@/components/CommandPalette.vue'
import ShortcutsHelp from '@/components/ShortcutsHelp.vue'
import NotifyHistoryPanel from '@/components/NotifyHistoryPanel.vue'
import { parseNotifyHistoryPrefill, type NotifyHistoryPrefill } from '@/utils/notifyHistoryPrefill'
import { parseShortcutsPrefill } from '@/utils/shortcutsPrefill'
import BreadcrumbBar from '@/components/BreadcrumbBar.vue'
import { useI18n } from '@/composables/useI18n'
import type { MessageKey } from '@/i18n/messages'
import { resolveRouteTitleKey } from '@/utils/pageTitle'
import {
  isEditableTarget,
  matchNotifyHistoryOpen,
  matchShortcutHelpOpen,
  matchSidebarFilterFocus,
} from '@/utils/keyboardShortcuts'
import {
  clearRecentRoutes,
  loadRecentRoutes,
  recordRecentRoute,
  setRecentRoutePinned,
  type RecentRouteEntry,
} from '@/utils/recentRoutes'
import { loadSidebarPrefs, saveSidebarPrefs } from '@/utils/sidebarPrefs'
import { CLIENT_PREFS_CHANGED_EVENT } from '@/utils/clientPrefs'
import { scrollElementToTop, shouldResetScrollOnRouteChange, shouldShowBackToTop } from '@/utils/scrollTop'
import { ArrowUp } from 'lucide-vue-next'
import { useMutationGuard } from '@/composables/useMutationGuard'
import { useNetworkStatus } from '@/composables/useNetworkStatus'
import { useAuth } from '@/composables/useAuth'
import { useAdminOpBusy } from '@/composables/useAdminOpBusy'
import { adminOpKindLabelKey, formatAdminOpElapsed } from '@/utils/adminOpBusy'
import { formatMessage } from '@/utils/formatMessage'
import { buildLoginLocation } from '@/utils/redirect'
import { useServerReachability } from '@/composables/useServerReachability'
import { shouldSyncOnVisibility } from '@/utils/pageVisibilitySync'
import { registerOpenNotifyHistory } from '@/utils/notifyHistoryBridge'

const { t } = useI18n()
const { offline, sessionWriteBlocked, sessionRemainingLabel, sessionUrgency } = useMutationGuard()
const { sync: syncNetworkStatus } = useNetworkStatus()
const { logout, isAdmin } = useAuth()
const {
  adminOpBusy,
  adminOpKind,
  adminOpStartedAtUnix,
  adminOpBusyError,
  refreshAdminOpBusy,
} = useAdminOpBusy()

/** busy 期间 1s tick，让横幅 elapsed 实时跳动 */
const adminOpNowMs = ref(Date.now())
let adminOpTickTimer: ReturnType<typeof setInterval> | null = null
function armAdminOpTick() {
  if (adminOpTickTimer) return
  adminOpNowMs.value = Date.now()
  adminOpTickTimer = setInterval(() => {
    adminOpNowMs.value = Date.now()
  }, 1000)
}
function disarmAdminOpTick() {
  if (adminOpTickTimer) clearInterval(adminOpTickTimer)
  adminOpTickTimer = null
}
watch(
  adminOpBusy,
  (busy) => {
    if (busy) armAdminOpTick()
    else disarmAdminOpTick()
  },
  { immediate: true },
)

const adminOpKindLabel = computed(() => {
  if (!adminOpBusy.value) return ''
  const opKey = adminOpKindLabelKey(adminOpKind.value) as MessageKey
  return t.value(opKey) || t.value('adminOpKindGeneric')
})

const adminOpElapsedLabel = computed(() => {
  if (!adminOpBusy.value) return ''
  return formatAdminOpElapsed(adminOpStartedAtUnix.value, adminOpNowMs.value)
})

const adminOpBusyDetail = computed(() => {
  if (!adminOpBusy.value) return ''
  return formatMessage(t.value('adminOpBusyBannerDetail'), {
    op: String(adminOpKindLabel.value || t.value('adminOpKindGeneric')),
    elapsed: adminOpElapsedLabel.value || '—',
  })
})

const adminOpBusyPollErrorLabel = computed(() => {
  const err = (adminOpBusyError.value || '').trim()
  if (!err) return ''
  return formatMessage(t.value('adminOpBusyPollError'), { error: err })
})

provide('adminOpBusySummary', computed(() => ({
  busy: adminOpBusy.value,
  op: adminOpKind.value,
  opLabel: adminOpKindLabel.value,
  elapsed: adminOpElapsedLabel.value,
  detail: adminOpBusyDetail.value,
  pollError: adminOpBusyPollErrorLabel.value,
})))
const { showUnreachableBanner, checkOnce: retryReadyz, checking: reachChecking } = useServerReachability()

function retryNetworkStatus() {
  syncNetworkStatus()
  void retryReadyz()
}

function onVisibilitySync() {
  if (!shouldSyncOnVisibility(document.visibilityState, document.hidden)) return
  syncNetworkStatus()
  void retryReadyz()
  if (isAdmin.value) void refreshAdminOpBusy()
}
const route = useRoute()
const router = useRouter()
const sidebarOpen = ref(false)
const sidebarCollapsed = ref(
  loadSidebarPrefs(typeof localStorage !== 'undefined' ? localStorage : null).collapsed,
)
const commandPaletteRef = ref<InstanceType<typeof CommandPalette> | null>(null)
const sidebarNavRef = ref<InstanceType<typeof SidebarNav> | null>(null)
const mainContentRef = ref<HTMLElement | null>(null)
const showBackToTop = ref(false)
const shortcutsOpen = ref(false)
const notifyHistoryOpen = ref(false)
const notifyHistoryPrefill = ref<NotifyHistoryPrefill | null>(null)
let unregisterNotifyHistoryBridge: (() => void) | null = null
const recent = ref<RecentRouteEntry[]>(loadRecentRoutes())
const showBreadcrumb = computed(() => route.path !== '/')

function toggleSidebar() { sidebarOpen.value = !sidebarOpen.value }
function closeSidebar() { sidebarOpen.value = false }
function toggleSidebarCollapse() {
  sidebarCollapsed.value = !sidebarCollapsed.value
  saveSidebarPrefs(typeof localStorage !== 'undefined' ? localStorage : null, {
    collapsed: sidebarCollapsed.value,
  })
}
function openCommandPalette() { commandPaletteRef.value?.openPalette() }
function goSessionRenew() {
  void router.push({ path: '/account', hash: '#account-session' })
}
function goAdminOpBusyOps() {
  void router.push({ path: '/operations', hash: '#ops-status-strip' })
}
async function goSessionRelogin() {
  await logout()
  await router.replace(
    buildLoginLocation({ reason: 'session', redirectRaw: '/account#account-session' }),
  )
}
function openShortcuts() { shortcutsOpen.value = true }

function recentLabel(entry: RecentRouteEntry): string {
  const key = resolveRouteTitleKey(entry.name || null)
  if (key) return t.value(key as MessageKey)
  return entry.path
}

function goRecent(path: string) {
  void router.push(path)
}

function clearRecent() {
  // 默认保留固定项
  recent.value = clearRecentRoutes()
  // 当前页立即再记入
  recent.value = recordRecentRoute(route.fullPath, route.name)
}

function togglePinRecent(path: string, e?: Event) {
  e?.stopPropagation()
  e?.preventDefault()
  const cur = recent.value.find((x) => x.path === path)
  recent.value = setRecentRoutePinned(path, !cur?.pinned)
}

function openNotifyHistory() {
  notifyHistoryOpen.value = true
}

function applyNotifyHistoryFromRoute() {
  const pre = parseNotifyHistoryPrefill(
    route.query as Record<string, unknown>,
    route.hash,
  )
  notifyHistoryPrefill.value = pre
  if (pre.open) notifyHistoryOpen.value = true
}

function applyShortcutsFromRoute() {
  const pre = parseShortcutsPrefill(
    route.query as Record<string, unknown>,
    route.hash,
  )
  if (pre.open) shortcutsOpen.value = true
}

watch(
  () => route.fullPath,
  () => {
    applyNotifyHistoryFromRoute()
    applyShortcutsFromRoute()
  },
  { immediate: true },
)

function onMainScroll() {
  const el = mainContentRef.value
  showBackToTop.value = shouldShowBackToTop(el?.scrollTop ?? 0)
}

function scrollMainToTop(behavior: ScrollBehavior = 'smooth') {
  scrollElementToTop(mainContentRef.value, behavior)
  showBackToTop.value = false
}


function onGlobalKey(e: KeyboardEvent) {
  if (shortcutsOpen.value && e.key === 'Escape') {
    e.preventDefault()
    shortcutsOpen.value = false
    return
  }
  if (notifyHistoryOpen.value && e.key === 'Escape') {
    e.preventDefault()
    notifyHistoryOpen.value = false
    return
  }
  if (matchNotifyHistoryOpen(e)) {
    e.preventDefault()
    notifyHistoryOpen.value = !notifyHistoryOpen.value
    return
  }
  if (matchShortcutHelpOpen(e, isEditableTarget(e.target))) {
    e.preventDefault()
    shortcutsOpen.value = !shortcutsOpen.value
    return
  }
  if (matchSidebarFilterFocus(e, isEditableTarget(e.target))) {
    e.preventDefault()
    // 移动端抽屉未开时先打开
    if (!sidebarOpen.value && typeof window !== 'undefined' && window.matchMedia('(max-width: 1023px)').matches) {
      sidebarOpen.value = true
    }
    sidebarNavRef.value?.focusFilter()
  }
}

watch(
  () => [route.fullPath, route.name] as const,
  ([path, name]) => {
    recent.value = recordRecentRoute(path, name)
  },
  { immediate: true },
)

// 路径切换时主内容回顶；同页仅 hash 变化保留滚动供锚点定位
watch(
  () => route.path,
  (toPath, fromPath) => {
    if (!shouldResetScrollOnRouteChange(fromPath || '', toPath || '')) return
    // 即时回顶，避免 smooth 与 hash 滚动竞态
    scrollMainToTop('auto')
  },
)

function onPrefsChanged() {
  sidebarCollapsed.value = loadSidebarPrefs(
    typeof localStorage !== 'undefined' ? localStorage : null,
  ).collapsed
}

onMounted(() => {
  document.addEventListener('visibilitychange', onVisibilitySync)
  window.addEventListener('keydown', onGlobalKey)
  window.addEventListener(CLIENT_PREFS_CHANGED_EVENT, onPrefsChanged)
  unregisterNotifyHistoryBridge = registerOpenNotifyHistory(openNotifyHistory)
})
onBeforeUnmount(() => {
  disarmAdminOpTick()
  document.removeEventListener('visibilitychange', onVisibilitySync)
  window.removeEventListener('keydown', onGlobalKey)
  window.removeEventListener(CLIENT_PREFS_CHANGED_EVENT, onPrefsChanged)
  unregisterNotifyHistoryBridge?.()
  unregisterNotifyHistoryBridge = null
})

provide('toggleSidebar', toggleSidebar)
provide('closeSidebar', closeSidebar)
provide('openCommandPalette', openCommandPalette)
provide('openShortcutsHelp', openShortcuts)
provide('openNotifyHistory', openNotifyHistory)
provide('focusSidebarFilter', () => {
  if (!sidebarOpen.value && typeof window !== 'undefined' && window.matchMedia('(max-width: 1023px)').matches) {
    sidebarOpen.value = true
  }
  sidebarNavRef.value?.focusFilter()
})
provide('toggleSidebarCollapse', toggleSidebarCollapse)
provide('scrollMainToTop', scrollMainToTop)

function onSkipToMain(e: Event) {
  e.preventDefault()
  const main = document.getElementById('main-content')
  if (!main) return
  if (!main.hasAttribute('tabindex')) main.setAttribute('tabindex', '-1')
  main.focus({ preventScroll: false })
  try {
    main.scrollIntoView({ block: 'start', behavior: 'smooth' })
  } catch {
    main.scrollIntoView()
  }
}
</script>

<template>
  <div class="flex h-screen bg-slate-50 dark:bg-slate-950">
    <a
      href="#main-content"
      class="sr-only focus:not-sr-only focus:absolute focus:left-3 focus:top-3 focus:z-[200] focus:rounded-lg focus:bg-white focus:px-3 focus:py-2 focus:text-sm focus:font-medium focus:text-slate-900 focus:shadow focus:outline-none focus:ring-2 focus:ring-sky-500 focus:ring-offset-2 focus:ring-offset-white dark:focus:bg-slate-800 dark:focus:text-slate-100 dark:focus:ring-sky-400 dark:focus:ring-offset-slate-950"
      data-testid="skip-to-main"
      @click="onSkipToMain"
    >
      {{ t('skipToMain') }}
    </a>
    <div class="no-print">
      <SidebarNav
        ref="sidebarNavRef"
        :visible="sidebarOpen"
        :collapsed="sidebarCollapsed"
        @close="closeSidebar"
        @toggle-collapse="toggleSidebarCollapse"
      />
    </div>
    <div class="flex flex-1 flex-col overflow-hidden">
      <div class="no-print">
        <TopBar
          @toggle-sidebar="toggleSidebar"
          @open-command-palette="openCommandPalette"
          @open-shortcuts="openShortcuts"
          @open-notify-history="openNotifyHistory"
        />
      </div>
      <BreadcrumbBar v-if="showBreadcrumb" />
      <div
        v-if="offline"
        class="no-print flex flex-wrap items-center justify-between gap-2 border-b border-amber-300 bg-amber-50 px-3 py-2 text-xs text-amber-950 dark:border-amber-800 dark:bg-amber-950/50 dark:text-amber-100 sm:px-6"
        role="status"
        aria-live="polite"
        data-testid="offline-banner"
      >
        <div class="min-w-0">
          <span class="font-semibold">{{ t('offlineBannerTitle') }}</span>
          <span class="ml-1">{{ t('offlineBanner') }}</span>
        </div>
        <button
          type="button"
          class="mts-btn mts-focus-ring shrink-0 !border-amber-300 !bg-white !text-amber-950 dark:!border-amber-800 dark:!bg-amber-950 dark:!text-amber-100"
          data-testid="offline-banner-retry"
          @click="retryNetworkStatus"
        >{{ t('offlineBannerRetry') }}</button>
      </div>
      <div
        v-else-if="sessionUrgency === 'warn'"
        class="no-print flex flex-wrap items-center justify-between gap-2 border-b border-amber-300 bg-amber-50 px-3 py-2 text-xs text-amber-950 dark:border-amber-800 dark:bg-amber-950/40 dark:text-amber-100 sm:px-6"
        role="status"
        aria-live="polite"
        data-testid="session-warn-banner"
      >
        <div class="min-w-0">
          <span class="font-semibold">{{ t('sessionWarnBannerTitle') }}</span>
          <span
            v-if="sessionRemainingLabel"
            class="ml-1 rounded-full bg-amber-100 px-1.5 py-0.5 font-mono text-[11px] text-amber-900 dark:bg-amber-900/50 dark:text-amber-100"
            data-testid="session-warn-remaining"
          >{{ sessionRemainingLabel }}</span>
          <span class="ml-1">{{ t('sessionWarnBanner') }}</span>
        </div>
        <div class="flex shrink-0 flex-wrap items-center gap-2">
          <button
            type="button"
            class="mts-btn mts-focus-ring !border-amber-300 !bg-white !text-amber-950 dark:!border-amber-800 dark:!bg-amber-950 dark:!text-amber-100"
            data-testid="session-warn-renew"
            @click="goSessionRenew"
          >{{ t('sessionWarnRenew') }}</button>
        </div>
      </div>
      <div
        v-else-if="sessionWriteBlocked"
        class="no-print flex flex-wrap items-center justify-between gap-2 border-b border-red-300 bg-red-50 px-3 py-2 text-xs text-red-950 dark:border-red-900 dark:bg-red-950/40 dark:text-red-100 sm:px-6"
        role="status"
        aria-live="polite"
        data-testid="session-critical-banner"
      >
        <div class="min-w-0">
          <span class="font-semibold">{{ t('sessionCriticalBannerTitle') }}</span>
          <span
            v-if="sessionRemainingLabel"
            class="ml-1 rounded-full bg-red-100 px-1.5 py-0.5 font-mono text-[11px] text-red-900 dark:bg-red-900/50 dark:text-red-100"
            data-testid="session-critical-remaining"
          >{{ sessionRemainingLabel }}</span>
          <span class="ml-1">{{ t('sessionCriticalBanner') }}</span>
        </div>
        <div class="flex shrink-0 flex-wrap items-center gap-2">
          <button
            type="button"
            class="mts-btn mts-focus-ring !border-red-300 !bg-white !text-red-900 dark:!border-red-800 dark:!bg-red-950 dark:!text-red-100"
            data-testid="session-critical-renew"
            @click="goSessionRenew"
          >{{ t('sessionCriticalRenew') }}</button>
          <button
            type="button"
            class="mts-btn mts-focus-ring !border-red-300 !bg-white !text-red-900 dark:!border-red-800 dark:!bg-red-950 dark:!text-red-100"
            data-testid="session-critical-relogin"
            @click="goSessionRelogin"
          >{{ t('sessionCriticalRelogin') }}</button>
        </div>
      </div>
      <div
        v-if="isAdmin && adminOpBusy && !offline"
        class="no-print flex flex-wrap items-center justify-between gap-2 border-b border-sky-300 bg-sky-50 px-3 py-2 text-xs text-sky-950 dark:border-sky-900 dark:bg-sky-950/40 dark:text-sky-100 sm:px-6"
        role="status"
        aria-live="polite"
        data-testid="admin-op-busy-banner"
      >
        <div class="min-w-0">
          <span class="font-semibold">{{ t('adminOpBusyBannerTitle') }}</span>
          <span class="ml-1">{{ t('adminOpBusyBanner') }}</span>
          <span v-if="adminOpBusyDetail" class="ml-1 font-medium" data-testid="admin-op-busy-detail">{{ adminOpBusyDetail }}</span>
          <span
            v-if="adminOpBusyPollErrorLabel"
            class="ml-1 text-amber-800 dark:text-amber-200"
            data-testid="admin-op-busy-poll-error"
          >{{ adminOpBusyPollErrorLabel }}</span>
        </div>
        <div class="flex shrink-0 flex-wrap items-center gap-2">
          <button
            type="button"
            class="mts-btn mts-focus-ring !border-sky-300 !bg-white !text-sky-900 dark:!border-sky-800 dark:!bg-sky-950 dark:!text-sky-100"
            data-testid="admin-op-busy-refresh"
            @click="refreshAdminOpBusy"
          >{{ t('adminOpBusyRefresh') }}</button>
          <button
            type="button"
            class="mts-btn mts-focus-ring !border-sky-300 !bg-white !text-sky-900 dark:!border-sky-800 dark:!bg-sky-950 dark:!text-sky-100"
            data-testid="admin-op-busy-open-ops"
            @click="goAdminOpBusyOps"
          >{{ t('adminOpBusyOpenOps') }}</button>
        </div>
      </div>
      <div
        v-if="isAdmin && adminOpBusyPollErrorLabel && !adminOpBusy && !offline"
        class="no-print flex flex-wrap items-center justify-between gap-2 border-b border-amber-300 bg-amber-50 px-3 py-2 text-xs text-amber-950 dark:border-amber-900 dark:bg-amber-950/40 dark:text-amber-100 sm:px-6"
        role="status"
        aria-live="polite"
        data-testid="admin-op-busy-poll-error-banner"
      >
        <div class="min-w-0">
          <span class="font-semibold">{{ t('adminOpBusyBannerTitle') }}</span>
          <span class="ml-1" data-testid="admin-op-busy-poll-error">{{ adminOpBusyPollErrorLabel }}</span>
        </div>
        <div class="flex shrink-0 flex-wrap items-center gap-2">
          <button
            type="button"
            class="mts-btn mts-focus-ring !border-amber-300 !bg-white !text-amber-900 dark:!border-amber-800 dark:!bg-amber-950 dark:!text-amber-100"
            data-testid="admin-op-busy-refresh"
            @click="refreshAdminOpBusy"
          >{{ t('adminOpBusyRefresh') }}</button>
        </div>
      </div>
      <div
        v-if="!offline && showUnreachableBanner"
        class="no-print flex flex-wrap items-center justify-between gap-2 border-b border-red-300 bg-red-50 px-3 py-2 text-xs text-red-950 dark:border-red-900 dark:bg-red-950/40 dark:text-red-100 sm:px-6"
        role="alert"
        aria-live="assertive"
        data-testid="server-unreachable-banner"
      >
        <div class="min-w-0">
          <span class="font-semibold">{{ t('serverUnreachableTitle') }}</span>
          <span class="ml-1">{{ t('serverUnreachableBody') }}</span>
        </div>
        <button
          type="button"
          class="mts-btn mts-focus-ring shrink-0"
          data-testid="server-unreachable-retry"
          :disabled="reachChecking"
          @click="retryReadyz"
        >
          {{ t('serverUnreachableRetry') }}
        </button>
      </div>
      <div
        v-if="recent.length"
        class="no-print flex flex-wrap items-center gap-2 border-b border-slate-200 bg-white px-3 py-1.5 dark:border-slate-700 dark:bg-slate-900 sm:px-6"
        data-testid="recent-routes"
      >
        <span class="text-[11px] mts-muted">{{ t('recentRoutes') }}</span>
        <span
          v-for="r in recent.slice(0, 6)"
          :key="r.path + r.at"
          class="inline-flex items-center gap-0.5 rounded-full border px-1 py-0.5 text-[11px]"
          :class="r.pinned
            ? 'border-sky-300 bg-sky-50 text-sky-800 dark:border-sky-800 dark:bg-sky-950/40 dark:text-sky-100'
            : 'border-slate-200 text-slate-600 dark:border-slate-700 dark:text-slate-300'"
          :data-testid="`recent-route-chip-${r.path}`"
        >
          <button
            type="button"
            class="rounded-full px-1.5 py-0.5 hover:bg-slate-100 dark:hover:bg-slate-800"
            :data-testid="`recent-route-${r.path}`"
            @click="goRecent(r.path)"
          >
            {{ recentLabel(r) }}
          </button>
          <button
            type="button"
            class="mts-focus-ring rounded-full px-1 text-[10px] opacity-70 hover:opacity-100"
            :data-testid="`recent-route-pin-${r.path}`"
            :aria-label="r.pinned ? t('recentRoutesUnpin') : t('recentRoutesPin')"
            :title="r.pinned ? t('recentRoutesUnpin') : t('recentRoutesPin')"
            :aria-pressed="r.pinned ? 'true' : 'false'"
            @click="togglePinRecent(r.path, $event)"
          >
            {{ r.pinned ? '★' : '☆' }}
          </button>
        </span>
        <button
          v-if="recent.filter((x) => !x.pinned).length > 0 && recent.length > 1"
          type="button"
          class="ml-auto rounded border border-slate-200 px-2 py-0.5 text-[11px] text-slate-500 hover:bg-slate-50 dark:border-slate-700 dark:text-slate-400 dark:hover:bg-slate-800"
          data-testid="recent-routes-clear"
          :aria-label="t('recentRoutesClear')"
          :title="t('recentRoutesClear')"
          @click="clearRecent"
        >
          {{ t('recentRoutesClear') }}
        </button>
      </div>
      <main ref="mainContentRef" id="main-content" class="mts-focus-ring flex-1 overflow-auto p-4 sm:p-6 outline-none" tabindex="-1" data-testid="main-content" @scroll.passive="onMainScroll">
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
    <button
      v-if="showBackToTop"
      type="button"
      class="no-print mts-focus-ring fixed bottom-5 right-5 z-[90] inline-flex h-10 w-10 items-center justify-center rounded-full border border-slate-200 bg-white text-slate-700 shadow-lg hover:bg-slate-50 dark:border-slate-700 dark:bg-slate-900 dark:text-slate-100 dark:hover:bg-slate-800"
      data-testid="back-to-top"
      :aria-label="t('backToTop')"
      :title="t('backToTop')"
      @click="() => scrollMainToTop('smooth')"
    >
      <ArrowUp class="h-4 w-4" aria-hidden="true" />
    </button>
    <CommandPalette ref="commandPaletteRef" />
    <ShortcutsHelp v-model:open="shortcutsOpen" />
    <NotifyHistoryPanel v-model:open="notifyHistoryOpen" :prefill="notifyHistoryPrefill" />
  </div>
</template>
