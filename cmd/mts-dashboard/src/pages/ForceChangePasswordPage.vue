<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useAuth } from '@/composables/useAuth'
import { useI18n } from '@/composables/useI18n'
import { validateNewPassword } from '@/utils/passwordPolicy'
import { formatRedirectLabel, sanitizeRedirect, withRedirectQuery } from '@/utils/redirect'
import { KeyRound } from 'lucide-vue-next'
import PasswordHints from '@/components/PasswordHints.vue'

const router = useRouter()
const route = useRoute()
const { currentUser, changePassword, logout } = useAuth()
const { t, locale } = useI18n()
const pendingRedirect = computed(() => sanitizeRedirect(route.query.redirect))
const pendingRedirectLabel = computed(() =>
  pendingRedirect.value ? formatRedirectLabel(pendingRedirect.value) : '',
)

const oldPassword = ref('')
const newPassword = ref('')
const confirmPassword = ref('')
const loading = ref(false)
const error = ref('')
const invalid = computed(() => !!error.value)

async function submit() {
  error.value = ''
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
    await router.replace({
      name: 'Login',
      query: withRedirectQuery({ reason: 'password_changed' }, route.query.redirect),
    })
  } finally {
    loading.value = false
  }
}

async function doLogout() {
  await logout()
  await router.replace({ name: 'Login' })
}
</script>

<template>
  <div class="flex min-h-screen items-center justify-center bg-slate-100 p-4 dark:bg-slate-950">
    <div
      class="w-full max-w-md rounded-xl bg-white p-8 shadow-sm dark:border dark:border-slate-700 dark:bg-slate-900"
      data-testid="force-password-panel"
    >
      <div class="mb-6 flex flex-col items-center gap-2 text-center">
        <KeyRound class="h-10 w-10 text-amber-600 dark:text-amber-300" aria-hidden="true" />
        <h1 class="text-xl font-semibold text-slate-800 dark:text-slate-100">{{ t('forcePasswordTitle') }}</h1>
        <p class="text-sm text-slate-500 dark:text-slate-400">
          {{ t('forcePasswordDesc') }}
          <span class="font-medium text-slate-700 dark:text-slate-200">{{ currentUser || 'admin' }}</span>
        </p>
        <p
          v-if="pendingRedirect"
          class="mt-2 rounded-lg border border-slate-200 bg-slate-50 px-3 py-2 text-left text-xs text-slate-700 dark:border-slate-700 dark:bg-slate-800/60 dark:text-slate-200"
          role="status"
          data-testid="force-redirect-hint"
        >
          {{ t('forceRedirectHint') }}
          <span class="mt-0.5 block break-all font-mono text-[11px]" data-testid="force-redirect-path">{{ pendingRedirectLabel }}</span>
        </p>
      </div>

      <form class="space-y-4" data-testid="force-password-form" @submit.prevent="submit">
        <div>
          <label class="mb-1 block text-sm font-medium text-slate-700 dark:text-slate-200" for="old">{{ t('accountOldPassword') }}</label>
          <input
            id="old"
            v-model="oldPassword"
            type="password"
            autocomplete="current-password"
            class="mts-input mts-focus-ring"
            data-testid="force-old"
            :aria-invalid="invalid ? 'true' : undefined"
            :aria-describedby="error ? 'force-password-error' : undefined"
          />
        </div>
        <div>
          <label class="mb-1 block text-sm font-medium text-slate-700 dark:text-slate-200" for="new">{{ t('accountNewPassword') }}</label>
          <input
            id="new"
            v-model="newPassword"
            type="password"
            autocomplete="new-password"
            class="mts-input mts-focus-ring"
            data-testid="force-new"
            :aria-invalid="invalid ? 'true' : undefined"
            :aria-describedby="error ? 'force-password-error' : undefined"
          />
        </div>
        <div>
          <label class="mb-1 block text-sm font-medium text-slate-700 dark:text-slate-200" for="confirm">{{ t('accountConfirmPassword') }}</label>
          <input
            id="confirm"
            v-model="confirmPassword"
            type="password"
            autocomplete="new-password"
            class="mts-input mts-focus-ring"
            data-testid="force-confirm"
            :aria-invalid="invalid ? 'true' : undefined"
            :aria-describedby="error ? 'force-password-error' : undefined"
          />
        </div>
        <PasswordHints
          class="mt-1"
          :old-password="oldPassword"
          :new-password="newPassword"
          :confirm-password="confirmPassword"
        />
        <p
          v-if="error"
          id="force-password-error"
          class="text-sm text-red-600 dark:text-red-300"
          role="alert"
          aria-live="assertive"
          :aria-label="t('forcePasswordErrorRegion')"
          data-testid="force-password-error"
        >{{ error }}</p>
        <button
          type="submit"
          class="mts-focus-ring w-full rounded-lg bg-slate-800 px-4 py-2 text-sm font-medium text-white hover:bg-slate-700 disabled:opacity-50 dark:bg-slate-100 dark:text-slate-900"
          data-testid="force-password-submit"
          :disabled="loading"
          :aria-busy="loading ? 'true' : undefined"
        >
          {{ loading ? t('loading') : t('forcePasswordSubmit') }}
        </button>
      </form>

      <button
        type="button"
        class="mts-focus-ring mt-4 w-full text-center text-xs text-slate-500 hover:underline dark:text-slate-400"
        data-testid="force-password-logout"
        @click="doLogout"
      >
        {{ t('logout') }}
      </button>
    </div>
  </div>
</template>
