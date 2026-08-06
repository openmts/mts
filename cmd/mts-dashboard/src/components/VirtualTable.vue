<script setup lang="ts" generic="T">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'

const props = withDefaults(defineProps<{
  items: T[]
  rowHeight?: number
  height?: number
  overscan?: number
  semanticRole?: 'list' | 'rowgroup'
}>(), {
  rowHeight: 36,
  height: 360,
  overscan: 8,
  semanticRole: 'list',
})

const scroller = ref<HTMLElement | null>(null)
const scrollTop = ref(0)

function onScroll() {
  scrollTop.value = scroller.value?.scrollTop ?? 0
}

const total = computed(() => props.items.length)
const viewportCount = computed(() => Math.ceil(props.height / props.rowHeight) + props.overscan * 2)
const start = computed(() => {
  const raw = Math.floor(scrollTop.value / props.rowHeight) - props.overscan
  return Math.max(0, raw)
})
const end = computed(() => Math.min(total.value, start.value + viewportCount.value))
const visible = computed(() => props.items.slice(start.value, end.value))
const offsetY = computed(() => start.value * props.rowHeight)
const innerHeight = computed(() => total.value * props.rowHeight)

watch(() => props.items.length, () => {
  // 结果刷新时回到顶部，避免空窗
  if (scroller.value) {
    scroller.value.scrollTop = 0
    scrollTop.value = 0
  }
})

onMounted(() => {
  scroller.value?.addEventListener('scroll', onScroll, { passive: true })
})
onBeforeUnmount(() => {
  scroller.value?.removeEventListener('scroll', onScroll)
})
</script>

<template>
  <div
    ref="scroller"
    class="relative w-full overflow-auto"
    :role="semanticRole"
    :style="{ height: height + 'px' }"
  >
    <div :style="{ height: innerHeight + 'px', position: 'relative' }">
      <div :style="{ transform: `translateY(${offsetY}px)` }">
        <div
          v-for="(item, i) in visible"
          :key="start + i"
          :style="{ height: rowHeight + 'px' }"
          :role="semanticRole === 'rowgroup' ? 'row' : 'listitem'"
          :aria-rowindex="semanticRole === 'rowgroup' ? start + i + 2 : undefined"
          :aria-posinset="semanticRole === 'list' ? start + i + 1 : undefined"
          :aria-setsize="semanticRole === 'list' ? total : undefined"
        >
          <slot :item="item" :index="start + i" />
        </div>
      </div>
    </div>
  </div>
</template>
