<script setup lang="ts">
import { useRoute, useRouter } from 'vue-router'
import { useAuth } from '@/composables/useAuth'
import { useTheme } from '@/composables/useTheme'
import { useI18n } from '@/composables/useI18n'
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { Menu, Moon, Sun, Languages } from 'lucide-vue-next'
import { parseExpiresAt, sessionExpiryView } from '@/utils/sessionExpiry'

const route = useRoute()
const router = useRouter()
const { currentUser, currentRole, logout, loggingOut, getTokenExpiresAt } = useAuth()
const { theme, toggleTheme } = useTheme()
const { t, locale, toggleLocale } = useI18n()
const nowMs = ref(Date.now())
let timer: ReturnType<typeof setInterval> | null = null

defineEmits<{ 'toggle-sidebar': [] }>()

const pageTitle = computed(() => {
  const name = route.name as string | undefined
  const map: Record<string, string> = {
    Overview: t.value('overview'),
    Databases: t.value('databases'),
    Users: t.value('users'),
    Config: t.value('config'),
    Operations: t.value('operations'),
    Downsample: t.value('downsample'),
    Query: t.value('query'),
    Audit: t.value('audit'),
    ApiSpec: t.value('apiSpec'),
    Storage: t.value('storage'),
    Readiness: t.value('readiness'),
    About: t.value('about'),
    Write: t.value('write'),
    NotFound: '404',
  }
  return name ? (map[name] ?? name) : ''
})

const sessionView = computed(() => {
  const exp = parseExpiresAt(getTokenExpiresAt())
  return sessionExpiryView(exp, nowMs.value)
})

const sessionBadgeClass = computed(() => {
  switch (sessionView.value.urgency) {
    case 'critical':
      return 'bg-red-100 text-red-700 dark:bg-red-950/50 dark:text-red-200'
    case 'warn':
      return 'bg-amber-100 text-amber-800 dark:bg-amber-950/50 dark:text-amber-200'
    case 'expired':
      return 'bg-red-200 text-red-900 dark:bg-red-900 dark:text-red-100'
    default:
      return 'bg-slate-100 text-slate-600 dark:bg-slate-800 dark:text-slate-300'
  }
})

async function onLogout() {
  await logout()
  await router.replace({ name: 'Login' })
}

onMounted(() => {
  timer = setInterval(() => { nowMs.value = Date.now() }, 15_000)
})
onBeforeUnmount(() => {
  if (timer) clearInterval(timer)
})
</script>

<template>
  <header class="flex h-14 items-center justify-between gap-2 border-b border-slate-200 bg-white px-3 dark:border-slate-700 dark:bg-slate-900 sm:px-6">
    <div class="flex items-center gap-3">
      <button
        class="rounded p-1 text-slate-500 dark:text-slate-400 hover:bg-slate-100 hover:text-slate-700 dark:hover:bg-slate-800 dark:hover:text-slate-200 lg:hidden"
        @click="$emit('toggle-sidebar')"
      >
        <Menu class="h-5 w-5" />
      </button>
      <h1 class="max-w-[40vw] truncate text-base font-medium text-slate-800 dark:text-slate-100 sm:max-w-none sm:text-lg">{{ pageTitle }}</h1>
    </div>
    <div class="flex items-center gap-2 text-sm text-slate-600 dark:text-slate-300">
      <span
        v-if="sessionView.urgency === 'warn' || sessionView.urgency === 'critical' || sessionView.urgency === 'expired'"
        class="hidden rounded-full px-2 py-0.5 text-[11px] font-medium sm:inline"
        :class="sessionBadgeClass"
        :title="t('sessionExpiry')"
      >{{ t('sessionLeft') }} {{ sessionView.label }}</span>
      <button class="rounded p-1.5 hover:bg-slate-100 dark:hover:bg-slate-800" :title="t('lang')" @click="toggleLocale">
        <Languages class="h-4 w-4" />
        <span class="sr-only">{{ locale }}</span>
      </button>
      <button class="rounded p-1.5 hover:bg-slate-100 dark:hover:bg-slate-800" @click="toggleTheme">
        <Moon v-if="theme === 'light'" class="h-4 w-4" />
        <Sun v-else class="h-4 w-4" />
      </button>
      <span class="hidden sm:inline">{{ currentUser }}<span v-if="currentRole" class="text-slate-400 dark:text-slate-500"> · {{ currentRole }}</span></span>
      <button
        class="rounded px-2 py-1 text-slate-500 hover:bg-slate-100 hover:text-slate-700 disabled:opacity-50 dark:text-slate-400 dark:hover:bg-slate-800 dark:hover:text-slate-200"
        :disabled="loggingOut"
        @click="onLogout"
      >
        {{ loggingOut ? t('loggingOut') : t('logout') }}
      </button>
    </div>
  </header>
</template>
