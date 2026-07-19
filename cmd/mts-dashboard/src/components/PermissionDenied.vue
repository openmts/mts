<script setup lang="ts">
import EmptyState from '@/components/EmptyState.vue'
import { useI18n } from '@/composables/useI18n'

const { t } = useI18n()
</script>

<template>
  <EmptyState
    :title="t('permissionDenied')"
    :description="t('permissionDeniedDesc')"
  >
    <template #icon>
      <span class="text-lg" aria-hidden="true">⛔</span>
    </template>
  </EmptyState>
</template>
