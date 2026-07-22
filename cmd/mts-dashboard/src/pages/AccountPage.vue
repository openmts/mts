<script setup lang="ts">
import { computed, inject, onBeforeUnmount, onMounted, ref, watch, type ComputedRef } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useHashScroll } from '@/composables/useHashScroll'
import { parseAccountPrefill, accountFormToPrefill } from '@/utils/routePrefill'
import { buildLoginLocation } from '@/utils/redirect'
import { useAuth } from '@/composables/useAuth'

import { getTokenExpiresAt } from '@/api/client'
import {
  effectiveSessionRemainingMs,
  formatRemaining,
  parseExpiresAt,
  sessionExpiryView,
  sessionViewFromRemainingMs,
} from '@/utils/sessionExpiry'
import { sessionClockTickMs } from '@/utils/sessionClock'
import { useI18n } from '@/composables/useI18n'
import { useNotify } from '@/composables/useNotify'
import ActionResultBanner from '@/components/ActionResultBanner.vue'
import AdminOpLastChip from '@/components/AdminOpLastChip.vue'
import InFlightBanner from '@/components/InFlightBanner.vue'
import { createActionAbort } from '@/utils/actionAbort'
import { isCanceledError, isTimeoutError } from '@/utils/apiError'
import EmptyState from '@/components/EmptyState.vue'
import {
  getDefaultAuthTTLSeconds,
  getForbiddenDefaultPasswords,
  getMaxAuthTTLSeconds,
  getMinPasswordLength,
  getPasswordPolicyVersion,
  validateNewPassword,
} from '@/utils/passwordPolicy'
import { bootstrapPasswordPolicy } from '@/utils/passwordPolicyBootstrap'
import { formatAuthTTLLimit } from '@/utils/loginTTL'
import { formatMessage } from '@/utils/formatMessage'
import { KeyRound, UserRound, Download, Copy } from 'lucide-vue-next'
import PasswordHints from '@/components/PasswordHints.vue'
import PasswordInputWithToggle from '@/components/PasswordInputWithToggle.vue'
import { buildAccountExport, formatAccountExportPretty } from '@/utils/accountExport'
import { buildClientPrefsExport, formatClientPrefsExportPretty } from '@/utils/clientPrefsExport'
import { stampFilename } from '@/utils/download'
import { useExportJob } from '@/composables/useExportJob'
import ExportJobBanner from '@/components/ExportJobBanner.vue'
import { copyText } from '@/utils/clipboard'
import {
  LANDING_PATH_OPTIONS,
  isAdminOnlyLandingPath,
  loadLandingPath,
  saveLandingPath,
} from '@/utils/landingPrefs'
import {
  buildLandingOptionViews,
  filterLandingOptions,
  groupLandingOptions,
} from '@/utils/landingOptionsView'
import type { MessageKey } from '@/i18n/messages'
import { useDensity } from '@/composables/useDensity'
import type { UiDensity } from '@/utils/densityPrefs'
import { loadSidebarPrefs, saveSidebarPrefs } from '@/utils/sidebarPrefs'
import { useTheme } from '@/composables/useTheme'
import {
  DEFAULT_CLIENT_PREFS,
  CLIENT_PREFS_CHANGED_EVENT,
  normalizeClientPrefs,
  parseClientPrefsImport,
  type ClientPrefs,
} from '@/utils/clientPrefs'
import { loadNavOrderPrefs, saveNavOrderMap } from '@/utils/navOrder'
import { useNetworkStatus } from '@/composables/useNetworkStatus'
import { shouldBlockOfflineMutation } from '@/utils/offlineGuard'
import { registerDirtyChecker } from '@/utils/routeDirty'

const router = useRouter()
const route = useRoute()
useHashScroll()
const { currentUser, currentRole, changePassword, isAdmin, logout, login, refreshSession, lastSessionRemainingSeconds, lastSessionCheckedAt, lastSessionServerTimeUnix } = useAuth()
const adminOpBusySummary = inject<ComputedRef<{ lastSummary?: string; lastError?: string; lastOk?: boolean | null }> | undefined>('adminOpBusySummary', undefined)
const accountAdminLastLabel = computed(() => (adminOpBusySummary?.value?.lastSummary || '').trim())
const accountAdminLastErrorDetail = computed(() => {
  if (adminOpBusySummary?.value?.lastOk !== false) return ''
  return (adminOpBusySummary?.value?.lastError || '').trim()
})
const { t, locale, setLocale } = useI18n()
const nowMs = ref(Date.now())
let sessionClock: ReturnType<typeof setInterval> | null = null
const expiresAt = computed(() => parseExpiresAt(getTokenExpiresAt()))
const sessionView = computed(() => {
  const loc = locale.value === 'en' ? 'en' : 'zh'
  const local = sessionExpiryView(
    expiresAt.value,
    nowMs.value,
    10 * 60_000,
    2 * 60_000,
    loc,
  )
  if (expiresAt.value == null) return local
  if (lastSessionRemainingSeconds.value == null || lastSessionCheckedAt.value == null) return local
  const effective = effectiveSessionRemainingMs(
    local.remainingMs,
    lastSessionRemainingSeconds.value,
    lastSessionCheckedAt.value,
    nowMs.value,
  )
  return sessionViewFromRemainingMs(effective, true, 10 * 60_000, 2 * 60_000, loc)
})
const remainingText = computed(() => {
  if (expiresAt.value == null) return t.value('accountSessionNone')
  if (sessionView.value.urgency === 'expired') return t.value('sessionExpiredLabel')
  return formatRemaining(Math.max(0, sessionView.value.remainingMs))
})

