<script setup lang="ts">
/**
 * 分项/后台失败提示：默认 warn + 可重试，避免整页错误态覆盖已有数据。
 */
import ActionResultBanner from '@/components/ActionResultBanner.vue'

withDefaults(defineProps<{
  message: string
  testId?: string
  retryable?: boolean
  dismissible?: boolean
}>(), {
  testId: 'partial-error-banner',
  retryable: true,
  dismissible: true,
})

const emit = defineEmits<{ retry: []; dismiss: [] }>()
</script>

<template>
  <ActionResultBanner
    kind="warn"
    :message="message"
    :retryable="retryable"
    :dismissible="dismissible"
    :data-testid="testId"
    @retry="emit('retry')"
    @dismiss="emit('dismiss')"
  />
</template>
