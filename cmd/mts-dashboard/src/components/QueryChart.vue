<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import type { QueryResultRow } from '@/api/types'
import { extractNumericFieldNames, extractMultiSeries, polylineInBounds, boundsOfSeries } from '@/utils/chart'
import { useI18n } from '@/composables/useI18n'

const props = defineProps<{ rows: QueryResultRow[] }>()
const { t } = useI18n()

const fieldNames = computed(() => extractNumericFieldNames(props.rows))
const selectedField = ref('')
const maxSeries = ref(6)

watch(fieldNames, (names) => {
  if (!names.includes(selectedField.value)) selectedField.value = names[0] || ''
}, { immediate: true })

const series = computed(() => {
  if (!selectedField.value) return []
  return extractMultiSeries(props.rows, selectedField.value, maxSeries.value)
})

const bounds = computed(() => boundsOfSeries(series.value))
const width = 640
const height = 220

const paths = computed(() => {
  const b = bounds.value
  if (!b) return [] as { d: string; color: string; key: string }[]
  return series.value.map((s) => ({
    key: s.key,
    color: s.color,
    d: polylineInBounds(s.points, b, width, height),
  }))
})
</script>

<template>
  <div class="mts-panel">
    <div class="mb-3 flex flex-wrap items-center justify-between gap-2">
      <h3 class="text-sm font-semibold text-slate-800 dark:text-slate-100">{{ t('chart') }}</h3>
      <div class="flex flex-wrap items-center gap-2 text-xs">
        <label class="mts-muted">{{ t('field') }}
          <select v-model="selectedField" class="ml-1 rounded border border-slate-300 px-2 py-1 dark:border-slate-600 dark:bg-slate-800">
            <option v-for="f in fieldNames" :key="f" :value="f">{{ f }}</option>
          </select>
        </label>
        <label class="mts-muted">max series
          <select v-model.number="maxSeries" class="ml-1 rounded border border-slate-300 px-2 py-1 dark:border-slate-600 dark:bg-slate-800">
            <option :value="3">3</option>
            <option :value="6">6</option>
            <option :value="10">10</option>
          </select>
        </label>
      </div>
    </div>
    <p v-if="!fieldNames.length" class="text-sm mts-muted">{{ t('noChartData') }}</p>
    <template v-else>
      <svg :viewBox="`0 0 ${width} ${height}`" class="h-56 w-full rounded bg-slate-50 dark:bg-slate-950/50">
        <path
          v-for="p in paths"
          :key="p.key"
          :d="p.d"
          fill="none"
          :stroke="p.color"
          stroke-width="2"
          stroke-linecap="round"
          stroke-linejoin="round"
        />
      </svg>
      <div class="mt-2 flex flex-wrap gap-3 text-[11px]">
        <span v-for="s in series" :key="s.key" class="inline-flex items-center gap-1">
          <span class="inline-block h-2 w-2 rounded-full" :style="{ background: s.color }" />
          <span class="font-mono text-slate-600 dark:text-slate-300">{{ s.label }} ({{ s.points.length }})</span>
        </span>
      </div>
    </template>
  </div>
</template>