const policyMinLength = computed(() => getMinPasswordLength())
const policyForbidden = computed(() => getForbiddenDefaultPasswords().join(', '))
const policyTTLMax = computed(() => formatAuthTTLLimit(getMaxAuthTTLSeconds()))
const policyTTLDefault = computed(() => formatAuthTTLLimit(getDefaultAuthTTLSeconds()))
const policyVersionLabel = computed(() => {
  const v = getPasswordPolicyVersion()
  if (v > 0) return formatMessage(t.value('passwordPolicyServerSynced'), { version: v })
  return t.value('passwordPolicyLocalDefault')
})
const sessionLevelLabel = computed(() => {
  switch (sessionView.value.urgency) {
    case 'ok': return t.value('sessionLevelOk')
    case 'warn': return t.value('sessionLevelWarn')
    case 'critical': return t.value('sessionLevelCritical')
    case 'expired': return t.value('sessionExpiredLabel')
    default: return t.value('sessionUnknown')
  }
})
const expiresAtText = computed(() => {
  if (expiresAt.value == null) return t.value('emptyValue')
  try {
    return new Date(expiresAt.value).toLocaleString()
  } catch {
    return String(expiresAt.value)
  }
})

async function reLoginPreserve() {
  await logout()
  await router.replace(buildLoginLocation({ reason: 'session', redirectRaw: '/account#account-session' }))
}

function roleLabel(role?: string | null): string {
  if (!role) return t.value('emptyValue')
  if (role === 'admin') return t.value('roleAdmin')
  if (role === 'user') return t.value('roleUser')
  return role
}
const { success, info: notifyInfo, error: notifyError } = useNotify()
const {
  exportJob,
  exportBusy,
  cancelExport,
  resetExport,
  retryLastExport,
  canRetryExport,
  runJSONExport,
} = useExportJob()
const renewPassword = ref('')
const renewTtlSeconds = ref(12 * 3600)
const renewTtlOptions = computed(() => {
  const max = getMaxAuthTTLSeconds()
  const all = [
    { value: 12 * 3600, labelKey: 'accountSessionRenewTtl12h' as const },
    { value: 24 * 3600, labelKey: 'accountSessionRenewTtl24h' as const },
    { value: 7 * 24 * 3600, labelKey: 'accountSessionRenewTtl7d' as const },
    { value: 30 * 24 * 3600, labelKey: 'accountSessionRenewTtl30d' as const },
  ]
  return all.filter((o) => o.value <= max)
})
const renewLoading = ref(false)
const renewError = ref('')
const renewRetryable = ref(false)
const sessionVerifyLoading = ref(false)
const sessionVerifyNote = ref('')


async function verifyServerSession() {
  sessionVerifyNote.value = ''
  sessionVerifyLoading.value = true
  try {
    const ok = await refreshSession()
    nowMs.value = Date.now()
    sessionVerifyNote.value = ok ? t.value('accountSessionVerifyOk') : t.value('accountSessionVerifyFailed')
    if (ok) success(sessionVerifyNote.value)
    else notifyError(sessionVerifyNote.value)
  } finally {
    sessionVerifyLoading.value = false
  }
}

const serverRemainingText = computed(() => {
  const sec = lastSessionRemainingSeconds.value
  if (sec == null) return t.value('accountSessionServerNone')
  return formatRemaining(Math.max(0, sec) * 1000)
})

const serverCheckedAtText = computed(() => {
  const at = lastSessionCheckedAt.value
  if (at == null) return t.value('emptyValue')
  try {
    return new Date(at).toLocaleString()
  } catch {
    return String(at)
  }
})

const serverTimeText = computed(() => {
  const sec = lastSessionServerTimeUnix.value
  if (sec == null) return t.value('accountSessionServerNone')
  try {
    return new Date(sec * 1000).toLocaleString()
  } catch {
    return String(sec)
  }
})

const serverClockSkewText = computed(() => {
  const serverUnix = lastSessionServerTimeUnix.value
  const checkedAt = lastSessionCheckedAt.value
  if (serverUnix == null || checkedAt == null) return t.value('accountSessionServerNone')
  const skewSec = Math.round(checkedAt / 1000 - serverUnix)
  if (Math.abs(skewSec) < 2) return t.value('accountSessionSkewOk')
  const sign = skewSec > 0 ? '+' : ''
  return `${sign}${skewSec}s`
})


function cancelAccountAction() {
  accountActionAbort.cancel()
}

function mapAccountActionError(err: string): { message: string; retryable: boolean } {
  if (isCanceledError(err)) {
    return { message: t.value('adminActionCancelled'), retryable: false }
  }
  if (isTimeoutError(err)) {
    return { message: t.value('adminActionTimedOut'), retryable: true }
  }
  return { message: err, retryable: true }
}

