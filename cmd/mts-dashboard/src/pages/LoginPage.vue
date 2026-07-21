<script setup lang="ts">
import { computed, onBeforeUnmount, ref } from 'vue'
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
import { Server } from 'lucide-vue-next'
import PasswordInputWithToggle from '@/components/PasswordInputWithToggle.vue'
import InFlightBanner from '@/components/InFlightBanner.vue'
import { useNetworkStatus } from '@/composables/useNetworkStatus'
import { shouldBlockOfflineMutation } from '@/utils/offlineGuard'
import { createActionAbort } from '@/utils/actionAbort'
import { isCanceledError, isTimeoutError } from '@/utils/apiError'

const router = useRouter()
const { login, mustChangePassword, isAdmin } = useAuth()
const { offline } = useNetworkStatus()
const { t, locale } = useI18n()

const storage = typeof localStorage !== 'undefined' ? localStorage : null
const remembered = loadLoginUsernamePref(storage)
const username = ref(remembered || 'admin')
const password = ref('')
const rememberUsername = ref(!!remembered)
const ttlSeconds = ref(loadLoginTTLPref(storage))
const loading = ref(false)
const loginStartedAt = ref<number | null>(null)
const loginAbort = createActionAbort()
const error = ref('')
const errorRetryable = ref(false)
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

function cancelLogin() {
  loginAbort.cancel()
}

function mapLoginError(err: string): { message: string; retryable: boolean } {
  if (isCanceledError(err)) {
    return { message: t.value('loginCancelled'), retryable: false }
  }
  if (isTimeoutError(err)) {
    return { message: t.value('loginTimedOut'), retryable: true }
  }
  return { message: err, retryable: true }
}

async function handleLogin() {
  if (loading.value) return
  if (shouldBlockOfflineMutation(offline.value)) {
    error.value = t.value('offlineLoginBlocked')
    errorRetryable.value = true
    return
  }
  if (!username.value.trim() || !password.value) {
    error.value = t.value('loginNeedCredentials')
    errorRetryable.value = false
    return
  }
  const ttl = parseLoginTTLSeconds(ttlSeconds.value)
  if (!ttl.ok) {
    error.value = t.value('loginTTLInvalid')
    errorRetryable.value = false
    return
  }
  error.value = ''
  errorRetryable.value = false
  loading.value = true
  loginStartedAt.value = Date.now()
  const signal = loginAbort.begin()
  try {
    const err = await login(
      username.value.trim(),
      password.value,
      {
        ...(ttl.seconds != null ? { ttlSeconds: ttl.seconds } : {}),
        signal,
      },
    )
    if (err) {
      const mapped = mapLoginError(err)
      error.value = mapped.message
      errorRetryable.value = mapped.retryable
      return
    }
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
  } finally {
    loginAbort.end()
    loading.value = false
    loginStartedAt.value = null
  }
}

onBeforeUnmount(() => {
  cancelLogin()
})
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

      <InFlightBanner
        class="mb-4"
        :active="loading"
        :started-at-ms="loginStartedAt"
        kind="login"
        @cancel="cancelLogin"
      />

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
            :disabled="loading"
            :aria-invalid="invalid ? 'true' : undefined"
            :aria-describedby="error ? 'login-error' : undefined"
          />
        </div>
        <div>
          <label class="mb-1 block text-sm font-medium text-slate-700 dark:text-slate-200" for="password">{{ t('password') }}</label>
          <PasswordInputWithToggle
            id="password"
            v-model="password"
            autocomplete="current-password"
            test-id="login-password"
            toggle-test-id="login-toggle-password"
            name="password"
            :placeholder="t('loginPasswordPlaceholder')"
            :invalid="invalid"
            :disabled="loading"
            :described-by="error ? 'login-error' : undefined"
          />
        </div>

        <label class="flex items-center gap-2 text-xs text-slate-600 dark:text-slate-300">
          <input
            v-model="rememberUsername"
            type="checkbox"
            class="rounded border-slate-300"
            data-testid="login-remember-user"
            :disabled="loading"
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
            :disabled="loading"
            :aria-invalid="error === t('loginTTLInvalid') ? 'true' : undefined"
            :aria-describedby="error ? 'login-error login-ttl-hint' : 'login-ttl-hint'"
          />
          <p id="login-ttl-hint" class="mt-1 text-[11px] text-slate-500 dark:text-slate-400">{{ t('loginTTLHint') }}</p>
        </div>

        <div
          v-if="error"
          id="login-error"
          class="rounded-lg border border-red-200 bg-red-50 px-3 py-2 dark:border-red-900 dark:bg-red-950/40"
          role="alert"
          aria-live="assertive"
          :aria-label="t('loginErrorRegion')"
          data-testid="login-error"
        >
          <p class="text-sm text-red-700 dark:text-red-200">{{ error }}</p>
          <div class="mt-2 flex flex-wrap gap-2">
            <button
              v-if="errorRetryable"
              type="button"
              class="mts-btn text-xs"
              data-testid="login-error-retry"
              :disabled="loading || offline"
              @click="handleLogin"
            >{{ t('retry') }}</button>
            <button
              type="button"
              class="mts-btn text-xs"
              data-testid="login-error-dismiss"
              :disabled="loading"
              @click="error = ''; errorRetryable = false"
            >{{ t('dismiss') }}</button>
          </div>
        </div>

        <div class="flex gap-2">
          <button
            type="submit"
            class="mts-focus-ring flex-1 rounded-lg bg-slate-800 px-4 py-2 text-sm font-medium text-white hover:bg-slate-700 disabled:opacity-50 dark:bg-slate-100 dark:text-slate-900"
            data-testid="login-submit"
            :disabled="loading || offline"
            :title="offline ? t('offlineLoginBlocked') : (loading ? t('loggingIn') : undefined)"
            :aria-busy="loading ? 'true' : undefined"
          >
            {{ loading ? t('loggingIn') : t('login') }}
          </button>
          <button
            v-if="loading"
            type="button"
            class="mts-btn shrink-0"
            data-testid="login-cancel"
            :aria-label="t('cancel')"
            @click="cancelLogin"
          >{{ t('cancel') }}</button>
        </div>
      </form>

      <div class="mt-6 rounded-lg border border-slate-200 bg-slate-50 p-3 text-[11px] text-slate-500 dark:border-slate-700 dark:bg-slate-900/50 dark:text-slate-400">
        <p class="font-medium text-slate-600 dark:text-slate-300">{{ t('loginDefaultPolicy') }}</p>
        <p class="mt-1">{{ t('loginDefaultHint') }}</p>
      </div>
    </div>
  </div>
</template>
