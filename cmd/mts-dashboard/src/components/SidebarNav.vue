<script setup lang="ts">
import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useAuth } from '@/composables/useAuth'
import { useI18n } from '@/composables/useI18n'
import {
  LayoutDashboard, Database, Users, Settings, Wrench,
  ArrowDownUp, Search, ScrollText, HardDrive, Send, X, BookOpen, Shield, ShieldCheck, Activity, ClipboardCheck,
} from 'lucide-vue-next'

defineProps<{ visible: boolean }>()
const emit = defineEmits<{ close: [] }>()
const route = useRoute()
const router = useRouter()
const { isAdmin } = useAuth()
const { t } = useI18n()

const allNavItems = computed(() => [
  { to: '/', label: t.value('overview'), icon: LayoutDashboard, adminOnly: false },
  { to: '/databases', label: t.value('databases'), icon: Database, adminOnly: true },
  { to: '/query', label: t.value('query'), icon: Search, adminOnly: false },
  { to: '/write', label: t.value('write'), icon: Send, adminOnly: false },
  { to: '/users', label: t.value('users'), icon: Users, adminOnly: false },
  { to: '/access', label: t.value('accessMatrix'), icon: Shield, adminOnly: false },
  { to: '/access/grants', label: t.value('accessGrants'), icon: ShieldCheck, adminOnly: true },
  { to: '/observability/metrics', label: t.value('metrics'), icon: Activity, adminOnly: true },
  { to: '/config', label: t.value('config'), icon: Settings, adminOnly: true },
  { to: '/operations', label: t.value('operations'), icon: Wrench, adminOnly: true },
  { to: '/downsample', label: t.value('downsample'), icon: ArrowDownUp, adminOnly: true },
  { to: '/audit', label: t.value('audit'), icon: ScrollText, adminOnly: true },
  { to: '/api-spec', label: t.value('apiSpec'), icon: BookOpen, adminOnly: true },
  { to: '/storage', label: t.value('storage'), icon: HardDrive, adminOnly: true },
  { to: '/ops/readiness', label: t.value('readiness'), icon: ClipboardCheck, adminOnly: true },
])

const navItems = computed(() => allNavItems.value.filter((i) => !i.adminOnly || isAdmin.value))

function isActive(to: string) {
  if (to === '/') return route.path === '/'
  return route.path === to || route.path.startsWith(to + '/')
}

function go(to: string) {
  void router.push(to)
  emit('close')
}
</script>

<template>
  <div>
    <div
      class="fixed inset-0 z-40 bg-black/30 lg:hidden"
      :class="visible ? 'block' : 'hidden'"
      @click="emit('close')"
    />
    <aside
      class="fixed inset-y-0 left-0 z-50 flex w-60 flex-col border-r border-slate-200 bg-white transition-transform dark:border-slate-700 dark:bg-slate-900 lg:static lg:translate-x-0"
      :class="visible ? 'translate-x-0' : '-translate-x-full'"
    >
      <div class="flex h-14 items-center justify-between border-b border-slate-200 px-4 dark:border-slate-700">
        <span class="text-sm font-semibold text-slate-800 dark:text-slate-100">{{ t('appName') }}</span>
        <button class="rounded p-1 text-slate-400 dark:text-slate-500 hover:bg-slate-100 dark:bg-slate-800 dark:hover:bg-slate-800 lg:hidden" @click="emit('close')">
          <X class="h-4 w-4" />
        </button>
      </div>
      <nav class="flex-1 space-y-0.5 overflow-auto p-2">
        <button
          v-for="item in navItems"
          :key="item.to"
          class="flex w-full items-center gap-2 rounded-lg px-3 py-2 text-left text-sm"
          :class="isActive(item.to)
            ? 'bg-slate-900 text-white dark:bg-slate-100 dark:bg-slate-800 dark:text-slate-900'
            : 'text-slate-600 dark:text-slate-300 hover:bg-slate-100 dark:bg-slate-800 dark:text-slate-300 dark:hover:bg-slate-800'"
          @click="go(item.to)"
        >
          <component :is="item.icon" class="h-4 w-4 shrink-0" />
          <span>{{ item.label }}</span>
        </button>
      </nav>
    </aside>
  </div>
</template>