async function renewSessionWithPassword() {
  if (renewLoading.value) return
  renewError.value = ''
  renewRetryable.value = false
  if (shouldBlockOfflineMutation(offline.value)) {
    renewError.value = t.value('offlineAccountBlocked')
    notifyError(renewError.value)
    return
  }
  const user = currentUser.value || ''
  if (!user) {
    renewError.value = t.value('accountSessionRenewNeedUser')
    notifyError(renewError.value)
    return
  }
  if (!renewPassword.value) {
    renewError.value = t.value('accountSessionRenewPasswordPlaceholder')
    return
  }
  renewLoading.value = true
  accountActionStartedAt.value = Date.now()
  const signal = accountActionAbort.begin()
  try {
    const err = await login(user, renewPassword.value, {
      ttlSeconds: renewTtlSeconds.value,
      signal,
    })
    if (err) {
      const mapped = mapAccountActionError(err)
      renewError.value = mapped.message
      renewRetryable.value = mapped.retryable
      // 错误密码时清空输入，避免重复提交；会话仍有效
      renewPassword.value = ''
      if (mapped.retryable) notifyError(mapped.message)
      else notifyInfo(mapped.message)
      return
    }
    renewPassword.value = ''
    success(t.value('accountSessionRenewOk'))
  } finally {
    accountActionAbort.end()
    renewLoading.value = false
    accountActionStartedAt.value = null
  }
}

const storage = typeof localStorage !== 'undefined' ? localStorage : null
const landingPath = ref(loadLandingPath(storage))
const landingFilter = ref('')
const landingOptions = computed(() =>
  LANDING_PATH_OPTIONS.filter((p) => isAdmin.value || !isAdminOnlyLandingPath(p)),
)
const landingViews = computed(() =>
  buildLandingOptionViews(landingOptions.value, landingLabel, isAdminOnlyLandingPath),
)
const filteredLandingViews = computed(() => filterLandingOptions(landingViews.value, landingFilter.value))
const groupedLanding = computed(() => groupLandingOptions(filteredLandingViews.value))

function applyAccountPrefillFromRoute() {
  const pre = parseAccountPrefill(route.query as Record<string, unknown>)
  if (pre.landing_q != null && landingFilter.value !== pre.landing_q) {
    landingFilter.value = pre.landing_q
    success(t.value('accountPrefillApplied'))
  }
}

async function copyAccountShareLink() {
  const path = accountFormToPrefill({ landing_q: landingFilter.value })
  const url = `${window.location.origin}${path}`
  const res = await copyText(url)
  if (res.ok) success(t.value('accountShareCopied'))
  else notifyError(res.error || t.value('failed'))
}

onMounted(() => {
  void bootstrapPasswordPolicy()
  unregisterAccountDirty = registerDirtyChecker('account', () => passwordFormDirty.value)
  window.addEventListener('beforeunload', onAccountBeforeUnload)
  applyAccountPrefillFromRoute()
  nowMs.value = Date.now()
  const armSessionClock = () => {
    if (sessionClock) clearInterval(sessionClock)
    const ms = sessionClockTickMs(sessionView.value.urgency, sessionView.value.remainingMs)
    sessionClock = setInterval(() => {
      nowMs.value = Date.now()
      armSessionClock()
    }, ms)
  }
  armSessionClock()
})

onBeforeUnmount(() => {
  accountActionAbort.cancel()
  unregisterAccountDirty?.()
  unregisterAccountDirty = null
  window.removeEventListener('beforeunload', onAccountBeforeUnload)
  if (sessionClock) {
    clearInterval(sessionClock)
    sessionClock = null
  }
})

watch(
  () => route.fullPath,
  (path, prev) => {
    if (prev != null && path !== prev) applyAccountPrefillFromRoute()
  },
)

function selectLanding(path: string) {
  if (landingPath.value === path) return
  landingPath.value = path
  onLandingChange()
}
function landingLabel(path: string): string {
  // map path -> route name roughly via pageTitle keys
  const map: Record<string, MessageKey> = {
    '/': 'overview',
    '/query': 'query',
    '/write': 'write',
    '/users': 'users',
    '/access': 'accessMatrix',
    '/access/grants': 'accessGrants',
    '/databases': 'databases',
    '/observability/metrics': 'metrics',
    '/config': 'config',
    '/operations': 'operations',
    '/downsample': 'downsample',
    '/audit': 'audit',
    '/api-spec': 'apiSpec',
    '/storage': 'storage',
    '/ops/readiness': 'readiness',
    '/about': 'about',
    '/account': 'account',
  }
  const key = map[path]
  return key ? t.value(key) : path
}
function onLandingChange() {
  saveLandingPath(storage, landingPath.value)
  success(t.value('accountLandingSaved'))
}

const { density, setDensity } = useDensity()
const { theme, setTheme } = useTheme()
const prefsImportText = ref('')
const prefsImportError = ref('')
const prefsFileRef = ref<HTMLInputElement | null>(null)

function emitPrefsChanged() {
  if (typeof window === 'undefined') return
  try {
    window.dispatchEvent(new CustomEvent(CLIENT_PREFS_CHANGED_EVENT))
  } catch { /* ignore */ }
}

function applyClientPrefs(prefs: ClientPrefs) {
  const n = normalizeClientPrefs(prefs)
  landingPath.value = n.landing_path
  saveLandingPath(storage, n.landing_path)
  setDensity(n.density)
  saveSidebarPrefs(storage, { collapsed: n.sidebar_collapsed })
  setLocale(n.locale)
  setTheme(n.theme)
  saveNavOrderMap(storage, n.nav_order || {})
  emitPrefsChanged()
}

