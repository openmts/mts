<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useAuth } from '@/composables/useAuth'
import { useI18n } from '@/composables/useI18n'
import { useNotify } from '@/composables/useNotify'
import ActionResultBanner from '@/components/ActionResultBanner.vue'
import { validateNewPassword } from '@/utils/passwordPolicy'
import { KeyRound, UserRound } from 'lucide-vue-next'

const router = useRouter()
const { currentUser, currentRole, changePassword } = useAuth()
const { t, locale } = useI18n()
function roleLabel(role?: string | null): string {
  if (!role) return t.value('emptyValue')
  if (role === 'admin') return t.value('roleAdmin')
  if (role === 'user') return t.value('roleUser')
  return role
}
const { success } = useNotify()

const oldPassword = ref('')
const newPassword = ref('')
const confirmPassword = ref('')
const loading = ref(false)
const error = ref('')
const info = ref('')
const invalid = computed(() => !!error.value)

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
    <div>
      <h1 class="mts-title flex items-center gap-2">
        <UserRound class="h-5 w-5" aria-hidden="true" />
        {{ t('accountTitle') }}
      </h1>
      <p class="text-xs mts-muted">{{ t('accountDesc') }}</p>
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
