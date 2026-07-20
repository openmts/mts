<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useAuth } from '@/composables/useAuth'
import { useI18n } from '@/composables/useI18n'
import {
  LayoutDashboard, Database, Users, Settings, Wrench,
  ArrowDownUp, Search, ScrollText, HardDrive, Send, X, BookOpen, Shield, ShieldCheck, Activity, ClipboardCheck, Info, UserRound, PanelLeftClose, PanelLeftOpen,
} from 'lucide-vue-next'
import { filterNavItems } from '@/utils/navFilter'

const props = defineProps<{ visible: boolean; collapsed: boolean }>()
const emit = defineEmits<{ close: []; 'toggle-collapse': [] }>()
const route = useRoute()
const router = useRouter()
const { isAdmin } = useAuth()
const { t } = useI18n()
const navFilter = ref('')

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
  { to: '/about', label: t.value('about'), icon: Info, adminOnly: false },
  { to: '/account', label: t.value('account'), icon: UserRound, adminOnly: false },
])

const roleNavItems = computed(() => allNavItems.value.filter((i) => !i.adminOnly || isAdmin.value))

const navItems = computed(() => {
  if (props.collapsed) return roleNavItems.value
  return filterNavItems(roleNavItems.value, navFilter.value)
})

watch(
  () => props.collapsed,
  (c) => {
    if (c) navFilter.value = ''
  },
)

function isActive(to: string) {
  if (to === '/') return route.path === '/'
  return route.path === to || route.path.startsWith(to + '/')
}

function navTestId(to: string): string {
  if (to === '/') return 'sidebar-nav-home'
  const slug = to.startsWith('/') ? to.slice(1) : to
  return `sidebar-nav-${slug.split('/').join('-')}`
}

function go(to: string) {
  void router.push(to)
  emit('close')
}

function clearFilter() {
  navFilter.value = ''
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
      class="fixed inset-y-0 left-0 z-50 flex flex-col border-r border-slate-200 bg-white transition-[width,transform] dark:border-slate-700 dark:bg-slate-900 lg:static lg:translate-x-0"
      :class="[
        visible ? 'translate-x-0' : '-translate-x-full',
        collapsed ? 'w-16' : 'w-60',
      ]"
      data-testid="sidebar"
      :data-collapsed="collapsed ? 'true' : 'false'"
    >
      <div
        class="flex h-14 items-center border-b border-slate-200 dark:border-slate-700"
        :class="collapsed ? 'justify-center px-2' : 'justify-between px-4'"
      >
        <span
          class="truncate text-sm font-semibold text-slate-800 dark:text-slate-100"
          :class="collapsed ? 'sr-only' : ''"
        >{{ t('appName') }}</span>
        <button
          type="button"
          class="mts-focus-ring rounded p-1 text-slate-400 hover:bg-slate-100 dark:text-slate-500 dark:hover:bg-slate-800 lg:hidden"
          :aria-label="t('topbarCloseNav')"
          :title="t('topbarCloseNav')"
          data-testid="sidebar-close"
          @click="emit('close')"
        >
          <X class="h-4 w-4" aria-hidden="true" />
        </button>
        <button
          type="button"
          class="mts-focus-ring hidden rounded p-1 text-slate-400 hover:bg-slate-100 dark:text-slate-500 dark:hover:bg-slate-800 lg:inline-flex"
          :aria-label="collapsed ? t('sidebarExpand') : t('sidebarCollapse')"
          :title="collapsed ? t('sidebarExpand') : t('sidebarCollapse')"
          data-testid="sidebar-collapse-toggle"
          @click="emit('toggle-collapse')"
        >
          <PanelLeftOpen v-if="collapsed" class="h-4 w-4" aria-hidden="true" />
          <PanelLeftClose v-else class="h-4 w-4" aria-hidden="true" />
        </button>
      </div>
      <div
        v-if="!collapsed"
        class="border-b border-slate-200 p-2 dark:border-slate-700"
        data-testid="sidebar-filter-wrap"
      >
        <label class="sr-only" for="sidebar-filter-input">{{ t('sidebarFilterPlaceholder') }}</label>
        <div class="relative">
          <input
            id="sidebar-filter-input"
            v-model="navFilter"
            type="search"
            class="mts-focus-ring w-full rounded-lg border border-slate-200 bg-slate-50 py-1.5 pl-2 pr-7 text-xs text-slate-800 placeholder:text-slate-400 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100 dark:placeholder:text-slate-500"
            :placeholder="t('sidebarFilterPlaceholder')"
            data-testid="sidebar-filter"
            autocomplete="off"
          />
          <button
            v-if="navFilter"
            type="button"
            class="mts-focus-ring absolute right-1 top-1/2 -translate-y-1/2 rounded p-0.5 text-slate-400 hover:text-slate-600 dark:hover:text-slate-200"
            :aria-label="t('sidebarFilterClear')"
            :title="t('sidebarFilterClear')"
            data-testid="sidebar-filter-clear"
            @click="clearFilter"
          >
            <X class="h-3.5 w-3.5" aria-hidden="true" />
          </button>
        </div>
      </div>
      <nav class="flex-1 space-y-0.5 overflow-auto p-2" :aria-label="t('appName')" data-testid="sidebar-nav">
        <p
          v-if="!collapsed && navFilter.trim() && navItems.length === 0"
          class="px-2 py-3 text-center text-xs text-slate-400 dark:text-slate-500"
          data-testid="sidebar-filter-empty"
        >
          {{ t('sidebarFilterEmpty') }}
        </p>
        <button
          v-for="item in navItems"
          :key="item.to"
          type="button"
          class="flex w-full items-center gap-2 rounded-lg py-2 text-left text-sm"
          :class="[
            isActive(item.to)
              ? 'bg-slate-900 text-white dark:bg-slate-100 dark:text-slate-900'
              : 'text-slate-600 hover:bg-slate-100 dark:text-slate-300 dark:hover:bg-slate-800',
            collapsed ? 'justify-center px-2' : 'px-3',
          ]"
          :aria-current="isActive(item.to) ? 'page' : undefined"
          :title="item.label"
          :aria-label="item.label"
          :data-testid="navTestId(item.to)"
          @click="go(item.to)"
        >
          <component :is="item.icon" class="h-4 w-4 shrink-0" aria-hidden="true" />
          <span :class="collapsed ? 'sr-only' : ''">{{ item.label }}</span>
        </button>
      </nav>
    </aside>
  </div>
</template>
