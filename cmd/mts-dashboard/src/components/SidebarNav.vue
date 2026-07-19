<script setup lang="ts">
import { computed, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useAuth } from '@/composables/useAuth'
import {
  LayoutDashboard, Database, Users, Settings, Wrench,
  ArrowDownUp, Search, Send, ScrollText, HardDrive, X,
} from 'lucide-vue-next'

defineProps<{ visible: boolean }>()
const emit = defineEmits<{ close: [] }>()

const route = useRoute()
const router = useRouter()
const { isAdmin } = useAuth()

const allNavItems = [
  { to: '/', label: '仪表盘', icon: LayoutDashboard, adminOnly: false },
  { to: '/databases', label: '数据库', icon: Database, adminOnly: true },
  { to: '/query', label: '查询', icon: Search, adminOnly: false },
  { to: '/write', label: '写入', icon: Send, adminOnly: false },
  { to: '/users', label: '用户', icon: Users, adminOnly: false },
  { to: '/config', label: '配置', icon: Settings, adminOnly: true },
  { to: '/operations', label: '运维', icon: Wrench, adminOnly: true },
  { to: '/downsample', label: '降采样', icon: ArrowDownUp, adminOnly: true },
  { to: '/audit', label: '审计', icon: ScrollText, adminOnly: true },
  { to: '/storage', label: '存储', icon: HardDrive, adminOnly: true },
]

const navItems = computed(() => allNavItems.filter((i) => !i.adminOnly || isAdmin.value))

function isActive(path: string): boolean {
  if (path === '/') return route.path === '/'
  return route.path.startsWith(path)
}

function navigate(to: string) {
  void router.push(to)
  emit('close')
}

watch(() => route.path, () => {
  emit('close')
})
</script>

<template>
  <div
    v-if="visible"
    class="fixed inset-0 z-40 bg-black/30 lg:hidden"
    @click="emit('close')"
  />

  <aside
    :class="[
      'fixed inset-y-0 left-0 z-50 flex w-56 flex-col border-r border-slate-200 bg-white transition-transform duration-200',
      'lg:relative lg:z-0 lg:translate-x-0',
      visible ? 'translate-x-0' : '-translate-x-full',
    ]"
  >
    <div class="flex h-14 items-center justify-between border-b border-slate-200 px-4">
      <span class="text-lg font-semibold text-slate-800">MTS Dashboard</span>
      <button class="rounded p-1 text-slate-400 hover:text-slate-600 lg:hidden" @click="emit('close')">
        <X class="h-5 w-5" />
      </button>
    </div>
    <nav class="flex-1 space-y-1 overflow-auto p-3">
      <button
        v-for="item in navItems"
        :key="item.to"
        :class="[
          'flex w-full items-center gap-3 rounded-lg px-3 py-2 text-left text-sm transition-colors',
          isActive(item.to)
            ? 'bg-slate-100 font-medium text-slate-900'
            : 'text-slate-600 hover:bg-slate-50 hover:text-slate-900',
        ]"
        @click="navigate(item.to)"
      >
        <component :is="item.icon" class="h-4 w-4 shrink-0" />
        {{ item.label }}
      </button>
    </nav>
  </aside>
</template>
