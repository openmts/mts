<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useAuth } from '@/composables/useAuth'
import { useI18n } from '@/composables/useI18n'
import { sanitizeRedirect } from '@/router'
import { formatRedirectLabel } from '@/utils/redirect'
import { loadLandingPath, resolveLandingPath } from '@/utils/landingPrefs'
import { loginReasonMessage } from '@/utils/authReason'
import { parseLoginTTLSeconds } from '@/utils/loginTTL'
import { loadLoginTTLPref, saveLoginTTLPref } from '@/utils/loginTTLPrefs'
import {
  clearLoginUsernamePref,
  loadLoginUsernamePref,
  saveLoginUsernamePref,
} from '@/utils/loginUsernamePrefs'
import { Eye, EyeOff, Server } from 'lucide-vue-next'

const router = useRouter()
const { login, mustChangePassword, isAdmin } = useAuth()
const { t, locale } = useI18n()

const storage = typeof localStorage !== 'undefined' ? localStorage : null
const remembered = loadLoginUsernamePref(storage)
const username = ref(remembered || 'admin')
const password = ref('')
const showPassword = ref(false)
const rememberUsername = ref(!!remembered)
const ttlSeconds = ref(loadLoginTTLPref(storage))
const loading = ref(false)
const error = ref('')
const reasonHint = computed(() =>
  loginReasonMessage(router.currentRoute.value.query.reason, locale.value),
)
const pendingRedirect = computed(() =>
  sanitizeRedirect(router.currentRoute.value.query.redirect),
)
const pendingRedirectLabel = computed(() =>
  pendingRedirect.value ? formatRedirectLabel(pendingRedirect.value) : '',
)
const invalid = computed(() => !!error.value)

