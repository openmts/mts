<script setup lang="ts">
/**
 * 页头「最近一次管理重操作」芯片 + 失败时错误明细（可商用运维可见性）。
 * 多根节点：芯片与错误行作为 flex 子项，便于塞进现有 flex-wrap 标题区。
 */
import { computed } from 'vue'
import { useI18n } from '@/composables/useI18n'
import { adminOpLastChipSurfaceClass } from '@/utils/adminOpBusy'

const props = withDefaults(
  defineProps<{
    label: string
    lastOk?: boolean | null
    lastError?: string | null
    testId?: string
    errorTestId?: string
    /** 失败时是否展示 error 明细行，默认 true */
    showError?: boolean
  }>(),
  {
    lastOk: null,
    lastError: '',
    testId: 'admin-last',
    errorTestId: 'admin-last-error',
    showError: true,
  },
)

const { t } = useI18n()

const surfaceClass = computed(() => adminOpLastChipSurfaceClass(props.lastOk))
const dataOk = computed(() =>
  props.lastOk === true ? 'true' : props.lastOk === false ? 'false' : undefined,
)
const errorDetail = computed(() => {
  if (!props.showError || props.lastOk !== false) return ''
  return String(props.lastError || '').trim()
})
</script>

<template>
  <span
    :class="surfaceClass"
    :data-testid="testId"
    :data-ok="dataOk"
    :title="label"
  >{{ t('opsStatusLastLabel') }}: {{ label }}</span>
  <span
    v-if="errorDetail"
    class="max-w-full break-all font-mono text-[11px] text-red-700 dark:text-red-300"
    :data-testid="errorTestId"
  >{{ t('adminOpLastErrorLabel') }}: {{ errorDetail }}</span>
</template>
