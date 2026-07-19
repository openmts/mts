<script setup lang="ts">
import NotifyHost from '@/components/NotifyHost.vue'
import ErrorBoundary from '@/components/ErrorBoundary.vue'
</script>

<template>
  <NotifyHost />
  <ErrorBoundary>
    <RouterView />
  </ErrorBoundary>
</template>
