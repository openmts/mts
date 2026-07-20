<script setup lang="ts">
import { useRoute, useRouter } from 'vue-router'
import { useAuth } from '@/composables/useAuth'
import { useTheme } from '@/composables/useTheme'
import { useI18n } from '@/composables/useI18n'
import { useNotify } from '@/composables/useNotify'
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { Menu, Moon, Sun, Languages, Search, Keyboard } from 'lucide-vue-next'
import { parseExpiresAt, sessionExpiryView } from '@/utils/sessionExpiry'
import { useServerReachability } from '@/composables/useServerReachability'
import {
  emptySessionGuardState,
  nextSessionGuardAction,
  shouldShowSessionBadge,
  type SessionGuardState,
} from '@/utils/sessionGuard'
import { resolveRouteTitleKey } from '@/utils/pageTitle'
import type { MessageKey } from '@/i18n/messages'

const route = useRoute()
const router = useRouter()
const { currentUser, currentRole, logout, loggingOut, getTokenExpiresAt, ensureSession } = useAuth()
const { theme, toggleTheme } = useTheme()
const { t, locale, toggleLocale } = useI18n()
const { kind: connectivityKind } = useServerReachability()
const { info, warn, error: notifyError } = useNotify()
const nowMs = ref(Date.now())
let timer: ReturnType<typeof setInterval> | null = null
const guardState = ref<SessionGuardState>(emptySessionGuardState())
let expireInFlight = false

const emit = defineEmits<{ 'toggle-sidebar': []; 'open-command-palette': []; 'open-shortcuts': [] }>()

const pageTitle = computed(() => {
  const key = resolveRouteTitleKey(route.name)
  if (!key) return route.name ? String(route.name) : ''
  return t.value(key as MessageKey)
})

const sessionView = computed(() => {
  const exp = parseExpiresAt(getTokenExpiresAt())
  return sessionExpiryView(exp, nowMs.value, undefined, undefined, locale.value === 'en' ? 'en' : 'zh')
})

function roleLabel(role?: string | null): string {
  if (role === 'admin') return t.value('roleAdmin')
  if (role === 'user') return t.value('roleUser')
  return role || ''
}


const connectivityBadgeClass = computed(() => {
  switch (connectivityKind.value) {
    case 'ok':
      return 'bg-emerald-50 text-emerald-800 dark:bg-emerald-950/40 dark:text-emerald-200'
    case 'unreachable':
      return 'bg-red-100 text-red-800 dark:bg-red-950/50 dark:text-red-200'
    case 'offline':
      return 'bg-amber-100 text-amber-900 dark:bg-amber-950/50 dark:text-amber-100'
    default:
      return 'bg-slate-100 text-slate-600 dark:bg-slate-800 dark:text-slate-300'
  }
})
const connectivityBadgeLabel = computed(() => {
  switch (connectivityKind.value) {
    case 'ok':
      return t.value('connectivityOk')
    case 'unreachable':
      return t.value('connectivityUnreachable')
    case 'offline':
      return t.value('connectivityOffline')
    default:
      return t.value('connectivityUnknown')
  }
})

const showSessionBadge = computed(() => shouldShowSessionBadge(sessionView.value.urgency))

const sessionBadgeClass = computed(() => {
  switch (sessionView.value.urgency) {
    case 'critical':
      return 'bg-red-100 text-red-700 dark:bg-red-950/50 dark:text-red-200'
    case 'warn':
      return 'bg-amber-100 text-amber-800 dark:bg-amber-950/50 dark:text-amber-200'
    case 'expired':
      return 'bg-red-200 text-red-900 dark:bg-red-900 dark:text-red-100'
    case 'ok':
      return 'bg-emerald-50 text-emerald-800 dark:bg-emerald-950/40 dark:text-emerald-200'
    default:
      return 'bg-slate-100 text-slate-600 dark:bg-slate-800 dark:text-slate-300'
  }
})

async function handleExpire() {
  if (expireInFlight) return
  expireInFlight = true
  try {
    notifyError(t.value('sessionExpired'))
    await logout()
    if (router.currentRoute.value.name !== 'Login') {
      await router.replace({ name: 'Login', query: { reason: 'session' } })
    }
  } finally {
    expireInFlight = false
  }
}

function tickSession() {
  nowMs.value = Date.now()
  const exp = parseExpiresAt(getTokenExpiresAt())
  // 无 token：若守卫尚未处理过期则触发一次登出
  if (!getTokenExpiresAt() && !ensureSession()) {
    const r = nextSessionGuardAction(nowMs.value - 1, nowMs.value, guardState.value)
    guardState.value = r.state
    if (r.action.type === 'expire') void handleExpire()
    return
  }
  const r = nextSessionGuardAction(exp, nowMs.value, guardState.value)
  guardState.value = r.state
  if (r.action.type === 'toast') {
    const msg =
      r.action.urgency === 'critical'
        ? `${t.value('sessionCriticalToast')} (${r.action.remainingLabel})`
        : `${t.value('sessionWarnToast')} (${r.action.remainingLabel})`
    if (r.action.urgency === 'critical') warn(msg)
    else info(msg)
  } else if (r.action.type === 'expire') {
    void handleExpire()
  }
}

async function onLogout() {
  await logout()
  await router.replace({ name: 'Login' })
}

