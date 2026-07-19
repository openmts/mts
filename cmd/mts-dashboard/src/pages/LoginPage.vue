<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { useAuth } from '@/composables/useAuth'
import { useI18n } from '@/composables/useI18n'
import { sanitizeRedirect } from '@/router'
import { Server } from 'lucide-vue-next'

const router = useRouter()
const { login } = useAuth()
const { t } = useI18n()

const username = ref('admin')
const password = ref('')
const loading = ref(false)
const error = ref('')

async function handleLogin() {
  if (!username.value.trim() || !password.value) {
    error.value = t.value('loginNeedCredentials')
    return
  }
  error.value = ''
  loading.value = true
  try {
    const err = await login(username.value.trim(), password.value)
    if (err) {
      error.value = err
    } else {
      const redirect = sanitizeRedirect(router.currentRoute.value.query.redirect) || '/'
      await router.push(redirect)
    }
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="flex min-h-screen items-center justify-center bg-slate-100 dark:bg-slate-950">
    <div class="w-full max-w-sm rounded-xl bg-white p-8 shadow-sm dark:bg-slate-900 dark:border dark:border-slate-700">
      <div class="mb-6 flex flex-col items-center gap-2">
        <Server class="h-10 w-10 text-slate-700 dark:text-slate-200" />
        <h1 class="text-xl font-semibold text-slate-800 dark:text-slate-100">{{ t('loginTitle') }}</h1>
        <p class="text-sm text-slate-500 dark:text-slate-400">{{ t('loginSubtitle') }}</p>
      </div>

      <form class="space-y-4" @submit.prevent="handleLogin">
        <div>
          <label class="mb-1 block text-sm font-medium text-slate-700 dark:text-slate-200" for="username">{{ t('username') }}</label>
          <input
            id="username"
            v-model="username"
            type="text"
            autocomplete="username"
            class="mts-input"
            :placeholder="t('loginUsernamePlaceholder')"
          />
        </div>
        <div>
          <label class="mb-1 block text-sm font-medium text-slate-700 dark:text-slate-200" for="password">{{ t('password') }}</label>
          <input
            id="password"
            v-model="password"
            type="password"
            autocomplete="current-password"
            class="mts-input"
            :placeholder="t('loginPasswordPlaceholder')"
          />
        </div>

        <p v-if="error" class="text-sm text-red-600 dark:text-red-300">{{ error }}</p>

        <button
          type="submit"
          :disabled="loading"
          class="w-full rounded-lg bg-slate-800 px-4 py-2 text-sm font-medium text-white hover:bg-slate-700 disabled:opacity-50 dark:bg-slate-100 dark:text-slate-900"
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
