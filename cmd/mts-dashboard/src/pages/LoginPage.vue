<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useAuth } from '@/composables/useAuth'
import { useI18n } from '@/composables/useI18n'
import { sanitizeRedirect } from '@/router'
import { loginReasonMessage } from '@/utils/authReason'
import { parseLoginTTLSeconds } from '@/utils/loginTTL'
import { loadLoginTTLPref, saveLoginTTLPref } from '@/utils/loginTTLPrefs'
import { Server } from 'lucide-vue-next'

const router = useRouter()
const { login, mustChangePassword } = useAuth()
const { t, locale } = useI18n()

const username = ref('admin')
const password = ref('')
const ttlSeconds = ref(loadLoginTTLPref(typeof localStorage !== 'undefined' ? localStorage : null))
const loading = ref(false)
const error = ref('')
const reasonHint = computed(() =>
  loginReasonMessage(router.currentRoute.value.query.reason, locale.value),
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
      saveLoginTTLPref(
        typeof localStorage !== 'undefined' ? localStorage : null,
        ttlSeconds.value,
      )
      if (mustChangePassword.value) {
        await router.push({ name: 'ForceChangePassword' })
        return
      }
      const redirect = sanitizeRedirect(router.currentRoute.value.query.redirect) || '/'
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
          <input
            id="password"
            v-model="password"
            type="password"
            name="password"
            autocomplete="current-password"
            class="mts-input mts-focus-ring"
            data-testid="login-password"
            :placeholder="t('loginPasswordPlaceholder')"
            :aria-invalid="invalid ? 'true' : undefined"
            :aria-describedby="error ? 'login-error' : undefined"
          />
        </div>

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
          :disabled="loading"
          class="mts-focus-ring w-full rounded-lg bg-slate-800 px-4 py-2 text-sm font-medium text-white hover:bg-slate-700 disabled:opacity-50 dark:bg-slate-100 dark:text-slate-900"
          data-testid="login-submit"
          :aria-busy="loading ? 'true' : undefined"
        >
          {{ loading ? t('loggingIn') : t('login') }}
        </button>
      </form>

      <div class="mt-5 rounded-lg bg-slate-50 p-3 text-xs leading-relaxed text-slate-500 dark:bg-slate-800/50 dark:text-slate-400">
        <p class="font-medium text-slate-600 dark:text-slate-300">{{ t('loginDefaultPolicy') }}</p>
        <p class="mt-1">{{ t('loginDefaultHint') }}</p>
      </div>
    </div>
  </div>
</template>
