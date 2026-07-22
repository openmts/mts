<script setup lang="ts">
/**
 * 页头「最近一次管理重操作」芯片 + 失败时错误明细（可商用运维可见性）。
 * 多根节点：芯片 / 可选复制钮 / 错误行作为 flex 子项。
 * 默认点击芯片跳转运维状态条；可选 showCopy 复制最近一次结果。
 */
import { computed } from 'vue'
import { useRouter } from 'vue-router'
import { Copy } from 'lucide-vue-next'
import { useI18n } from '@/composables/useI18n'
import { useNotify } from '@/composables/useNotify'
import { useAdminOpBusy } from '@/composables/useAdminOpBusy'
import { copyText } from '@/utils/clipboard'
import {
  ADMIN_OP_BUSY_OPS_PATH,
  adminOpKindLabelKey,
  adminOpLastChipSurfaceClass,
  formatAdminHeavyLastCopyText,
} from '@/utils/adminOpBusy'
import type { MessageKey } from '@/i18n/messages'

const props = withDefaults(
  defineProps<{
    label: string
    lastOk?: boolean | null
    lastError?: string | null
    testId?: string
    errorTestId?: string
    copyTestId?: string
    /** 失败时是否展示 error 明细行，默认 true */
    showError?: boolean
    /** 点击芯片跳转运维（默认 true） */
    linkToOps?: boolean
    /** 展示复制按钮（默认 false，关键页按需开启） */
    showCopy?: boolean
  }>(),
  {
    lastOk: null,
    lastError: '',
    testId: 'admin-last',
    errorTestId: 'admin-last-error',
    copyTestId: 'admin-last-copy',
    showError: true,
    linkToOps: true,
    showCopy: false,
  },
)

const { t } = useI18n()
const router = useRouter()
const { success, error: notifyError } = useNotify()
const { adminOpLast } = useAdminOpBusy()

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

async function copyLast(ev?: Event) {
  ev?.stopPropagation()
  const raw = adminOpLast.value
  if (raw && raw.op) {
    const key = adminOpKindLabelKey(raw.op) as MessageKey
    const kind = t.value(key) || raw.op
    const textToCopy = formatAdminHeavyLastCopyText(raw, kind)
    if (!textToCopy) {
      notifyError(t.value('opsStatusLastEmpty'))
      return
    }
    const res = await copyText(textToCopy)
    if (res.ok) success(t.value('opsStatusLastCopied'))
    else notifyError(res.error || t.value('failed'))
    return
  }
  const label = String(props.label || '').trim()
  const err = errorDetail.value
  const textToCopy = err ? `${label}\nerror=${err}` : label
  if (!textToCopy) {
    notifyError(t.value('opsStatusLastEmpty'))
    return
  }
  const res = await copyText(textToCopy)
  if (res.ok) success(t.value('opsStatusLastCopied'))
  else notifyError(res.error || t.value('failed'))
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
  <button
    v-if="showCopy && label"
    type="button"
    class="mts-btn py-0.5 text-[11px] shrink-0"
    :data-testid="copyTestId"
    :title="t('opsStatusLastCopy')"
    @click="copyLast"
  >
    <Copy class="h-3 w-3" /> {{ t('opsStatusLastCopy') }}
  </button>
  <span
    v-if="errorDetail"
    class="max-w-full break-all font-mono text-[11px] text-red-700 dark:text-red-300"
    :data-testid="errorTestId"
  >{{ t('adminOpLastErrorLabel') }}: {{ errorDetail }}</span>
</template>
