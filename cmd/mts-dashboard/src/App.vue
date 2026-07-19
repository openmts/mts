<script setup lang="ts">
import NotifyHost from '@/components/NotifyHost.vue'
import ErrorBoundary from '@/components/ErrorBoundary.vue'
import GlobalProgress from '@/components/GlobalProgress.vue'
import { useDocumentTitle } from '@/composables/useDocumentTitle'

useDocumentTitle()
</script>

<template>
  <GlobalProgress />
  <NotifyHost />
  <ErrorBoundary>
    <RouterView />
  </ErrorBoundary>
</template>
