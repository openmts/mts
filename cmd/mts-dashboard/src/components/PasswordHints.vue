<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from '@/composables/useI18n'
import type { MessageKey } from '@/i18n/messages'
import { formatMessage } from '@/utils/formatMessage'
import { getMinPasswordLength } from '@/utils/passwordPolicy'
import {
  assignedPasswordHints,
  passwordHintsProgress,
  passwordRequirementHints,
  type PasswordHintItem,
} from '@/utils/passwordHints'
import { Check, Circle } from 'lucide-vue-next'

const props = withDefaults(
  defineProps<{
    /** change: 自助改密；assigned: 管理员设密/创建；change-no-confirm: 无确认框改密 */
    mode?: 'change' | 'assigned' | 'change-no-confirm'
    oldPassword?: string
    newPassword?: string
    confirmPassword?: string
    password?: string
    /** assigned 模式下是否展示确认匹配项 */
    showConfirmHint?: boolean
  }>(),
  {
    mode: 'change',
    oldPassword: '',
    newPassword: '',
    confirmPassword: '',
    password: '',
    showConfirmHint: false,
  },
)

const { t } = useI18n()
const hints = computed((): PasswordHintItem[] => {
  if (props.mode === 'assigned') {
    const pwd = props.password || props.newPassword || ''
    if (props.showConfirmHint) return assignedPasswordHints(pwd, props.confirmPassword)
    return assignedPasswordHints(pwd)
  }
  if (props.mode === 'change-no-confirm') {
    return passwordRequirementHints(props.oldPassword || '', props.newPassword || '', props.newPassword || '').filter(
      (h) => h.id !== 'confirm_match',
    )
  }
  return passwordRequirementHints(
    props.oldPassword || '',
    props.newPassword || '',
    props.confirmPassword || '',
  )
})
const progress = computed(() => passwordHintsProgress(hints.value))

function labelOf(key: string): string {
  if (key === 'passwordHintMinLength') {
    return formatMessage(t.value(key as MessageKey), { n: getMinPasswordLength() })
  }
  return t.value(key as MessageKey)
}
</script>

<template>
  <div class="rounded-lg border border-slate-200 bg-slate-50 p-3 dark:border-slate-700 dark:bg-slate-900/60" data-testid="password-hints">
    <div class="mb-2 flex items-center justify-between gap-2">
      <p class="text-[11px] font-medium text-slate-600 dark:text-slate-300">{{ t('passwordHintsTitle') }}</p>
      <span
        class="font-mono text-[10px] text-slate-500 dark:text-slate-400"
        data-testid="password-hints-progress-label"
      >{{ progress.done }}/{{ progress.total }} · {{ progress.percent }}%</span>
    </div>
    <div
      class="mb-2 h-1.5 overflow-hidden rounded-full bg-slate-200 dark:bg-slate-700"
      data-testid="password-hints-progress"
      role="progressbar"
      :aria-valuenow="progress.percent"
      aria-valuemin="0"
      aria-valuemax="100"
      :aria-label="t('passwordHintsTitle')"
    >
      <div
        class="h-full rounded-full transition-all"
        :class="progress.percent >= 100 ? 'bg-emerald-500' : 'bg-amber-500'"
        :style="{ width: `${progress.percent}%` }"
      />
    </div>
    <ul class="space-y-1">
      <li
        v-for="h in hints"
        :key="h.id"
        class="flex items-center gap-2 text-[11px]"
        :class="h.ok ? 'text-emerald-700 dark:text-emerald-300' : 'text-slate-500 dark:text-slate-400'"
        :data-testid="`password-hint-${h.id}`"
        :data-ok="h.ok ? 'true' : 'false'"
      >
        <Check v-if="h.ok" class="h-3 w-3 shrink-0" aria-hidden="true" />
        <Circle v-else class="h-3 w-3 shrink-0" aria-hidden="true" />
        <span>{{ labelOf(h.labelKey) }}</span>
      </li>
    </ul>
  </div>
</template>
