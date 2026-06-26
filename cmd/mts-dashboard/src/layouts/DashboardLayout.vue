<script setup lang="ts">
import { ref, provide } from 'vue'
import SidebarNav from '@/components/SidebarNav.vue'
import TopBar from '@/components/TopBar.vue'

const sidebarOpen = ref(false)

function toggleSidebar() {
  sidebarOpen.value = !sidebarOpen.value
}

function closeSidebar() {
  sidebarOpen.value = false
}

provide('toggleSidebar', toggleSidebar)
provide('closeSidebar', closeSidebar)
</script>

<template>
  <div class="flex h-screen bg-slate-50">
    <SidebarNav :visible="sidebarOpen" @close="closeSidebar" />
    <div class="flex flex-1 flex-col overflow-hidden">
      <TopBar @toggle-sidebar="toggleSidebar" />
      <main class="flex-1 overflow-auto p-4 sm:p-6">
        <RouterView />
      </main>
    </div>
  </div>
</template>
