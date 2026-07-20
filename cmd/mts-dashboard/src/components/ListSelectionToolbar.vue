<script setup lang="ts">
import { useI18n } from '@/composables/useI18n'
import { formatMessage } from '@/utils/formatMessage'

const props = withDefaults(defineProps<{
  /** testid 前缀，如 users / databases / access-matrix */
  prefix: string
  selectedCount: number
  hasVisible: boolean
}>(), {
  selectedCount: 0,
  hasVisible: false,
})

const emit = defineEmits<{
  'select-all': []
  clear: []
}>()

const { t } = useI18n()
</script>

<template>
  <div class="flex flex-wrap items-center gap-2" :data-testid="`${prefix}-selection-toolbar`">
    <span
      v-if="selectedCount > 0"
      class="text-xs text-sky-700 dark:text-sky-300"
      :data-testid="`${prefix}-selected-count`"
    >{{ formatMessage(t('listSelectedCount'), { count: selectedCount }) }}</span>
    <button
      type="button"
      class="mts-btn"
      :data-testid="`${prefix}-select-all`"
      :disabled="!hasVisible"
      @click="emit('select-all')"
    >{{ t('listSelectAll') }}</button>
    <button
      type="button"
      class="mts-btn"
      :data-testid="`${prefix}-clear-selection`"
      :disabled="selectedCount <= 0"
      @click="emit('clear')"
    >{{ t('listClearSelection') }}</button>
    <slot name="actions" />
  </div>
</template>
