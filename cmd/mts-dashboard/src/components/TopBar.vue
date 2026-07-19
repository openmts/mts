<script setup lang="ts">
import { useRoute, useRouter } from 'vue-router'
import { useAuth } from '@/composables/useAuth'
import { computed } from 'vue'
import { Menu } from 'lucide-vue-next'

const route = useRoute()
const router = useRouter()
const { currentUser, currentRole, logout, loggingOut } = useAuth()

defineEmits<{ 'toggle-sidebar': [] }>()

const routeLabels: Record<string, string> = {
  Overview: '仪表盘',
  Databases: '数据库管理',
  Users: '用户管理',
  Config: '配置管理',
  Operations: '运维操作',
  Downsample: '降采样管理',
  Query: '数据查询',
  Audit: '审计日志',
  Storage: '存储快照',
  Write: '写入管理',
  NotFound: '未找到',
}

const pageTitle = computed(() => {
  const name = route.name as string | undefined
  return name ? (routeLabels[name] ?? name) : ''
})

async function onLogout() {
  await logout()
  await router.replace({ name: 'Login' })
}
</script>

<template>
  <header class="flex h-14 items-center justify-between border-b border-slate-200 bg-white px-4 sm:px-6">
    <div class="flex items-center gap-3">
      <button
        class="rounded p-1 text-slate-500 hover:bg-slate-100 hover:text-slate-700 lg:hidden"
        @click="$emit('toggle-sidebar')"
      >
        <Menu class="h-5 w-5" />
      </button>
      <h1 class="text-lg font-medium text-slate-800">{{ pageTitle }}</h1>
    </div>
    <div class="flex items-center gap-3 text-sm text-slate-600">
      <span class="hidden sm:inline">{{ currentUser }}<span v-if="currentRole" class="text-slate-400"> · {{ currentRole }}</span></span>
      <button
        class="rounded px-2 py-1 text-slate-500 hover:bg-slate-100 hover:text-slate-700 disabled:opacity-50"
        :disabled="loggingOut"
        @click="onLogout"
      >
        {{ loggingOut ? '退出中...' : '退出' }}
      </button>
    </div>
  </header>
</template>
