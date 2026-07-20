<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from '@/composables/useI18n'
import { formatMessage } from '@/utils/formatMessage'

const props = withDefaults(defineProps<{
  /** testid 前缀，如 users / databases / access-matrix */
  prefix: string
  selectedCount: number
  hasVisible: boolean
  /** 覆盖默认 `${prefix}-selection-toolbar` */
  toolbarTestId?: string
  /** 覆盖默认 `${prefix}-select-all` */
  selectAllTestId?: string
  /** 覆盖默认 `${prefix}-clear-selection`（Downsample 兼容 clear-select） */
  clearTestId?: string
  /** 覆盖默认 `${prefix}-selected-count` */
  selectedCountTestId?: string
}>(), {
  selectedCount: 0,
  hasVisible: false,
  toolbarTestId: '',
  selectAllTestId: '',
  clearTestId: '',
  selectedCountTestId: '',
})

const emit = defineEmits<{
  'select-all': []
  clear: []
}>()

const { t } = useI18n()

const toolbarId = computed(() => props.toolbarTestId || `${props.prefix}-selection-toolbar`)
const selectAllId = computed(() => props.selectAllTestId || `${props.prefix}-select-all`)
const clearId = computed(() => props.clearTestId || `${props.prefix}-clear-selection`)
const selectedId = computed(() => props.selectedCountTestId || `${props.prefix}-selected-count`)
</script>

<template>
  <div class="flex flex-wrap items-center gap-2" :data-testid="toolbarId">
    <span
      v-if="selectedCount > 0"
      class="text-xs text-sky-700 dark:text-sky-300"
      :data-testid="selectedId"
    >{{ formatMessage(t('listSelectedCount'), { count: selectedCount }) }}</span>
    <button
      type="button"
      class="mts-btn"
      :data-testid="selectAllId"
      :disabled="!hasVisible"
      @click="emit('select-all')"
    >{{ t('listSelectAll') }}</button>
    <button
      type="button"
      class="mts-btn"
      :data-testid="clearId"
      :disabled="selectedCount <= 0"
      @click="emit('clear')"
    >{{ t('listClearSelection') }}</button>
    <slot name="actions" />
  </div>
</template>
