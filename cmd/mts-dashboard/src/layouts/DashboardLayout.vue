<script setup lang="ts">
import { ref, provide } from 'vue'
import SidebarNav from '@/components/SidebarNav.vue'
import TopBar from '@/components/TopBar.vue'
import PageSkeleton from '@/components/PageSkeleton.vue'
import CommandPalette from '@/components/CommandPalette.vue'

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
    <SidebarNav :visible="sidebarOpen" @close="closeSidebar" />
    <div class="flex flex-1 flex-col overflow-hidden">
      <TopBar @toggle-sidebar="toggleSidebar" @open-command-palette="openCommandPalette" />
      <main class="flex-1 overflow-auto p-4 sm:p-6">
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
