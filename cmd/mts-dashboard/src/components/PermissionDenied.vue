<script setup lang="ts">
import EmptyState from '@/components/EmptyState.vue'
import { useI18n } from '@/composables/useI18n'

const { t } = useI18n()
</script>

<template>
  <EmptyState
    :title="t('permissionDenied')"
    description="当前账号缺少此页面所需的管理权限。请使用管理员登录，或联系管理员授权后重试。"
  >
    <template #icon>
      <span class="text-lg" aria-hidden="true">⛔</span>
    </template>
  </EmptyState>
</template>
