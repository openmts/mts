<script setup lang="ts">
import { useRoute, useRouter } from 'vue-router'
import { useAuth } from '@/composables/useAuth'
import { useTheme } from '@/composables/useTheme'
import { useI18n } from '@/composables/useI18n'
import { computed } from 'vue'
import { Menu, Moon, Sun, Languages } from 'lucide-vue-next'

const route = useRoute()
const router = useRouter()
const { currentUser, currentRole, logout, loggingOut } = useAuth()
const { theme, toggleTheme } = useTheme()
const { t, locale, toggleLocale } = useI18n()

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
    Write: t.value('write'),
    NotFound: '404',
  }
  return name ? (map[name] ?? name) : ''
})

async function onLogout() {
  await logout()
  await router.replace({ name: 'Login' })
}
</script>

<template>
  <header class="flex h-14 items-center justify-between border-b border-slate-200 bg-white px-4 dark:border-slate-700 dark:bg-slate-900 sm:px-6">
    <div class="flex items-center gap-3">
      <button
        class="rounded p-1 text-slate-500 dark:text-slate-400 dark:text-slate-500 hover:bg-slate-100 dark:bg-slate-800 hover:text-slate-700 dark:text-slate-200 dark:hover:bg-slate-800 lg:hidden"
        @click="$emit('toggle-sidebar')"
      >
        <Menu class="h-5 w-5" />
      </button>
      <h1 class="text-lg font-medium text-slate-800 dark:text-slate-100">{{ pageTitle }}</h1>
    </div>
    <div class="flex items-center gap-2 text-sm text-slate-600 dark:text-slate-300">
      <button class="rounded p-1.5 hover:bg-slate-100 dark:bg-slate-800 dark:hover:bg-slate-800" :title="t('lang')" @click="toggleLocale">
        <Languages class="h-4 w-4" />
        <span class="sr-only">{{ locale }}</span>
      </button>
      <button class="rounded p-1.5 hover:bg-slate-100 dark:bg-slate-800 dark:hover:bg-slate-800" @click="toggleTheme">
        <Moon v-if="theme === 'light'" class="h-4 w-4" />
        <Sun v-else class="h-4 w-4" />
      </button>
      <span class="hidden sm:inline">{{ currentUser }}<span v-if="currentRole" class="text-slate-400 dark:text-slate-500"> · {{ currentRole }}</span></span>
      <button
        class="rounded px-2 py-1 text-slate-500 dark:text-slate-400 dark:text-slate-500 hover:bg-slate-100 dark:bg-slate-800 hover:text-slate-700 dark:text-slate-200 disabled:opacity-50 dark:hover:bg-slate-800"
        :disabled="loggingOut"
        @click="onLogout"
      >
        {{ loggingOut ? t('loggingOut') : t('logout') }}
      </button>
    </div>
  </header>
</template>
