<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useAuth } from '@/composables/useAuth'
import { getTokenExpiresAt } from '@/api/client'
import { parseExpiresAt, sessionExpiryView, formatRemaining } from '@/utils/sessionExpiry'
import { useI18n } from '@/composables/useI18n'
import { useNotify } from '@/composables/useNotify'
import ActionResultBanner from '@/components/ActionResultBanner.vue'
import EmptyState from '@/components/EmptyState.vue'
import { validateNewPassword } from '@/utils/passwordPolicy'
import { KeyRound, UserRound, Download, Copy } from 'lucide-vue-next'
import PasswordHints from '@/components/PasswordHints.vue'
import { buildAccountExport, formatAccountExportPretty } from '@/utils/accountExport'
import { buildClientPrefsExport, formatClientPrefsExportPretty } from '@/utils/clientPrefsExport'
import { downloadJSON, stampFilename } from '@/utils/download'
import { copyText } from '@/utils/clipboard'
import { formatMessage } from '@/utils/formatMessage'
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

const router = useRouter()
const { currentUser, currentRole, changePassword, isAdmin } = useAuth()
const { t, locale, setLocale } = useI18n()
const nowMs = ref(Date.now())
const expiresAt = computed(() => parseExpiresAt(getTokenExpiresAt()))
const sessionView = computed(() =>
  sessionExpiryView(
    expiresAt.value,
    nowMs.value,
    10 * 60_000,
    2 * 60_000,
    locale.value === 'en' ? 'en' : 'zh',
  ),
)
const remainingText = computed(() => {
  if (expiresAt.value == null) return t.value('accountSessionNone')
  if (sessionView.value.urgency === 'expired') return t.value('sessionExpiredLabel')
  return formatRemaining(Math.max(0, sessionView.value.remainingMs))
})
const expiresAtText = computed(() => {
  if (expiresAt.value == null) return t.value('emptyValue')
  try {
    return new Date(expiresAt.value).toLocaleString()
  } catch {
    return String(expiresAt.value)
  }
})
function roleLabel(role?: string | null): string {
  if (!role) return t.value('emptyValue')
  if (role === 'admin') return t.value('roleAdmin')
  if (role === 'user') return t.value('roleUser')
  return role
}
const { success, error: notifyError } = useNotify()
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

const oldPassword = ref('')
const newPassword = ref('')
const confirmPassword = ref('')
const loading = ref(false)
const error = ref('')
const info = ref('')
const invalid = computed(() => !!error.value)


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

function exportAccount() {
  downloadJSON(stampFilename('mts-account', 'json'), buildAccountExport(accountSnapshotInput()))
  success(t.value('accountExported'))
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

function exportClientPrefsOnly() {
  downloadJSON(
    stampFilename('mts-client-prefs', 'json'),
    buildClientPrefsExport(currentClientPrefs()),
  )
  success(t.value('accountPrefsExported'))
}

async function copyClientPrefsOnly() {
  const res = await copyText(formatClientPrefsExportPretty(currentClientPrefs()))
  if (res.ok) success(t.value('accountPrefsCopied'))
  else notifyError(res.error || t.value('failed'))
}

async function submit() {
  error.value = ''
  info.value = ''
  const check = validateNewPassword(oldPassword.value, newPassword.value, confirmPassword.value, {
    locale: locale.value,
  })
  if (!check.ok) {
    error.value = check.error || t.value('failed')
    return
  }
  loading.value = true
  try {
    const err = await changePassword(oldPassword.value, newPassword.value)
    if (err) {
      error.value = err
      return
    }
    success(t.value('accountPasswordChanged'))
    await router.replace({ name: 'Login', query: { reason: 'password_changed' } })
  } finally {
    loading.value = false
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
        </h1>
        <p class="text-xs mts-muted">{{ t('accountDesc') }}</p>
      </div>
      <div class="flex flex-wrap gap-2">
        <button type="button" class="mts-btn" data-testid="account-export-json" @click="exportAccount">
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

    <div class="mts-card p-4" data-testid="account-landing">
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

    <div class="mts-card p-4" data-testid="account-density">
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

    <div class="mts-card p-4" data-testid="account-prefs-tools">
      <h2 class="mb-1 text-sm font-semibold">{{ t('accountPrefsToolsTitle') }}</h2>
      <p class="mb-3 text-xs mts-muted">{{ t('accountPrefsToolsHint') }}</p>
      <div class="mb-3 flex flex-wrap gap-2">
        <button type="button" class="mts-btn mts-focus-ring" data-testid="account-prefs-export" @click="exportClientPrefsOnly">
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

    <div class="mts-card p-4" data-testid="account-session">
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
          <dd class="font-mono" data-testid="account-session-level">{{ sessionView.urgency }}</dd>
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
        <ActionResultBanner kind="error" :message="error" @dismiss="error = ''" />
      </div>
      <ActionResultBanner v-if="info" kind="info" :message="info" @dismiss="info = ''" />

      <form class="space-y-3" data-testid="account-password-form" @submit.prevent="submit">
        <div>
          <label class="mb-1 block text-sm font-medium" for="acct-old">{{ t('accountOldPassword') }}</label>
          <input
            id="acct-old"
            v-model="oldPassword"
            type="password"
            autocomplete="current-password"
            class="mts-input mts-focus-ring"
            data-testid="account-old-password"
            :aria-invalid="invalid ? 'true' : undefined"
            :aria-describedby="error ? 'account-password-error-desc' : undefined"
          />
        </div>
        <div>
          <label class="mb-1 block text-sm font-medium" for="acct-new">{{ t('accountNewPassword') }}</label>
          <input
            id="acct-new"
            v-model="newPassword"
            type="password"
            autocomplete="new-password"
            class="mts-input mts-focus-ring"
            data-testid="account-new-password"
            :aria-invalid="invalid ? 'true' : undefined"
            :aria-describedby="error ? 'account-password-error-desc' : undefined"
          />
        </div>
        <div>
          <label class="mb-1 block text-sm font-medium" for="acct-confirm">{{ t('accountConfirmPassword') }}</label>
          <input
            id="acct-confirm"
            v-model="confirmPassword"
            type="password"
            autocomplete="new-password"
            class="mts-input mts-focus-ring"
            data-testid="account-confirm-password"
            :aria-invalid="invalid ? 'true' : undefined"
            :aria-describedby="error ? 'account-password-error-desc' : undefined"
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
          :disabled="loading"
          data-testid="account-password-submit"
          :aria-busy="loading ? 'true' : undefined"
        >
          {{ loading ? t('loading') : t('accountSubmitPassword') }}
        </button>
      </form>
    </div>
  </div>
</template>
