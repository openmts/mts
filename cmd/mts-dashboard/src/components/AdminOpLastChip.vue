<script setup lang="ts">
/**
 * 页头「最近一次管理重操作」芯片 + 失败时错误明细（可商用运维可见性）。
 * 多根节点：芯片与错误行作为 flex 子项，便于塞进现有 flex-wrap 标题区。
 * 默认点击芯片跳转运维状态条，便于从任意页回到运维上下文。
 */
import { computed } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from '@/composables/useI18n'
import { ADMIN_OP_BUSY_OPS_PATH, adminOpLastChipSurfaceClass } from '@/utils/adminOpBusy'

const props = withDefaults(
  defineProps<{
    label: string
    lastOk?: boolean | null
    lastError?: string | null
    testId?: string
    errorTestId?: string
    /** 失败时是否展示 error 明细行，默认 true */
    showError?: boolean
    /** 点击芯片跳转运维（默认 true） */
    linkToOps?: boolean
  }>(),
  {
    lastOk: null,
    lastError: '',
    testId: 'admin-last',
    errorTestId: 'admin-last-error',
    showError: true,
    linkToOps: true,
  },
)

const { t } = useI18n()
const router = useRouter()

const surfaceClass = computed(() => {
  const base = adminOpLastChipSurfaceClass(props.lastOk)
  return props.linkToOps
    ? `${base} cursor-pointer hover:opacity-90 focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-1 focus-visible:outline-sky-500`
    : base
})
const dataOk = computed(() =>
  props.lastOk === true ? 'true' : props.lastOk === false ? 'false' : undefined,
)
const errorDetail = computed(() => {
  if (!props.showError || props.lastOk !== false) return ''
  return String(props.lastError || '').trim()
})
const chipTitle = computed(() => {
  const open = props.linkToOps ? t.value('adminOpBusyOpenOps') : ''
  if (open) return `${props.label} · ${open}`
  return props.label
})

function openOps() {
  if (!props.linkToOps) return
  const path = ADMIN_OP_BUSY_OPS_PATH
  const hashIdx = path.indexOf('#')
  if (hashIdx >= 0) {
    void router.push({ path: path.slice(0, hashIdx) || '/operations', hash: path.slice(hashIdx) })
    return
  }
  void router.push(path)
}
</script>

<template>
  <span
    :class="surfaceClass"
    :data-testid="testId"
    :data-ok="dataOk"
    :title="chipTitle"
    :role="linkToOps ? 'link' : undefined"
    :tabindex="linkToOps ? 0 : undefined"
    @click="openOps"
    @keydown.enter.prevent="openOps"
    @keydown.space.prevent="openOps"
  >{{ t('opsStatusLastLabel') }}: {{ label }}</span>
  <span
    v-if="errorDetail"
    class="max-w-full break-all font-mono text-[11px] text-red-700 dark:text-red-300"
    :data-testid="errorTestId"
  >{{ t('adminOpLastErrorLabel') }}: {{ errorDetail }}</span>
</template>