function resetClientPrefs() {
  prefsImportError.value = ''
  applyClientPrefs(DEFAULT_CLIENT_PREFS)
  success(t.value('accountPrefsReset'))
}

function importClientPrefsFromText(raw: string) {
  prefsImportError.value = ''
  const parsed = parseClientPrefsImport(raw)
  if (!parsed.ok) {
    prefsImportError.value = t.value(
      parsed.error === 'empty'
        ? 'accountPrefsImportEmpty'
        : parsed.error === 'invalid_json'
          ? 'accountPrefsImportInvalidJson'
          : 'accountPrefsImportInvalidShape',
    )
    return
  }
  applyClientPrefs(parsed.prefs)
  prefsImportText.value = ''
  success(t.value('accountPrefsImported'))
}

function onPrefsImportSubmit() {
  importClientPrefsFromText(prefsImportText.value)
}

function onPrefsFilePick() {
  prefsFileRef.value?.click()
}

async function onPrefsFileChange(e: Event) {
  const input = e.target as HTMLInputElement
  const file = input.files?.[0]
  input.value = ''
  if (!file) return
  try {
    const text = await file.text()
    importClientPrefsFromText(text)
  } catch {
    prefsImportError.value = t.value('accountPrefsImportInvalidJson')
  }
}
function onDensityChange(e: Event) {
  const v = (e.target as HTMLSelectElement).value as UiDensity
  setDensity(v)
  success(t.value('accountDensitySaved'))
}

const { offline } = useNetworkStatus()
const oldPassword = ref('')
const newPassword = ref('')
const confirmPassword = ref('')
const loading = ref(false)
const accountActionStartedAt = ref<number | null>(null)
const accountActionAbort = createActionAbort()
const error = ref('')
const passwordRetryable = ref(false)
const info = ref('')
const invalid = computed(() => !!error.value)
const passwordFormDirty = computed(
  () => !!(oldPassword.value || newPassword.value || confirmPassword.value),
)

function onAccountBeforeUnload(e: BeforeUnloadEvent) {
  if (!passwordFormDirty.value) return
  e.preventDefault()
  e.returnValue = ''
}

let unregisterAccountDirty: (() => void) | null = null


function accountSnapshotInput() {
  const side = loadSidebarPrefs(storage)
  return {
    username: currentUser.value || '',
    role: currentRole.value || '',
    session: {
      expires_at: expiresAtText.value,
      remaining: remainingText.value,
      urgency: sessionView.value.urgency,
    },
    prefs: {
      landing_path: landingPath.value,
      density: density.value,
      sidebar_collapsed: side.collapsed,
      locale: locale.value,
      theme: theme.value,
      nav_order: loadNavOrderPrefs(storage),
    },
  }
}

async function exportAccount() {
  if (exportBusy.value) return
  const payload = buildAccountExport(accountSnapshotInput())
  const outcome = await runJSONExport({
    label: 'JSON',
    filename: stampFilename('mts-account', 'json'),
    total: 1,
    build: async ({ isCancelled, progress }) => {
      progress(0, 1)
      if (isCancelled()) return null
      progress(1, 1)
      return payload
    },
  })
  if (outcome === 'done') success(t.value('accountExported') || t.value('inventoryExported'))
  else if (outcome === 'cancelled') notifyInfo(t.value('exportCancelledToast'))
  else if (outcome === 'error') notifyError(exportJob.value.error || t.value('failed'))
}


async function copyAccount() {
  const res = await copyText(formatAccountExportPretty(accountSnapshotInput()))
  if (res.ok) success(t.value('accountCopied'))
  else notifyError(res.error || t.value('failed'))
}

function currentClientPrefs() {
  const snap = accountSnapshotInput().prefs
  return {
    landing_path: snap.landing_path,
    density: snap.density as ClientPrefs['density'],
    sidebar_collapsed: snap.sidebar_collapsed,
    locale: snap.locale as ClientPrefs['locale'],
    theme: snap.theme as ClientPrefs['theme'],
    nav_order: snap.nav_order || {},
  }
}

async function exportClientPrefsOnly() {
  if (exportBusy.value) return
  const payload = buildClientPrefsExport(currentClientPrefs())
  const outcome = await runJSONExport({
    label: 'JSON',
    filename: stampFilename('mts-client-prefs', 'json'),
    total: 1,
    build: async ({ isCancelled, progress }) => {
      progress(0, 1)
      if (isCancelled()) return null
      progress(1, 1)
      return payload
    },
  })
  if (outcome === 'done') success(t.value('accountPrefsExported'))
  else if (outcome === 'cancelled') notifyInfo(t.value('exportCancelledToast'))
  else if (outcome === 'error') notifyError(exportJob.value.error || t.value('failed'))
}

async function copyClientPrefsOnly() {
  const res = await copyText(formatClientPrefsExportPretty(currentClientPrefs()))
  if (res.ok) success(t.value('accountPrefsCopied'))
  else notifyError(res.error || t.value('failed'))
}

