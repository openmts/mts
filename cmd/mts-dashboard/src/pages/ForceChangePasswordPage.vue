<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useAuth } from '@/composables/useAuth'
import { useI18n } from '@/composables/useI18n'
import { validateNewPassword } from '@/utils/passwordPolicy'
import { buildLoginLocation, formatRedirectLabel, sanitizeRedirect, withRedirectQuery } from '@/utils/redirect'
import { Eye, EyeOff, KeyRound } from 'lucide-vue-next'
import PasswordHints from '@/components/PasswordHints.vue'
import { useNetworkStatus } from '@/composables/useNetworkStatus'
import { shouldBlockOfflineMutation } from '@/utils/offlineGuard'
import { registerDirtyChecker } from '@/utils/routeDirty'

const router = useRouter()
const route = useRoute()
const { currentUser, changePassword, logout } = useAuth()
const { t, locale } = useI18n()
const pendingRedirect = computed(() => sanitizeRedirect(route.query.redirect))
const pendingRedirectLabel = computed(() =>
  pendingRedirect.value ? formatRedirectLabel(pendingRedirect.value) : '',
)

const { offline } = useNetworkStatus()
const oldPassword = ref('')
const newPassword = ref('')
const confirmPassword = ref('')
const showOldPassword = ref(false)
const showNewPassword = ref(false)
const showConfirmPassword = ref(false)
const loading = ref(false)
const error = ref('')
const errorRetryable = ref(false)
const invalid = computed(() => !!error.value)
const passwordFormDirty = computed(
  () => !!(oldPassword.value || newPassword.value || confirmPassword.value),
)

function onForceBeforeUnload(e: BeforeUnloadEvent) {
  if (!passwordFormDirty.value) return
  e.preventDefault()
  e.returnValue = ''
}

let unregisterForceDirty: (() => void) | null = null

onMounted(() => {
  unregisterForceDirty = registerDirtyChecker('force-password', () => passwordFormDirty.value)
  window.addEventListener('beforeunload', onForceBeforeUnload)
})

onBeforeUnmount(() => {
  unregisterForceDirty?.()
  unregisterForceDirty = null
  window.removeEventListener('beforeunload', onForceBeforeUnload)
})