onMounted(() => {
  tickSession()
  timer = setInterval(() => { tickSession() }, 15_000)
})
onBeforeUnmount(() => {
  if (timer) clearInterval(timer)
})
</script>

<template>
  <header class="flex h-14 items-center justify-between gap-2 border-b border-slate-200 bg-white px-3 dark:border-slate-700 dark:bg-slate-900 sm:px-6">
    <div class="flex items-center gap-3">
      <button
        type="button"
        class="mts-focus-ring rounded p-1 text-slate-500 dark:text-slate-400 hover:bg-slate-100 hover:text-slate-700 dark:hover:bg-slate-800 dark:hover:text-slate-200 lg:hidden"
        :aria-label="t('topbarMenu')"
        :title="t('topbarMenu')"
        data-testid="topbar-menu"
        @click="emit('toggle-sidebar')"
      >
        <Menu class="h-5 w-5" aria-hidden="true" />
      </button>
      <h1 class="max-w-[40vw] truncate text-base font-medium text-slate-800 dark:text-slate-100 sm:max-w-none sm:text-lg">{{ pageTitle }}</h1>
    </div>
    <div class="flex items-center gap-2 text-sm text-slate-600 dark:text-slate-300">
      <button
        type="button"
        class="hidden items-center gap-1 rounded-lg border border-slate-200 px-2 py-1 text-[11px] text-slate-500 hover:bg-slate-100 dark:border-slate-700 dark:text-slate-400 dark:hover:bg-slate-800 sm:inline-flex"
        :title="t('commandPaletteTitle')"
        :aria-label="t('commandPaletteTitle')"
        data-testid="topbar-command-palette"
        @click="emit('open-command-palette')"
      >
        <Search class="h-3.5 w-3.5" />
        <span>{{ t('commandPaletteShort') }}</span>
        <kbd class="rounded border border-slate-200 px-1 text-[10px] dark:border-slate-600">⌘K</kbd>
      </button>
      <button
        type="button"
        class="inline-flex items-center gap-1 rounded-lg border border-slate-200 bg-slate-50 px-2 py-1 text-xs text-slate-600 hover:bg-slate-100 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-300 dark:hover:bg-slate-700"
        data-testid="topbar-shortcuts"
        :title="t('shortcutHelpTitle')"
        :aria-label="t('shortcutHelpTitle')"
        @click="emit('open-shortcuts')"
      >
        <Keyboard class="h-3.5 w-3.5" />
        <span class="hidden sm:inline">?</span>
      </button>
      <span
        class="hidden rounded-full px-2 py-0.5 text-[11px] font-medium sm:inline"
        :class="connectivityBadgeClass"
        :title="`${t('topbarConnectivity')}: ${connectivityBadgeLabel}`"
        :aria-label="`${t('topbarConnectivity')}: ${connectivityBadgeLabel}`"
        data-testid="topbar-connectivity"
        role="status"
        aria-live="polite"
      >{{ connectivityBadgeLabel }}</span>
      <span
        v-if="showSessionBadge && sessionView.label"
        class="hidden rounded-full px-2 py-0.5 text-[11px] font-medium sm:inline"
        :class="sessionBadgeClass"
        :title="t('sessionExpiry')"
        data-testid="session-badge"
        role="status"
        aria-live="polite"
      >{{ t('sessionLeft') }} {{ sessionView.label }}</span>
      <button type="button" class="mts-focus-ring rounded p-1.5 hover:bg-slate-100 dark:hover:bg-slate-800" :title="t('lang')" :aria-label="t('lang')" data-testid="topbar-lang" @click="toggleLocale">
        <Languages class="h-4 w-4" />
        <span class="sr-only">{{ locale }}</span>
      </button>
      <button
        type="button"
        class="mts-focus-ring rounded p-1.5 hover:bg-slate-100 dark:hover:bg-slate-800"
        :aria-label="t('topbarTheme')"
        :title="t('topbarTheme')"
        data-testid="topbar-theme"
        @click="toggleTheme"
      >
        <Moon v-if="theme === 'light'" class="h-4 w-4" aria-hidden="true" />
        <Sun v-else class="h-4 w-4" aria-hidden="true" />
      </button>
      <button
        type="button"
        class="hidden max-w-[10rem] truncate rounded px-1.5 py-0.5 text-left hover:bg-slate-100 dark:hover:bg-slate-800 sm:inline"
        :title="t('account')"
        :aria-label="t('account')"
        data-testid="topbar-account"
        @click="router.push({ name: 'Account' })"
      >
        {{ currentUser }}<span v-if="currentRole" class="text-slate-400 dark:text-slate-500"> · {{ roleLabel(currentRole) }}</span>
      </button>
      <button
        type="button"
        class="mts-focus-ring rounded px-2 py-1 text-slate-500 hover:bg-slate-100 hover:text-slate-700 disabled:opacity-50 dark:text-slate-400 dark:hover:bg-slate-800 dark:hover:text-slate-200"
        data-testid="topbar-logout"
        :aria-label="loggingOut ? t('loggingOut') : t('logout')"
        :disabled="loggingOut"
        @click="onLogout"
      >
        {{ loggingOut ? t('loggingOut') : t('logout') }}
      </button>
    </div>
  </header>
</template>
