<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { useAuth } from '@/composables/useAuth'
import { KeyRound } from 'lucide-vue-next'

const router = useRouter()
const { currentUser, changePassword, logout } = useAuth()

const oldPassword = ref('')
const newPassword = ref('')
const confirmPassword = ref('')
const loading = ref(false)
const error = ref('')

async function submit() {
  error.value = ''
  if (!oldPassword.value || !newPassword.value) {
    error.value = '请填写旧密码与新密码'
    return
  }
  if (newPassword.value.length < 8) {
    error.value = '新密码至少 8 位'
    return
  }
  if (newPassword.value === 'admin') {
    error.value = '不能继续使用默认密码 admin'
    return
  }
  if (newPassword.value !== confirmPassword.value) {
    error.value = '两次输入的新密码不一致'
    return
  }
  if (newPassword.value === oldPassword.value) {
    error.value = '新密码不能与旧密码相同'
    return
  }
  loading.value = true
  try {
    const err = await changePassword(oldPassword.value, newPassword.value)
    if (err) {
      error.value = err
      return
    }
    await router.replace({ name: 'Login', query: { reason: 'password_changed' } })
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
    <div class="w-full max-w-md rounded-xl bg-white p-8 shadow-sm dark:border dark:border-slate-700 dark:bg-slate-900">
      <div class="mb-6 flex flex-col items-center gap-2 text-center">
        <KeyRound class="h-10 w-10 text-amber-600 dark:text-amber-300" />
        <h1 class="text-xl font-semibold text-slate-800 dark:text-slate-100">需要修改密码</h1>
        <p class="text-sm text-slate-500 dark:text-slate-400">
          账号 <span class="font-medium text-slate-700 dark:text-slate-200">{{ currentUser || 'admin' }}</span>
          仍在使用 bootstrap 默认密码。完成修改后需重新登录。
        </p>
      </div>

      <form class="space-y-4" @submit.prevent="submit">
        <div>
          <label class="mb-1 block text-sm font-medium text-slate-700 dark:text-slate-200" for="old">旧密码</label>
          <input id="old" v-model="oldPassword" type="password" autocomplete="current-password" class="mts-input" />
        </div>
        <div>
          <label class="mb-1 block text-sm font-medium text-slate-700 dark:text-slate-200" for="new">新密码</label>
          <input id="new" v-model="newPassword" type="password" autocomplete="new-password" class="mts-input" />
        </div>
        <div>
          <label class="mb-1 block text-sm font-medium text-slate-700 dark:text-slate-200" for="confirm">确认新密码</label>
          <input id="confirm" v-model="confirmPassword" type="password" autocomplete="new-password" class="mts-input" />
        </div>
        <p v-if="error" class="text-sm text-red-600 dark:text-red-300">{{ error }}</p>
        <button
          type="submit"
          class="w-full rounded-lg bg-slate-800 px-4 py-2 text-sm font-medium text-white hover:bg-slate-700 disabled:opacity-50 dark:bg-slate-100 dark:text-slate-900"
          :disabled="loading"
        >
          {{ loading ? '提交中...' : '修改密码并重新登录' }}
        </button>
      </form>

      <button type="button" class="mt-4 w-full text-center text-xs text-slate-500 hover:underline dark:text-slate-400" @click="doLogout">
        退出登录
      </button>
    </div>
  </div>
</template>
