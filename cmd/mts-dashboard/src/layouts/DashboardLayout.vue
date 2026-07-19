<script setup lang="ts">
import { ref, provide } from 'vue'
import SidebarNav from '@/components/SidebarNav.vue'
import TopBar from '@/components/TopBar.vue'
import PageSkeleton from '@/components/PageSkeleton.vue'
import CommandPalette from '@/components/CommandPalette.vue'
import { useI18n } from '@/composables/useI18n'

const { t } = useI18n()
const sidebarOpen = ref(false)
const commandPaletteRef = ref<InstanceType<typeof CommandPalette> | null>(null)

function toggleSidebar() { sidebarOpen.value = !sidebarOpen.value }
function closeSidebar() { sidebarOpen.value = false }
function openCommandPalette() { commandPaletteRef.value?.openPalette() }

provide('toggleSidebar', toggleSidebar)
provide('closeSidebar', closeSidebar)
provide('openCommandPalette', openCommandPalette)
</script>

<template>
  <div class="flex h-screen bg-slate-50 dark:bg-slate-950">
    <a
      href="#main-content"
      class="sr-only focus:not-sr-only focus:absolute focus:left-3 focus:top-3 focus:z-[200] focus:rounded-lg focus:bg-white focus:px-3 focus:py-2 focus:text-sm focus:font-medium focus:text-slate-900 focus:shadow dark:focus:bg-slate-800 dark:focus:text-slate-100"
      data-testid="skip-to-main"
    >
      {{ t('skipToMain') }}
    </a>
    <SidebarNav :visible="sidebarOpen" @close="closeSidebar" />
    <div class="flex flex-1 flex-col overflow-hidden">
      <TopBar @toggle-sidebar="toggleSidebar" @open-command-palette="openCommandPalette" />
      <main id="main-content" class="flex-1 overflow-auto p-4 sm:p-6" tabindex="-1">
        <RouterView v-slot="{ Component }">
          <Suspense>
            <component :is="Component" />
            <template #fallback>
              <PageSkeleton />
            </template>
          </Suspense>
        </RouterView>
      </main>
    </div>
    <CommandPalette ref="commandPaletteRef" />
  </div>
</template>
