<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import type { QueryResultRow } from '@/composables/useQueryWorkbench'
import { buildPolyline, extractNumericFieldNames, extractSeries } from '@/utils/chart'
import { useI18n } from '@/composables/useI18n'

const props = defineProps<{ rows: QueryResultRow[] }>()
const { t } = useI18n()
const field = ref('')
const width = 640
const height = 220

const fields = computed(() => extractNumericFieldNames(props.rows))
watch(fields, (fs) => {
  if (!fs.includes(field.value)) field.value = fs[0] || ''
}, { immediate: true })

const series = computed(() => (field.value ? extractSeries(props.rows, field.value) : []))
const poly = computed(() => buildPolyline(series.value, width, height))
</script>

<template>
  <div class="rounded-2xl border border-slate-200 bg-white p-4 shadow-sm dark:border-slate-700 dark:bg-slate-900">
    <div class="mb-3 flex flex-wrap items-center justify-between gap-2">
      <h3 class="text-sm font-semibold text-slate-800 dark:text-slate-100">{{ t('chart') }}</h3>
      <label class="flex items-center gap-2 text-xs text-slate-500 dark:text-slate-400">
        {{ t('field') }}
        <select v-model="field" class="rounded border border-slate-300 bg-white px-2 py-1 text-xs dark:border-slate-600 dark:bg-slate-800 dark:text-slate-100">
          <option v-for="f in fields" :key="f" :value="f">{{ f }}</option>
        </select>
      </label>
    </div>
    <div v-if="!fields.length" class="py-8 text-center text-sm text-slate-400">{{ t('noChartData') }}</div>
    <svg v-else :viewBox="`0 0 ${width} ${height}`" class="h-56 w-full">
      <rect x="0" y="0" :width="width" :height="height" class="fill-slate-50 dark:fill-slate-950" />
      <path :d="poly.path" fill="none" stroke="#2563eb" stroke-width="2" />
      <text x="8" y="16" class="fill-slate-400 text-[10px]">{{ poly.maxY.toFixed(2) }}</text>
      <text x="8" :y="height - 8" class="fill-slate-400 text-[10px]">{{ poly.minY.toFixed(2) }}</text>
    </svg>
  </div>
</template>
