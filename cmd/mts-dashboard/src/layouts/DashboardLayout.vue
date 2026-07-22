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
  DOWNSAMPLE_ERROR_JUMP_PATH,
  isEditableTarget,
  matchNotifyHistoryOpen,
  matchSequenceChordD,
  matchSequenceChordStartG,
  matchShortcutHelpOpen,
  matchSidebarFilterFocus,
} from '@/utils/keyboardShortcuts'
import {
  clearRecentRoutes,
  loadRecentRoutes,
  recordRecentRoute,
  setRecentRoutePinned,
  RECENT_ROUTES_MAX,
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
import {
  adminOpKindLabelKey,
  formatAdminHeavyLastSummary,
  formatAdminHeavyLastDetail,
  formatAdminHeavyLastCopyText,
  formatAdminOpElapsed,
  readDismissedAdminOpLastFinishedAt,
  shouldShowAdminOpLastBanner,
  writeDismissedAdminOpLastFinishedAt,
  writeFailAckedAdminOpLastFinishedAt,
  readFailAckedAdminOpLastFinishedAt,
  canDismissAdminOpLast,
  adminOpLastBannerSurfaceClass,
} from '@/utils/adminOpBusy'
import { formatMessage } from '@/utils/formatMessage'
import { buildLoginLocation } from '@/utils/redirect'
import { useServerReachability } from '@/composables/useServerReachability'
import { shouldSyncOnVisibility } from '@/utils/pageVisibilitySync'
import { nextSessionProbe } from '@/utils/sessionProbe'
import { clockSkewView, shouldShowClockSkewBanner } from '@/utils/clockSkew'
import { registerOpenNotifyHistory } from '@/utils/notifyHistoryBridge'
import { copyText } from '@/utils/clipboard'
import { useNotify } from '@/composables/useNotify'
import { apiGet } from '@/api/client'
import {
  normalizeDownsampleStatusSummary,
  downsampleStatusHealthJump,
  downsampleStatusSummaryTone,
  type DownsampleStatusSummaryInput,
} from '@/utils/downsampleStatusSummary'
import {
  downsampleHealthFingerprint,
  formatDownsampleHealthBannerCopyText,
  readDismissedDownsampleHealthFingerprint,
  shouldShowDownsampleHealthBanner as shouldShowDownsampleHealthBannerState,
  writeDismissedDownsampleHealthFingerprint,
} from '@/utils/downsampleHealthBanner'

const { t } = useI18n()
const { offline, sessionWriteBlocked, sessionRemainingLabel, sessionUrgency } = useMutationGuard()
const { sync: syncNetworkStatus } = useNetworkStatus()
const { logout, isAdmin, refreshSession, isAuthenticated, lastSessionServerTimeUnix, lastSessionCheckedAt } = useAuth()
const layoutClockSkew = computed(() =>
  clockSkewView(lastSessionServerTimeUnix.value, lastSessionCheckedAt.value),
)
const showClockSkewBanner = computed(
  () => isAuthenticated.value && shouldShowClockSkewBanner(layoutClockSkew.value),
)
const clockSkewBannerText = computed(() =>
  formatMessage(t.value('clockSkewBanner'), { skew: layoutClockSkew.value.label || '—' }),
)

function goClockSkewSession() {
  void router.push({ path: '/account', hash: '#account-session' })
}

const { success, error: notifyError } = useNotify()

async function refreshSessionForSkew() {
  try {
    const ok = await refreshSession()
    if (ok) success(t.value('accountSessionVerifyOk'))
    else notifyError(t.value('accountSessionVerifyFailed'))
  } catch {
    notifyError(t.value('accountSessionVerifyFailed'))
  }
}

const {
  adminOpBusy,
  adminOpKind,
  adminOpStartedAtUnix,
  adminOpLast,
  adminOpBusyError,
  adminOpBusyFailStreak,
  adminOpPollIntervalMs,
  refreshAdminOpBusy,
} = useAdminOpBusy()
const dismissedAdminOpLastFinishedAt = ref<number | null>(
  typeof localStorage !== 'undefined' ? readDismissedAdminOpLastFinishedAt(localStorage) : null,
)
const failAckedAdminOpLastFinishedAt = ref<number | null>(
  typeof localStorage !== 'undefined' ? readFailAckedAdminOpLastFinishedAt(localStorage) : null,
)

/** 降采样健康（admin 全局告警；summary_only 轻量） */
const downsampleHealthSummary = ref<ReturnType<typeof normalizeDownsampleStatusSummary> | null>(null)
const dismissedDownsampleHealthFp = ref<string | null>(
  typeof sessionStorage !== 'undefined' ? readDismissedDownsampleHealthFingerprint(sessionStorage) : null,
)
const downsampleHealthTone = computed(() => {
  if (!downsampleHealthSummary.value) return 'ok' as const
  return downsampleStatusSummaryTone(downsampleHealthSummary.value)
})
const showDownsampleHealthBanner = computed(() =>
  shouldShowDownsampleHealthBannerState({
    isAdmin: isAdmin.value,
    offline: offline.value,
    summary: downsampleHealthSummary.value,
    dismissedFingerprint: dismissedDownsampleHealthFp.value,
  }),
)
function dismissDownsampleHealthBanner() {
  const s = downsampleHealthSummary.value
  if (!s) return
  const fp = downsampleHealthFingerprint(s)
  if (typeof sessionStorage !== 'undefined') {
    writeDismissedDownsampleHealthFingerprint(sessionStorage, fp)
  }
  dismissedDownsampleHealthFp.value = fp
}
async function copyDownsampleHealthBanner() {
  const textToCopy = formatDownsampleHealthBannerCopyText(downsampleHealthSummary.value)
  if (!textToCopy.trim()) {
    notifyError(t.value('failed'))
    return
  }
  const res = await copyText(textToCopy)
  if (res.ok) success(t.value('downsampleHealthBannerCopied'))
  else notifyError(res.error || t.value('failed'))
}
const downsampleHealthBannerClass = computed(() => {
  if (downsampleHealthTone.value === 'bad') {
    return 'border-red-300 bg-red-50 text-red-950 dark:border-red-900 dark:bg-red-950/40 dark:text-red-100'
  }
  return 'border-amber-300 bg-amber-50 text-amber-950 dark:border-amber-900 dark:bg-amber-950/40 dark:text-amber-100'
})
const downsampleHealthErrorJump = computed(() => downsampleStatusHealthJump('error'))
const downsampleHealthLaggingJump = computed(() => downsampleStatusHealthJump('lagging'))
let downsampleHealthTimer: ReturnType<typeof setInterval> | null = null
let downsampleHealthInflight = false
async function refreshDownsampleHealth() {
  if (!isAdmin.value || offline.value || downsampleHealthInflight) return
  downsampleHealthInflight = true
  try {
    const st = await apiGet<{ summary?: DownsampleStatusSummaryInput }>(
      '/api/v1/admin/downsample/statuses?summary_only=1',
    )
    downsampleHealthSummary.value = normalizeDownsampleStatusSummary(st.summary)
  } catch {
    // 全局告警失败静默；保留上次摘要
  } finally {
    downsampleHealthInflight = false
  }
}
function armDownsampleHealthPoll() {
  if (downsampleHealthTimer) return
  void refreshDownsampleHealth()
  downsampleHealthTimer = setInterval(() => {
    void refreshDownsampleHealth()
  }, 60_000)
}
function disarmDownsampleHealthPoll() {
  if (downsampleHealthTimer) clearInterval(downsampleHealthTimer)
  downsampleHealthTimer = null
}

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
watch(
  isAdmin,
  (ok) => {
    if (ok) armDownsampleHealthPoll()
    else {
      disarmDownsampleHealthPoll()
      downsampleHealthSummary.value = null
    }
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
  const base = formatMessage(t.value('adminOpBusyPollError'), { error: err })
  const streak = adminOpBusyFailStreak.value || 0
  if (streak <= 0) return base
  const sec = Math.max(1, Math.round((adminOpPollIntervalMs.value || 0) / 1000))
  return `${base} · ${formatMessage(t.value('adminOpBusyPollBackoff'), { n: String(streak), sec: String(sec) })}`
})

const adminOpLastSummary = computed(() => {
  const last = adminOpLast.value
  if (!last || !last.op) return ''
  const key = adminOpKindLabelKey(last.op) as import('@/i18n/messages').MessageKey
  const kind = t.value(key) || last.op
  return formatAdminHeavyLastSummary(last, kind)
})

const adminOpLastDetail = computed(() => formatAdminHeavyLastDetail(adminOpLast.value))

const canDismissLastBanner = computed(() =>
  canDismissAdminOpLast({
    lastOk: adminOpLast.value ? adminOpLast.value.ok : null,
    lastFinishedAtUnix: adminOpLast.value?.finishedAtUnix ?? null,
    failAckedFinishedAtUnix: failAckedAdminOpLastFinishedAt.value,
  }),
)

const showAdminOpLastBanner = computed(() =>
  shouldShowAdminOpLastBanner({
    isAdmin: isAdmin.value,
    busy: adminOpBusy.value,
    offline: offline.value,
    pollError: adminOpBusyError.value,
    lastSummary: adminOpLastSummary.value,
    lastOk: adminOpLast.value ? adminOpLast.value.ok : null,
    lastFinishedAtUnix: adminOpLast.value?.finishedAtUnix ?? null,
    dismissedFinishedAtUnix: dismissedAdminOpLastFinishedAt.value,
    failAckedFinishedAtUnix: failAckedAdminOpLastFinishedAt.value,
  }),
)

function ackAdminOpLastFailIfNeeded() {
  const last = adminOpLast.value
  if (!last || last.ok !== false) return
  const finished = last.finishedAtUnix ?? null
  if (typeof localStorage !== 'undefined') {
    writeFailAckedAdminOpLastFinishedAt(localStorage, finished)
  }
  failAckedAdminOpLastFinishedAt.value =
    finished != null && finished > 0 ? Math.floor(finished) : null
}

async function copyAdminOpLastBanner() {
  const last = adminOpLast.value
  if (!last || !last.op) {
    notifyError(t.value('opsStatusLastEmpty'))
    return
  }
  const key = adminOpKindLabelKey(last.op) as MessageKey
  const kind = t.value(key) || last.op
  const textToCopy = formatAdminHeavyLastCopyText(last, kind)
  if (!textToCopy) {
    notifyError(t.value('opsStatusLastEmpty'))
    return
  }
  const res = await copyText(textToCopy)
  if (res.ok) success(t.value('opsStatusLastCopied'))
  else notifyError(res.error || t.value('failed'))
}

function dismissAdminOpLastBanner() {
  const last = adminOpLast.value
  const finished = last?.finishedAtUnix ?? null
  if (
    !canDismissAdminOpLast({
      lastOk: last ? last.ok : null,
      lastFinishedAtUnix: finished,
      failAckedFinishedAtUnix: failAckedAdminOpLastFinishedAt.value,
    })
  ) {
    return false
  }
  if (typeof localStorage !== 'undefined') {
    writeDismissedAdminOpLastFinishedAt(localStorage, finished)
  }
  dismissedAdminOpLastFinishedAt.value = finished != null && finished > 0 ? Math.floor(finished) : null
  return true
}

provide('adminOpBusySummary', computed(() => ({
  busy: adminOpBusy.value,
  op: adminOpKind.value,
  opLabel: adminOpKindLabel.value,
  elapsed: adminOpElapsedLabel.value,
  detail: adminOpBusyDetail.value,
  pollError: adminOpBusyPollErrorLabel.value,
  lastSummary: adminOpLastSummary.value,
  lastError: adminOpLastDetail.value,
  lastOk: adminOpLast.value ? adminOpLast.value.ok : null,
  lastFinishedAtUnix: adminOpLast.value?.finishedAtUnix ?? null,
})))
provide('dismissAdminOpLastBanner', dismissAdminOpLastBanner)
provide('ackAdminOpLastFail', ackAdminOpLastFailIfNeeded)
provide(
  'showAdminOpLastBanner',
  computed(() => showAdminOpLastBanner.value),
)
provide('canDismissAdminOpLastBanner', canDismissLastBanner)
const { showUnreachableBanner, checkOnce: retryReadyz, checking: reachChecking } = useServerReachability()

function retryNetworkStatus() {
  syncNetworkStatus()
  void retryReadyz()
}

function onVisibilitySync() {
  if (!shouldSyncOnVisibility(document.visibilityState, document.hidden)) return
  syncNetworkStatus()
  void retryReadyz()
  if (isAdmin.value) {
    void refreshAdminOpBusy()
    void refreshDownsampleHealth()
  }
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
  ackAdminOpLastFailIfNeeded()
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


let pendingGoChordUntil = 0
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
    return
  }
  const editable = isEditableTarget(e.target)
  const now = Date.now()
  if (pendingGoChordUntil > now && matchSequenceChordD(e, editable)) {
    e.preventDefault()
    pendingGoChordUntil = 0
    if (isAdmin.value) void router.push(DOWNSAMPLE_ERROR_JUMP_PATH)
    return
  }
  if (matchSequenceChordStartG(e, editable)) {
    // 不 preventDefault，避免干扰输入法；仅记录等待窗
    pendingGoChordUntil = now + 800
    return
  }
  pendingGoChordUntil = 0
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

let sessionProbeTimer: ReturnType<typeof setTimeout> | null = null
let lastSessionProbeAt = 0

function clearSessionProbeTimer() {
  if (sessionProbeTimer) {
    clearTimeout(sessionProbeTimer)
    sessionProbeTimer = null
  }
}

function armSessionProbe() {
  clearSessionProbeTimer()
  if (!isAuthenticated.value) return
  const age = lastSessionProbeAt > 0 ? Date.now() - lastSessionProbeAt : Number.POSITIVE_INFINITY
  const decision = nextSessionProbe(sessionUrgency.value, age)
  const delay = decision.shouldProbe ? 1_000 : decision.nextDelayMs
  sessionProbeTimer = setTimeout(() => {
    void (async () => {
      if (!isAuthenticated.value) return
      const ageNow = lastSessionProbeAt > 0 ? Date.now() - lastSessionProbeAt : Number.POSITIVE_INFINITY
      const d = nextSessionProbe(sessionUrgency.value, ageNow)
      if (d.shouldProbe) {
        lastSessionProbeAt = Date.now()
        try {
          await refreshSession()
        } catch {
          /* 静默：可见性/鉴权失败由全局处理 */
        }
      }
      armSessionProbe()
    })()
  }, delay)
}

watch(sessionUrgency, () => {
  armSessionProbe()
})

onMounted(() => {
  document.addEventListener('visibilitychange', onVisibilitySync)
  armSessionProbe()
  window.addEventListener('keydown', onGlobalKey)
  window.addEventListener(CLIENT_PREFS_CHANGED_EVENT, onPrefsChanged)
  unregisterNotifyHistoryBridge = registerOpenNotifyHistory(openNotifyHistory)
})
onBeforeUnmount(() => {
  disarmAdminOpTick()
  disarmDownsampleHealthPoll()
  document.removeEventListener('visibilitychange', onVisibilitySync)
  clearSessionProbeTimer()
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
        v-if="showClockSkewBanner && !offline"
        class="no-print flex flex-wrap items-center justify-between gap-2 border-b border-amber-300 bg-amber-50 px-3 py-2 text-xs text-amber-950 dark:border-amber-900 dark:bg-amber-950/40 dark:text-amber-100 sm:px-6"
        role="status"
        aria-live="polite"
        data-testid="clock-skew-banner"
      >
        <div class="min-w-0 flex-1">
          <div class="font-medium" data-testid="clock-skew-banner-title">{{ t('clockSkewBannerTitle') }}</div>
          <div class="mt-0.5 opacity-90" data-testid="clock-skew-banner-detail">{{ clockSkewBannerText }}</div>
        </div>
        <div class="flex flex-wrap gap-2">
          <button
            type="button"
            class="mts-btn mts-focus-ring !border-amber-300 !bg-white !text-amber-950 dark:!border-amber-800 dark:!bg-amber-950 dark:!text-amber-100"
            data-testid="clock-skew-banner-refresh"
            @click="refreshSessionForSkew"
          >{{ t('clockSkewBannerRefresh') }}</button>
          <button
            type="button"
            class="mts-btn mts-focus-ring !border-amber-300 !bg-white !text-amber-950 dark:!border-amber-800 dark:!bg-amber-950 dark:!text-amber-100"
            data-testid="clock-skew-banner-open-session"
            @click="goClockSkewSession"
          >{{ t('clockSkewBannerOpenSession') }}</button>
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
        v-if="showAdminOpLastBanner"
        :class="adminOpLastBannerSurfaceClass(adminOpLast?.ok)"
        role="status"
        aria-live="polite"
        data-testid="admin-op-last-banner"
        :data-ok="adminOpLast?.ok === true ? 'true' : (adminOpLast?.ok === false ? 'false' : undefined)"
      >
        <div class="min-w-0">
          <span class="font-semibold">{{ t('adminOpLastBannerTitle') }}</span>
          <span class="ml-1" data-testid="admin-op-last-summary">{{ adminOpLastSummary }}</span>
          <p
            v-if="adminOpLastDetail"
            class="mt-1 break-all font-mono text-[11px] opacity-90"
            data-testid="admin-op-last-error"
          >{{ adminOpLastDetail }}</p>
        </div>
        <div class="flex shrink-0 flex-wrap items-center gap-2">
          <span
            v-if="adminOpLast?.ok === false && !canDismissLastBanner"
            class="text-[11px] opacity-90"
            data-testid="admin-op-last-fail-ack-hint"
          >{{ t('adminOpLastFailAckHint') }}</span>
          <button
            type="button"
            class="mts-btn mts-focus-ring"
            data-testid="admin-op-last-copy"
            :title="t('opsStatusLastCopy')"
            @click="copyAdminOpLastBanner"
          >{{ t('opsStatusLastCopy') }}</button>
          <button
            type="button"
            class="mts-btn mts-focus-ring"
            data-testid="admin-op-last-open-ops"
            @click="goAdminOpBusyOps"
          >{{ t('adminOpLastOpenOps') }}</button>
          <button
            type="button"
            class="mts-btn mts-focus-ring"
            data-testid="admin-op-last-dismiss"
            :disabled="!canDismissLastBanner"
            :title="t('adminOpLastDismiss')"
            @click="dismissAdminOpLastBanner"
          >{{ t('adminOpLastDismiss') }}</button>
        </div>
      </div>
      <div
        v-if="showDownsampleHealthBanner"
        class="no-print flex flex-wrap items-center justify-between gap-2 border-b px-3 py-2 text-xs sm:px-6"
        :class="downsampleHealthBannerClass"
        role="status"
        aria-live="polite"
        data-testid="downsample-health-banner"
      >
        <div class="min-w-0">
          <span class="font-semibold">{{ t('downsampleHealthBannerTitle') }}</span>
          <span class="ml-1" data-testid="downsample-health-banner-detail">
            {{ formatMessage(t('downsampleHealthBannerDetail'), {
              error: String(downsampleHealthSummary?.error ?? 0),
              lagging: String(downsampleHealthSummary?.lagging ?? 0),
              maxLag: String(downsampleHealthSummary?.max_lag_seconds ?? 0),
            }) }}
          </span>
        </div>
        <div class="flex shrink-0 flex-wrap items-center gap-2">
          <router-link
            v-if="(downsampleHealthSummary?.error ?? 0) > 0"
            class="mts-btn mts-focus-ring text-xs"
            :to="downsampleHealthErrorJump"
            data-testid="downsample-health-banner-error"
          >{{ t('overviewDownsampleJumpError') }}</router-link>
          <router-link
            v-if="(downsampleHealthSummary?.lagging ?? 0) > 0"
            class="mts-btn mts-focus-ring text-xs"
            :to="downsampleHealthLaggingJump"
            data-testid="downsample-health-banner-lagging"
          >{{ t('overviewDownsampleJumpLagging') }}</router-link>
          <button
            type="button"
            class="mts-btn mts-focus-ring text-xs"
            data-testid="downsample-health-banner-refresh"
            @click="refreshDownsampleHealth"
          >{{ t('refresh') }}</button>
          <button
            type="button"
            class="mts-btn mts-focus-ring text-xs"
            data-testid="downsample-health-banner-copy"
            :title="t('downsampleHealthBannerCopy')"
            @click="copyDownsampleHealthBanner"
          >{{ t('downsampleHealthBannerCopy') }}</button>
          <button
            type="button"
            class="mts-btn mts-focus-ring text-xs"
            data-testid="downsample-health-banner-dismiss"
            :title="t('downsampleHealthBannerDismiss')"
            @click="dismissDownsampleHealthBanner"
          >{{ t('downsampleHealthBannerDismiss') }}</button>
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
          v-for="r in recent.slice(0, RECENT_ROUTES_MAX)"
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
