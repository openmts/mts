<script setup lang="ts">
/**
 * 分项/后台失败提示：默认 warn + 可重试，避免整页错误态覆盖已有数据。
 * 若文案命中 admin heavy 互斥，自动提供「打开运维」快捷动作。
 */
import { computed } from 'vue'
import ActionResultBanner from '@/components/ActionResultBanner.vue'
import { useI18n } from '@/composables/useI18n'
import { actionResultAdminBusyAction } from '@/utils/adminOpBusy'

const props = withDefaults(defineProps<{
  message: string
  testId?: string
  retryable?: boolean
  dismissible?: boolean
  /** 显式覆盖动作；默认按 message 识别 admin busy */
  actionLabel?: string
  actionPath?: string
}>(), {
  testId: 'partial-error-banner',
  retryable: true,
  dismissible: true,
  actionLabel: '',
  actionPath: '',
})

const emit = defineEmits<{ retry: []; dismiss: [] }>()
const { t } = useI18n()

const busyAction = computed(() =>
  actionResultAdminBusyAction({
    message: props.message,
    openLabel: t.value('adminOpBusyOpenOps'),
  }),
)
const resolvedActionLabel = computed(
  () => (props.actionLabel || '').trim() || busyAction.value?.label || '',
)
const resolvedActionPath = computed(
  () => (props.actionPath || '').trim() || busyAction.value?.path || '',
)
</script>

<template>
  <ActionResultBanner
    kind="warn"
    :message="message"
    :retryable="retryable"
    :dismissible="dismissible"
    :action-label="resolvedActionLabel"
    :action-path="resolvedActionPath"
    :data-testid="testId"
    @retry="emit('retry')"
    @dismiss="emit('dismiss')"
  />
</template>