async function submit() {
  error.value = ''
  errorRetryable.value = false
  if (shouldBlockOfflineMutation(offline.value)) {
    error.value = t.value('offlineAccountBlocked')
    errorRetryable.value = true
    return
  }
  const check = validateNewPassword(oldPassword.value, newPassword.value, confirmPassword.value, {
    locale: locale.value,
  })
  if (!check.ok) {
    error.value = check.error || t.value('failed')
    errorRetryable.value = false
    return
  }
  loading.value = true
  try {
    const err = await changePassword(oldPassword.value, newPassword.value)
    if (err) {
      error.value = err
      errorRetryable.value = true
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
  await router.replace(buildLoginLocation({ redirectRaw: route.query.redirect }))
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
        <h1 class="text-xl font-semibold text-slate-800 dark:text-slate-100">
          {{ t('forcePasswordTitle') }}
          <span
            v-if="passwordFormDirty"
            data-testid="force-password-dirty-badge"
            class="ml-2 align-middle rounded-full bg-amber-100 px-2 py-0.5 text-[11px] font-medium text-amber-800 dark:bg-amber-950/50 dark:text-amber-200"
          >{{ t('accountPasswordDirtyBadge') }}</span>
        </h1>
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
          <div class="relative">
            <input
              id="old"
              v-model="oldPassword"
              :type="showOldPassword ? 'text' : 'password'"
              autocomplete="current-password"
              class="mts-input mts-focus-ring pr-10"
              data-testid="force-old"
              :aria-invalid="invalid ? 'true' : undefined"
              :aria-describedby="error ? 'force-password-error' : undefined"
            />
            <button
              type="button"
              class="mts-focus-ring absolute inset-y-0 right-1 my-auto inline-flex h-8 w-8 items-center justify-center rounded text-slate-400 hover:bg-slate-100 hover:text-slate-700 dark:hover:bg-slate-800 dark:hover:text-slate-200"
              data-testid="force-toggle-old"
              :aria-label="showOldPassword ? t('loginHidePassword') : t('loginShowPassword')"
              :title="showOldPassword ? t('loginHidePassword') : t('loginShowPassword')"
              :aria-pressed="showOldPassword ? 'true' : 'false'"
              @click="showOldPassword = !showOldPassword"
            >
              <EyeOff v-if="showOldPassword" class="h-4 w-4" aria-hidden="true" />
              <Eye v-else class="h-4 w-4" aria-hidden="true" />
            </button>
          </div>
        </div>
        <div>
          <label class="mb-1 block text-sm font-medium text-slate-700 dark:text-slate-200" for="new">{{ t('accountNewPassword') }}</label>
          <div class="relative">
            <input
              id="new"
              v-model="newPassword"
              :type="showNewPassword ? 'text' : 'password'"
              autocomplete="new-password"
              class="mts-input mts-focus-ring pr-10"
              data-testid="force-new"
              :aria-invalid="invalid ? 'true' : undefined"
              :aria-describedby="error ? 'force-password-error' : undefined"
            />
            <button
              type="button"
              class="mts-focus-ring absolute inset-y-0 right-1 my-auto inline-flex h-8 w-8 items-center justify-center rounded text-slate-400 hover:bg-slate-100 hover:text-slate-700 dark:hover:bg-slate-800 dark:hover:text-slate-200"
              data-testid="force-toggle-new"
              :aria-label="showNewPassword ? t('loginHidePassword') : t('loginShowPassword')"
              :title="showNewPassword ? t('loginHidePassword') : t('loginShowPassword')"
              :aria-pressed="showNewPassword ? 'true' : 'false'"
              @click="showNewPassword = !showNewPassword"
            >
              <EyeOff v-if="showNewPassword" class="h-4 w-4" aria-hidden="true" />
              <Eye v-else class="h-4 w-4" aria-hidden="true" />
            </button>
          </div>
        </div>
        <div>
          <label class="mb-1 block text-sm font-medium text-slate-700 dark:text-slate-200" for="confirm">{{ t('accountConfirmPassword') }}</label>
          <div class="relative">
            <input
              id="confirm"
              v-model="confirmPassword"
              :type="showConfirmPassword ? 'text' : 'password'"
              autocomplete="new-password"
              class="mts-input mts-focus-ring pr-10"
              data-testid="force-confirm"
              :aria-invalid="invalid ? 'true' : undefined"
              :aria-describedby="error ? 'force-password-error' : undefined"
            />
            <button
              type="button"
              class="mts-focus-ring absolute inset-y-0 right-1 my-auto inline-flex h-8 w-8 items-center justify-center rounded text-slate-400 hover:bg-slate-100 hover:text-slate-700 dark:hover:bg-slate-800 dark:hover:text-slate-200"
              data-testid="force-toggle-confirm"
              :aria-label="showConfirmPassword ? t('loginHidePassword') : t('loginShowPassword')"
              :title="showConfirmPassword ? t('loginHidePassword') : t('loginShowPassword')"
              :aria-pressed="showConfirmPassword ? 'true' : 'false'"
              @click="showConfirmPassword = !showConfirmPassword"
            >
              <EyeOff v-if="showConfirmPassword" class="h-4 w-4" aria-hidden="true" />
              <Eye v-else class="h-4 w-4" aria-hidden="true" />
            </button>
          </div>
        </div>
        <PasswordHints
          class="mt-1"
          :old-password="oldPassword"
          :new-password="newPassword"
          :confirm-password="confirmPassword"
        />
        <div
          v-if="error"
          id="force-password-error"
          class="rounded-lg border border-red-200 bg-red-50 px-3 py-2 dark:border-red-900 dark:bg-red-950/40"
          role="alert"
          aria-live="assertive"
          :aria-label="t('forcePasswordErrorRegion')"
          data-testid="force-password-error"
        >
          <p class="text-sm text-red-700 dark:text-red-200">{{ error }}</p>
          <div class="mt-2 flex flex-wrap gap-2">
            <button
              v-if="errorRetryable"
              type="button"
              class="mts-btn text-xs"
              data-testid="force-password-error-retry"
              :disabled="loading || offline"
              @click="submit"
            >{{ t('retry') }}</button>
            <button
              type="button"
              class="mts-btn text-xs"
              data-testid="force-password-error-dismiss"
              @click="error = ''; errorRetryable = false"
            >{{ t('dismiss') }}</button>
          </div>
        </div>
        <button
          type="submit"
          class="mts-focus-ring w-full rounded-lg bg-slate-800 px-4 py-2 text-sm font-medium text-white hover:bg-slate-700 disabled:opacity-50 dark:bg-slate-100 dark:text-slate-900"
          data-testid="force-password-submit"
          :disabled="loading || offline"
          :title="offline ? t('offlineAccountBlocked') : undefined"
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
