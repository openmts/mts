import { computed, ref } from 'vue'

const inflight = ref(0)
const routeLoading = ref(false)

export function beginRequest() {
  inflight.value += 1
}

export function endRequest() {
  inflight.value = Math.max(0, inflight.value - 1)
}

export function setRouteLoading(v: boolean) {
  routeLoading.value = v
}

export function useGlobalLoading() {
  const busy = computed(() => inflight.value > 0 || routeLoading.value)
  const requestCount = computed(() => inflight.value)
  return {
    busy,
    requestCount,
    routeLoading,
    beginRequest,
    endRequest,
    setRouteLoading,
  }
}
