<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useAuth } from '@/composables/useAuth'
import { useI18n } from '@/composables/useI18n'
import { validateNewPassword } from '@/utils/passwordPolicy'
import { bootstrapPasswordPolicy } from '@/utils/passwordPolicyBootstrap'
import { passwordRequirementHints, passwordHintsAllOk, passwordHintsProgress } from '@/utils/passwordHints'
import { buildLoginLocation, formatRedirectLabel, sanitizeRedirect, withRedirectQuery } from '@/utils/redirect'
import { KeyRound } from 'lucide-vue-next'
import PasswordHints from '@/components/PasswordHints.vue'
import PasswordInputWithToggle from '@/components/PasswordInputWithToggle.vue'
import { useNetworkStatus } from '@/composables/useNetworkStatus'
import { shouldBlockOfflineMutation } from '@/utils/offlineGuard'
import { registerDirtyChecker } from '@/utils/routeDirty'
import { createActionAbort } from '@/utils/actionAbort'
import InFlightBanner from '@/components/InFlightBanner.vue'
import { isCanceledError, isTimeoutError } from '@/utils/apiError'

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
const passwordHintsView = computed(() =>
  passwordRequirementHints(oldPassword.value, newPassword.value, confirmPassword.value),
)
const passwordHintsReady = computed(() => passwordHintsAllOk(passwordHintsView.value))
const passwordHintsProgressView = computed(() => passwordHintsProgress(passwordHintsView.value))
const loading = ref(false)
const error = ref('')
const errorRetryable = ref(false)
const actionStartedAt = ref<number | null>(null)
const actionAbort = createActionAbort()
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
  void bootstrapPasswordPolicy()
  unregisterForceDirty = registerDirtyChecker('force-password', () => passwordFormDirty.value)
  window.addEventListener('beforeunload', onForceBeforeUnload)
})

onBeforeUnmount(() => {
  actionAbort.cancel()
  unregisterForceDirty?.()
  unregisterForceDirty = null
  window.removeEventListener('beforeunload', onForceBeforeUnload)
})

function cancelForceAction() {
  actionAbort.cancel()
}

async function submit() {
  if (loading.value) return
  error.value = ''
  errorRetryable.value = false
  if (shouldBlockOfflineMutation(offline.value)) {
    error.value = t.value('offlineAccountBlocked')
    errorRetryable.value = true
    return
  }
  if (!passwordHintsReady.value) {
    error.value = t.value('forcePasswordPolicyGate')
    errorRetryable.value = false
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
  actionStartedAt.value = Date.now()
  const signal = actionAbort.begin()
  try {
    const userForRelogin = currentUser.value || ''
    const err = await changePassword(oldPassword.value, newPassword.value, { signal })
    if (err) {
      if (isCanceledError(err)) {
        error.value = t.value('adminActionCancelled')
        errorRetryable.value = false
        return
      }
      if (isTimeoutError(err)) {
        error.value = t.value('adminActionTimedOut')
        errorRetryable.value = true
        return
      }
      error.value = err
      // 旧密码错误等校验失败可立即重试；网络类同样可重试
      errorRetryable.value = true
      return
    }
    // 成功后清脏表单，避免路由守卫拦截跳转登录
    oldPassword.value = ''
    newPassword.value = ''
    confirmPassword.value = ''
    error.value = ''
    errorRetryable.value = false
    await router.replace({
      name: 'Login',
      query: withRedirectQuery(
        { reason: 'password_changed', ...(userForRelogin ? { user: userForRelogin } : {}) },
        route.query.redirect,
      ),
    })
  } finally {
    actionAbort.end()
    loading.value = false
    actionStartedAt.value = null
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

      <InFlightBanner
        :active="loading"
        :started-at-ms="actionStartedAt"
        kind="admin"
        @cancel="cancelForceAction"
      />
      <form class="space-y-4" data-testid="force-password-form" @submit.prevent="submit">
        <div>
          <label class="mb-1 block text-sm font-medium text-slate-700 dark:text-slate-200" for="old">{{ t('accountOldPassword') }}</label>
          <PasswordInputWithToggle
            id="old"
            v-model="oldPassword"
            autocomplete="current-password"
            test-id="force-old"
            toggle-test-id="force-toggle-old"
            :invalid="invalid"
            :described-by="error ? 'force-password-error' : undefined"
          />
        </div>
        <div>
          <label class="mb-1 block text-sm font-medium text-slate-700 dark:text-slate-200" for="new">{{ t('accountNewPassword') }}</label>
          <PasswordInputWithToggle
            id="new"
            v-model="newPassword"
            autocomplete="new-password"
            test-id="force-new"
            toggle-test-id="force-toggle-new"
            :invalid="invalid"
            :described-by="error ? 'force-password-error' : undefined"
          />
        </div>
        <div>
          <label class="mb-1 block text-sm font-medium text-slate-700 dark:text-slate-200" for="confirm">{{ t('accountConfirmPassword') }}</label>
          <PasswordInputWithToggle
            id="confirm"
            v-model="confirmPassword"
            autocomplete="new-password"
            test-id="force-confirm"
            toggle-test-id="force-toggle-confirm"
            :invalid="invalid"
            :described-by="error ? 'force-password-error' : undefined"
          />
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
        <p
          v-if="!passwordHintsReady && (newPassword || confirmPassword || oldPassword)"
          class="text-[11px] text-amber-700 dark:text-amber-200"
          data-testid="force-password-policy-gate"
        >
          {{ t('forcePasswordPolicyGate') }}
          <span class="ml-1 font-mono" data-testid="force-password-progress-text">
            {{ passwordHintsProgressView.done }}/{{ passwordHintsProgressView.total }}
          </span>
        </p>
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
