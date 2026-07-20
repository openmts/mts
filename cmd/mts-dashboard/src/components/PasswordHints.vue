<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from '@/composables/useI18n'
import type { MessageKey } from '@/i18n/messages'
import { formatMessage } from '@/utils/formatMessage'
import { MIN_PASSWORD_LENGTH } from '@/utils/passwordPolicy'
import { passwordRequirementHints } from '@/utils/passwordHints'
import { Check, Circle } from 'lucide-vue-next'

const props = defineProps<{
  oldPassword: string
  newPassword: string
  confirmPassword: string
}>()

const { t } = useI18n()
const hints = computed(() =>
  passwordRequirementHints(props.oldPassword, props.newPassword, props.confirmPassword),
)

function labelOf(key: string): string {
  if (key === 'passwordHintMinLength') {
    return formatMessage(t.value(key as MessageKey), { n: MIN_PASSWORD_LENGTH })
  }
  return t.value(key as MessageKey)
}
</script>

<template>
  <div class="rounded-lg border border-slate-200 bg-slate-50 p-3 dark:border-slate-700 dark:bg-slate-900/60" data-testid="password-hints">
    <p class="mb-2 text-[11px] font-medium text-slate-600 dark:text-slate-300">{{ t('passwordHintsTitle') }}</p>
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