async function submit() {
  if (loading.value) return
  error.value = ''
  info.value = ''
  passwordRetryable.value = false
  if (shouldBlockOfflineMutation(offline.value)) {
    error.value = t.value('offlineAccountBlocked')
    notifyError(error.value)
    return
  }
  const check = validateNewPassword(oldPassword.value, newPassword.value, confirmPassword.value, {
    locale: locale.value,
  })
  if (!check.ok) {
    error.value = check.error || t.value('failed')
    return
  }
  loading.value = true
  accountActionStartedAt.value = Date.now()
  const signal = accountActionAbort.begin()
  try {
    const userForRelogin = currentUser.value || ''
    const err = await changePassword(oldPassword.value, newPassword.value, { signal })
    if (err) {
      const mapped = mapAccountActionError(err)
      error.value = mapped.message
      passwordRetryable.value = mapped.retryable
      if (mapped.retryable) notifyError(mapped.message)
      else {
        info.value = mapped.message
        notifyInfo(mapped.message)
      }
      return
    }
    // 成功后清脏表单，避免路由守卫拦截跳转登录
    oldPassword.value = ''
    newPassword.value = ''
    confirmPassword.value = ''
    error.value = ''
    passwordRetryable.value = false
    success(t.value('accountPasswordChanged'))
    await router.replace({
      name: 'Login',
      query: { reason: 'password_changed', ...(userForRelogin ? { user: userForRelogin } : {}) },
    })
  } finally {
    accountActionAbort.end()
    loading.value = false
    accountActionStartedAt.value = null
  }
}
</script>