async function handleLogin() {
  if (!username.value.trim() || !password.value) {
    error.value = t.value('loginNeedCredentials')
    return
  }
  const ttl = parseLoginTTLSeconds(ttlSeconds.value)
  if (!ttl.ok) {
    error.value = t.value('loginTTLInvalid')
    return
  }
  error.value = ''
  loading.value = true
  try {
    const err = await login(
      username.value.trim(),
      password.value,
      ttl.seconds != null ? { ttlSeconds: ttl.seconds } : undefined,
    )
    if (err) {
      error.value = err
    } else {
      saveLoginTTLPref(storage, ttlSeconds.value)
      if (rememberUsername.value) {
        saveLoginUsernamePref(storage, username.value)
      } else {
        clearLoginUsernamePref(storage)
      }
      if (mustChangePassword.value) {
        const q: Record<string, string> = {}
        if (pendingRedirect.value) q.redirect = pendingRedirect.value
        await router.push({ name: 'ForceChangePassword', query: q })
        return
      }
      const redirect = resolveLandingPath({
        redirectRaw: router.currentRoute.value.query.redirect,
        preferredPath: loadLandingPath(storage),
        isAdmin: isAdmin.value,
        sanitizeRedirect,
      })
      await router.push(redirect)
    }
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="flex min-h-screen items-center justify-center bg-slate-100 p-4 dark:bg-slate-950">
    <div
      class="w-full max-w-sm rounded-xl bg-white p-8 shadow-sm dark:border dark:border-slate-700 dark:bg-slate-900"
      data-testid="login-panel"
    >
      <div class="mb-6 flex flex-col items-center gap-2">
        <Server class="h-10 w-10 text-slate-700 dark:text-slate-200" aria-hidden="true" />
        <h1 class="text-xl font-semibold text-slate-800 dark:text-slate-100">{{ t('loginTitle') }}</h1>
        <p class="text-sm text-slate-500 dark:text-slate-400">{{ t('loginSubtitle') }}</p>
      </div>

      <p
        v-if="reasonHint"
        class="mb-4 rounded-lg border border-amber-200 bg-amber-50 px-3 py-2 text-xs text-amber-900 dark:border-amber-900 dark:bg-amber-950/40 dark:text-amber-100"
        role="status"
        data-testid="login-reason"
      >
        {{ reasonHint }}
      </p>

      <p
        v-if="pendingRedirect"
        class="mb-4 rounded-lg border border-slate-200 bg-slate-50 px-3 py-2 text-xs text-slate-700 dark:border-slate-700 dark:bg-slate-800/60 dark:text-slate-200"
        role="status"
        data-testid="login-redirect-hint"
      >
        {{ t('loginRedirectHint') }}
        <span class="mt-0.5 block break-all font-mono text-[11px]" data-testid="login-redirect-path">{{ pendingRedirectLabel }}</span>
      </p>

      <form class="space-y-4" data-testid="login-form" @submit.prevent="handleLogin">
        <div>
          <label class="mb-1 block text-sm font-medium text-slate-700 dark:text-slate-200" for="username">{{ t('username') }}</label>
          <input
            id="username"
            v-model="username"
            type="text"
            name="username"
            autocomplete="username"
            class="mts-input mts-focus-ring"
            data-testid="login-username"
            :placeholder="t('loginUsernamePlaceholder')"
            :aria-invalid="invalid ? 'true' : undefined"
            :aria-describedby="error ? 'login-error' : undefined"
          />
        </div>
        <div>
          <label class="mb-1 block text-sm font-medium text-slate-700 dark:text-slate-200" for="password">{{ t('password') }}</label>
          <div class="relative">
            <input
              id="password"
              v-model="password"
              :type="showPassword ? 'text' : 'password'"
              name="password"
              autocomplete="current-password"
              class="mts-input mts-focus-ring pr-10"
              data-testid="login-password"
              :placeholder="t('loginPasswordPlaceholder')"
              :aria-invalid="invalid ? 'true' : undefined"
              :aria-describedby="error ? 'login-error' : undefined"
            />
            <button
              type="button"
              class="mts-focus-ring absolute inset-y-0 right-1 my-auto inline-flex h-8 w-8 items-center justify-center rounded text-slate-400 hover:bg-slate-100 hover:text-slate-700 dark:hover:bg-slate-800 dark:hover:text-slate-200"
              data-testid="login-toggle-password"
              :aria-label="showPassword ? t('loginHidePassword') : t('loginShowPassword')"
              :title="showPassword ? t('loginHidePassword') : t('loginShowPassword')"
              :aria-pressed="showPassword ? 'true' : 'false'"
              @click="showPassword = !showPassword"
            >
              <EyeOff v-if="showPassword" class="h-4 w-4" aria-hidden="true" />
              <Eye v-else class="h-4 w-4" aria-hidden="true" />
            </button>
          </div>
        </div>

        <label class="flex items-center gap-2 text-xs text-slate-600 dark:text-slate-300">
          <input
            v-model="rememberUsername"
            type="checkbox"
            class="rounded border-slate-300"
            data-testid="login-remember-user"
          />
          {{ t('loginRememberUser') }}
        </label>

        <div>
          <label class="mb-1 block text-xs font-medium text-slate-600 dark:text-slate-300" for="ttl">{{ t('loginTTLLabel') }}</label>
          <input
            id="ttl"
            v-model="ttlSeconds"
            type="text"
            inputmode="numeric"
            autocomplete="off"
            class="mts-input mts-focus-ring"
            data-testid="login-ttl"
            :placeholder="t('loginTTLPlaceholder')"
            :aria-invalid="error === t('loginTTLInvalid') ? 'true' : undefined"
            :aria-describedby="error ? 'login-error login-ttl-hint' : 'login-ttl-hint'"
          />
          <p id="login-ttl-hint" class="mt-1 text-[11px] text-slate-500 dark:text-slate-400">{{ t('loginTTLHint') }}</p>
        </div>

        <p
          v-if="error"
          id="login-error"
          class="text-sm text-red-600 dark:text-red-300"
          role="alert"
          aria-live="assertive"
          :aria-label="t('loginErrorRegion')"
          data-testid="login-error"
        >{{ error }}</p>

        <button
          type="submit"
          class="mts-focus-ring w-full rounded-lg bg-slate-800 px-4 py-2 text-sm font-medium text-white hover:bg-slate-700 disabled:opacity-50 dark:bg-slate-100 dark:text-slate-900"
          data-testid="login-submit"
          :disabled="loading"
          :aria-busy="loading ? 'true' : undefined"
        >
          {{ loading ? t('loggingIn') : t('login') }}
        </button>
      </form>

      <div class="mt-6 rounded-lg border border-slate-200 bg-slate-50 p-3 text-[11px] text-slate-500 dark:border-slate-700 dark:bg-slate-900/50 dark:text-slate-400">
        <p class="font-medium text-slate-600 dark:text-slate-300">{{ t('loginDefaultPolicy') }}</p>
        <p class="mt-1">{{ t('loginDefaultHint') }}</p>
      </div>
    </div>
  </div>
</template>
