<script setup lang="ts">
import { useI18n } from '@/composables/useI18n'
const { t } = useI18n()
</script>
<template>
  <div class="rounded-xl border border-amber-200 bg-amber-50 p-6 text-sm text-amber-900 dark:border-amber-900 dark:bg-amber-950 dark:text-amber-100">
    {{ t('permissionDenied') }}
  </div>
</template>
