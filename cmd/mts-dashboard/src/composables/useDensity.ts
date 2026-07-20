import { ref, watch } from 'vue'
import {
  applyUiDensity,
  loadUiDensity,
  saveUiDensity,
  type UiDensity,
} from '@/utils/densityPrefs'

const storage = typeof localStorage !== 'undefined' ? localStorage : null
const density = ref<UiDensity>(loadUiDensity(storage))

function apply(d: UiDensity) {
  if (typeof document === 'undefined') return
  applyUiDensity(d, document.documentElement)
}

apply(density.value)

watch(density, (d) => {
  apply(d)
  saveUiDensity(storage, d)
})

export function useDensity() {
  function setDensity(next: UiDensity) {
    density.value = next === 'compact' ? 'compact' : 'comfortable'
  }
  function toggleDensity() {
    density.value = density.value === 'compact' ? 'comfortable' : 'compact'
  }
  return { density, setDensity, toggleDensity }
}
