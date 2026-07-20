<script setup lang="ts">
import NotifyHost from '@/components/NotifyHost.vue'
import ErrorBoundary from '@/components/ErrorBoundary.vue'
import GlobalProgress from '@/components/GlobalProgress.vue'
import { useDocumentTitle } from '@/composables/useDocumentTitle'
import { useDensity } from '@/composables/useDensity'

useDocumentTitle()
// 挂载时应用 data-density
useDensity()
</script>

<template>
  <GlobalProgress />
  <NotifyHost />
  <ErrorBoundary>
    <RouterView />
  </ErrorBoundary>
</template>