<template>
  <div class="mx-auto max-w-xl space-y-6" data-testid="account-page">
    <div class="flex flex-wrap items-start justify-between gap-3">
      <div>
        <h1 class="mts-title flex items-center gap-2">
          <UserRound class="h-5 w-5" aria-hidden="true" />
          {{ t('accountTitle') }}
          <span
            v-if="passwordFormDirty"
            data-testid="account-password-dirty-badge"
            class="rounded-full bg-amber-100 px-2 py-0.5 text-[11px] font-medium text-amber-800 dark:bg-amber-950/50 dark:text-amber-200"
          >{{ t('accountPasswordDirtyBadge') }}</span>
        </h1>
        <div class="mt-1 flex flex-wrap items-center gap-2">
          <p class="text-xs mts-muted">{{ t('accountDesc') }}</p>
          <AdminOpLastChip
            v-if="accountAdminLastLabel"
            :label="accountAdminLastLabel"
            :last-ok="adminOpBusySummary?.lastOk"
            :last-error="accountAdminLastErrorDetail"
            test-id="account-admin-last"
            show-copy
            copy-test-id="account-admin-last-copy"
            error-test-id="account-admin-last-error"
          />

        </div>
      </div>
      <div class="flex flex-wrap gap-2">
        <ExportJobBanner class="w-full basis-full" :job="exportJob" :retryable="canRetryExport" @cancel="cancelExport" @retry="retryLastExport" @dismiss="resetExport" />
        <button type="button" class="mts-btn" data-testid="account-share-link" @click="copyAccountShareLink">
          {{ t('accountShareLink') }}
        </button>
        <button type="button" class="mts-btn" data-testid="account-export-json" :disabled="exportBusy" @click="exportAccount">
          <Download class="h-3.5 w-3.5" /> {{ t('accountExportJSON') }}
        </button>
        <button type="button" class="mts-btn" data-testid="account-copy-snapshot" @click="copyAccount">
          <Copy class="h-3.5 w-3.5" /> {{ t('accountCopySnapshot') }}
        </button>
      </div>
    </div>

    <div class="mts-card p-4">
      <h2 class="mb-3 text-sm font-semibold">{{ t('accountProfile') }}</h2>
      <dl class="space-y-2 text-sm">
        <div class="flex justify-between gap-3">
          <dt class="mts-muted">{{ t('username') }}</dt>
          <dd class="font-mono" data-testid="account-username">{{ currentUser || t('emptyValue') }}</dd>
        </div>
        <div class="flex justify-between gap-3">
          <dt class="mts-muted">{{ t('accountRole') }}</dt>
          <dd class="font-mono" data-testid="account-role">{{ roleLabel(currentRole) }}</dd>
        </div>
      </dl>
    </div>

    <div id="account-landing" class="mts-card scroll-mt-20 p-4" data-testid="account-landing">
      <h2 class="mb-1 text-sm font-semibold">{{ t('accountLandingTitle') }}</h2>
      <p class="mb-3 text-xs mts-muted">{{ t('accountLandingHint') }}</p>
      <div class="mb-2 flex flex-wrap items-center justify-between gap-2">
        <label class="text-sm font-medium" for="account-landing-filter">{{ t('accountLandingLabel') }}</label>
        <span class="text-[11px] mts-muted" data-testid="account-landing-count">
          {{ formatMessage(t('accountLandingCount'), { shown: filteredLandingViews.length, total: landingViews.length }) }}
        </span>
      </div>
      <input
        id="account-landing-filter"
        v-model="landingFilter"
        class="mts-input mts-focus-ring mb-3"
        data-testid="account-landing-filter"
        :placeholder="t('accountLandingFilterPh')"
      />
      <!-- 保留原生 select 兼容与 a11y 降级 -->
      <select
        id="account-landing-select"
        v-model="landingPath"
        class="mts-input mts-focus-ring mb-3"
        data-testid="account-landing-select"
        @change="onLandingChange"
      >
        <option v-for="p in landingOptions" :key="p" :value="p">
          {{ landingLabel(p) }} ({{ p }})
        </option>
      </select>
      <div v-if="filteredLandingViews.length" class="space-y-3" data-testid="account-landing-list">
        <div v-if="groupedLanding.common.length">
          <p class="mb-1 text-[11px] font-medium uppercase tracking-wide mts-muted">{{ t('accountLandingGroupCommon') }}</p>
          <ul class="divide-y divide-slate-100 overflow-hidden rounded-lg border border-slate-100 dark:divide-slate-800 dark:border-slate-800">
            <li v-for="it in groupedLanding.common" :key="it.path">
              <button
                type="button"
                class="flex w-full items-center justify-between gap-2 px-3 py-2 text-left text-sm hover:bg-slate-50 dark:hover:bg-slate-800/60"
                :data-testid="`account-landing-item-${it.path.replaceAll('/', '_') || 'root'}`"
                :aria-pressed="landingPath === it.path"
                :class="landingPath === it.path ? 'bg-slate-50 dark:bg-slate-800/80' : ''"
                @click="selectLanding(it.path)"
              >
                <span>
                  <span class="font-medium text-slate-800 dark:text-slate-100">{{ it.label }}</span>
                  <span class="ml-2 font-mono text-[11px] mts-muted">{{ it.path }}</span>
                </span>
                <span v-if="landingPath === it.path" class="text-[11px] text-emerald-600 dark:text-emerald-300" data-testid="account-landing-current">{{ t('accountLandingCurrent') }}</span>
              </button>
            </li>
          </ul>
        </div>
        <div v-if="groupedLanding.admin.length">
          <p class="mb-1 text-[11px] font-medium uppercase tracking-wide mts-muted">{{ t('accountLandingGroupAdmin') }}</p>
          <ul class="divide-y divide-slate-100 overflow-hidden rounded-lg border border-slate-100 dark:divide-slate-800 dark:border-slate-800">
            <li v-for="it in groupedLanding.admin" :key="it.path">
              <button
                type="button"
                class="flex w-full items-center justify-between gap-2 px-3 py-2 text-left text-sm hover:bg-slate-50 dark:hover:bg-slate-800/60"
                :data-testid="`account-landing-item-${it.path.replaceAll('/', '_') || 'root'}`"
                :aria-pressed="landingPath === it.path"
                :class="landingPath === it.path ? 'bg-slate-50 dark:bg-slate-800/80' : ''"
                @click="selectLanding(it.path)"
              >
                <span>
                  <span class="font-medium text-slate-800 dark:text-slate-100">{{ it.label }}</span>
                  <span class="ml-2 font-mono text-[11px] mts-muted">{{ it.path }}</span>
                  <span class="ml-2 rounded bg-amber-50 px-1.5 py-0.5 text-[10px] text-amber-700 dark:bg-amber-950/40 dark:text-amber-200">{{ t('accountLandingAdminBadge') }}</span>
                </span>
                <span v-if="landingPath === it.path" class="text-[11px] text-emerald-600 dark:text-emerald-300">{{ t('accountLandingCurrent') }}</span>
              </button>
            </li>
          </ul>
        </div>
      </div>
      <EmptyState
        v-else
        compact
        data-testid="account-landing-empty"
        :title="t('accountLandingEmptyTitle')"
        :description="t('accountLandingEmptyDesc')"
      />
    </div>

    <div id="account-density" class="mts-card scroll-mt-20 p-4" data-testid="account-density">
      <h2 class="mb-1 text-sm font-semibold">{{ t('accountDensityTitle') }}</h2>
      <p class="mb-3 text-xs mts-muted">{{ t('accountDensityHint') }}</p>
      <label class="mb-1 block text-sm font-medium" for="account-density-select">{{ t('accountDensityLabel') }}</label>
      <select
        id="account-density-select"
        class="mts-input mts-focus-ring"
        data-testid="account-density-select"
        :value="density"
        @change="onDensityChange"
      >
        <option value="comfortable">{{ t('accountDensityComfortable') }}</option>
        <option value="compact">{{ t('accountDensityCompact') }}</option>
      </select>
    </div>

    <div id="account-prefs-tools" class="mts-card scroll-mt-20 p-4" data-testid="account-prefs-tools">
      <h2 class="mb-1 text-sm font-semibold">{{ t('accountPrefsToolsTitle') }}</h2>
      <p class="mb-3 text-xs mts-muted">{{ t('accountPrefsToolsHint') }}</p>
      <div class="mb-3 flex flex-wrap gap-2">
        <button type="button" class="mts-btn mts-focus-ring" data-testid="account-prefs-export" :disabled="exportBusy" @click="exportClientPrefsOnly">
          <Download class="h-3.5 w-3.5" aria-hidden="true" />
          {{ t('accountPrefsExportBtn') }}
        </button>
        <button type="button" class="mts-btn mts-focus-ring" data-testid="account-prefs-copy" @click="copyClientPrefsOnly">
          <Copy class="h-3.5 w-3.5" aria-hidden="true" />
          {{ t('accountPrefsCopyBtn') }}
        </button>
        <button type="button" class="mts-btn mts-focus-ring" data-testid="account-prefs-reset" @click="resetClientPrefs">
          {{ t('accountPrefsResetBtn') }}
        </button>
        <button type="button" class="mts-btn mts-focus-ring" data-testid="account-prefs-import-file" @click="onPrefsFilePick">
          {{ t('accountPrefsImportFile') }}
        </button>
        <input
          ref="prefsFileRef"
          type="file"
          accept="application/json,.json"
          class="hidden"
          data-testid="account-prefs-file-input"
          @change="onPrefsFileChange"
        />
      </div>
      <label class="mb-1 block text-sm font-medium" for="account-prefs-import-text">{{ t('accountPrefsImportLabel') }}</label>
      <textarea
        id="account-prefs-import-text"
        v-model="prefsImportText"
        rows="4"
        class="mts-input mts-focus-ring font-mono text-xs"
        data-testid="account-prefs-import-text"
        :placeholder="t('accountPrefsImportPlaceholder')"
      />
      <p
        v-if="prefsImportError"
        class="mt-2 text-xs text-red-600 dark:text-red-300"
        role="alert"
        data-testid="account-prefs-import-error"
      >{{ prefsImportError }}</p>
      <button
        type="button"
        class="mts-btn-primary mts-focus-ring mt-3"
        data-testid="account-prefs-import-submit"
        @click="onPrefsImportSubmit"
      >
        {{ t('accountPrefsImportBtn') }}
      </button>
    </div>

    <div id="account-session" class="mts-card scroll-mt-20 p-4" data-testid="account-session">
      <h2 class="mb-3 text-sm font-semibold">{{ t('accountSessionCard') }}</h2>
      <dl class="space-y-2 text-sm">
        <div class="flex justify-between gap-3">
          <dt class="mts-muted">{{ t('accountSessionExpiresAt') }}</dt>
          <dd class="font-mono" data-testid="account-session-expires">{{ expiresAtText }}</dd>
        </div>
        <div class="flex justify-between gap-3">
          <dt class="mts-muted">{{ t('accountSessionRemaining') }}</dt>
          <dd class="font-mono" data-testid="account-session-remaining">{{ remainingText }}</dd>
        </div>
        <div class="flex justify-between gap-3">
          <dt class="mts-muted">{{ t('sessionExpiry') }}</dt>
          <dd class="font-mono" data-testid="account-session-level">{{ sessionLevelLabel }}</dd>
        </div>
        <div class="flex justify-between gap-3">
          <dt class="mts-muted">{{ t('accountSessionServerRemaining') }}</dt>
          <dd class="font-mono" data-testid="account-session-server-remaining">{{ serverRemainingText }}</dd>
        </div>
        <div class="flex justify-between gap-3">
          <dt class="mts-muted">{{ t('accountSessionServerCheckedAt') }}</dt>
          <dd class="font-mono" data-testid="account-session-server-checked-at">{{ serverCheckedAtText }}</dd>
        </div>
        <div class="flex justify-between gap-3">
          <dt class="mts-muted">{{ t('accountSessionServerTime') }}</dt>
          <dd class="font-mono" data-testid="account-session-server-time">{{ serverTimeText }}</dd>
        </div>
        <div class="flex justify-between gap-3">
          <dt class="mts-muted">{{ t('accountSessionClockSkew') }}</dt>
          <dd class="font-mono" data-testid="account-session-clock-skew">{{ serverClockSkewText }}</dd>
        </div>
      </dl>
      <p class="mt-3 text-xs mts-muted" data-testid="account-session-hint">{{ t('accountSessionRenewHint') }}</p>
      <div class="mt-3 flex flex-wrap gap-2">
        <button
          type="button"
          class="mts-btn-primary"
          data-testid="account-session-relogin"
          @click="reLoginPreserve"
        >
          {{ t('accountSessionRelogin') }}
        </button>
        <button
          type="button"
          class="mts-btn"
          data-testid="account-session-verify"
          :disabled="sessionVerifyLoading || offline"
          :aria-busy="sessionVerifyLoading ? 'true' : undefined"
          @click="verifyServerSession"
        >
          {{ sessionVerifyLoading ? t('loading') : t('accountSessionVerify') }}
        </button>
      </div>
      <p v-if="sessionVerifyNote" class="mt-2 text-xs mts-muted" data-testid="account-session-verify-note">{{ sessionVerifyNote }}</p>

      <form class="mt-4 space-y-2 border-t border-slate-200 pt-3 dark:border-slate-700" data-testid="account-session-renew-form" @submit.prevent="renewSessionWithPassword">
        <p class="text-xs font-medium text-slate-700 dark:text-slate-200">{{ t('accountSessionRenewWithPassword') }}</p>
        <label class="block text-xs mts-muted">
          {{ t('accountSessionRenewPasswordPlaceholder') }}
          <PasswordInputWithToggle
            v-model="renewPassword"
            autocomplete="current-password"
            input-class="mts-input mt-1 pr-10"
            test-id="account-session-renew-password"
            toggle-test-id="account-session-renew-toggle"
            :disabled="renewLoading"
          />
        </label>
        <label class="block text-xs mts-muted">
          {{ t('accountSessionRenewTtlLabel') }}
          <select v-model.number="renewTtlSeconds" class="mts-input mt-1" data-testid="account-session-renew-ttl" :disabled="renewLoading">
            <option v-for="o in renewTtlOptions" :key="o.value" :value="o.value">{{ t(o.labelKey) }}</option>
          </select>
          <p class="mt-1 text-[11px] mts-muted" data-testid="account-session-ttl-policy">
            {{ t('accountSessionTTLPolicy') }}: {{ policyTTLDefault }} / {{ policyTTLMax }}
          </p>
        </label>
        <div
          v-if="renewError"
          role="alert"
          aria-live="assertive"
          class="mb-1"
          data-testid="account-session-renew-error"
        >
          <ActionResultBanner
            kind="error"
            :message="renewError"
            :retryable="renewRetryable"
            data-testid="account-session-renew-error-banner"
            @retry="renewSessionWithPassword"
            @dismiss="renewError = ''; renewRetryable = false"
          />
        </div>
        <button
          type="submit"
          class="mts-btn-primary"
          data-testid="account-session-renew-submit"
          :disabled="renewLoading || !renewPassword || offline"
          :title="offline ? t('offlineAccountBlocked') : undefined"
        >
          {{ renewLoading ? t('accountSessionRenewBusy') : t('accountSessionRenewSubmit') }}
        </button>
      </form>
    </div>

    <InFlightBanner
      :active="loading || renewLoading"
      :started-at-ms="accountActionStartedAt"
      kind="admin"
      @cancel="cancelAccountAction"
    />

    <div class="mts-card p-4" data-testid="account-password-policy">
      <h2 class="mb-1 text-sm font-semibold">{{ t('accountPasswordPolicyTitle') }}</h2>
      <p class="mb-3 text-[11px] mts-muted" data-testid="account-password-policy-sync">{{ policyVersionLabel }}</p>
      <dl class="grid grid-cols-1 gap-2 text-xs sm:grid-cols-2">
        <div>
          <dt class="mts-muted">{{ t('accountPasswordPolicyMin') }}</dt>
          <dd class="font-mono" data-testid="account-password-policy-min">{{ policyMinLength }}</dd>
        </div>
        <div>
          <dt class="mts-muted">{{ t('accountPasswordPolicyForbidden') }}</dt>
          <dd class="font-mono" data-testid="account-password-policy-forbidden">{{ policyForbidden }}</dd>
        </div>
        <div class="sm:col-span-2">
          <dt class="mts-muted">{{ t('accountSessionTTLPolicy') }}</dt>
          <dd class="font-mono" data-testid="account-password-policy-ttl">{{ policyTTLDefault }} / {{ policyTTLMax }}</dd>
        </div>
      </dl>
    </div>

    <div class="mts-card p-4">
      <h2 class="mb-1 flex items-center gap-2 text-sm font-semibold">
        <KeyRound class="h-4 w-4" aria-hidden="true" />
        {{ t('accountChangePassword') }}
      </h2>
      <p class="mb-4 text-xs mts-muted">{{ t('accountChangePasswordHint') }}</p>

      <div
        v-if="error"
        role="alert"
        aria-live="assertive"
        :aria-label="t('accountErrorRegion')"
        data-testid="account-password-error"
        class="mb-3"
      >
        <ActionResultBanner
          kind="error"
          :message="error"
          :retryable="passwordRetryable"
          data-testid="account-password-error-banner"
          @retry="submit"
          @dismiss="error = ''; passwordRetryable = false"
        />
      </div>
      <ActionResultBanner v-if="info" kind="info" :message="info" @dismiss="info = ''" />

      <form id="account-password" class="space-y-3 scroll-mt-20" data-testid="account-password-form" @submit.prevent="submit">
        <div>
          <label class="mb-1 block text-sm font-medium" for="acct-old">{{ t('accountOldPassword') }}</label>
          <PasswordInputWithToggle
            id="acct-old"
            v-model="oldPassword"
            autocomplete="current-password"
            test-id="account-old-password"
            toggle-test-id="account-toggle-old"
            :invalid="invalid"
            :described-by="error ? 'account-password-error-desc' : undefined"
          />
        </div>
        <div>
          <label class="mb-1 block text-sm font-medium" for="acct-new">{{ t('accountNewPassword') }}</label>
          <PasswordInputWithToggle
            id="acct-new"
            v-model="newPassword"
            autocomplete="new-password"
            test-id="account-new-password"
            toggle-test-id="account-toggle-new"
            :invalid="invalid"
            :described-by="error ? 'account-password-error-desc' : undefined"
          />
        </div>
        <div>
          <label class="mb-1 block text-sm font-medium" for="acct-confirm">{{ t('accountConfirmPassword') }}</label>
          <PasswordInputWithToggle
            id="acct-confirm"
            v-model="confirmPassword"
            autocomplete="new-password"
            test-id="account-confirm-password"
            toggle-test-id="account-toggle-confirm"
            :invalid="invalid"
            :described-by="error ? 'account-password-error-desc' : undefined"
          />
        </div>
        <p v-if="error" id="account-password-error-desc" class="sr-only">{{ error }}</p>
      <PasswordHints
        :old-password="oldPassword"
        :new-password="newPassword"
        :confirm-password="confirmPassword"
      />
<button
          type="submit"
          class="mts-btn-primary mts-focus-ring"
          :disabled="loading || offline"
          :title="offline ? t('offlineAccountBlocked') : undefined"
          data-testid="account-password-submit"
          :aria-busy="loading ? 'true' : undefined"
        >
          {{ loading ? t('loading') : t('accountSubmitPassword') }}
        </button>
      </form>
    </div>
  </div>
</template>
