import { computed, ref } from 'vue'

const inflight = ref(0)
const routeLoading = ref(false)
/** 最近一次从空闲进入忙碌的时间戳；空闲时为 0 */
const busySinceMs = ref(0)
const nowMs = ref(Date.now())

/** 超过该阈值（ms）视为长请求，用于 UI 文案提示 */
export const LONG_REQUEST_THRESHOLD_MS = 1200

let tickTimer: ReturnType<typeof setInterval> | null = null

function ensureTicker() {
  if (tickTimer != null) return
  if (typeof setInterval === 'undefined') return
  tickTimer = setInterval(() => {
    nowMs.value = Date.now()
    // 空闲时停表，减少无用定时器
    if (inflight.value === 0 && !routeLoading.value) {
      if (tickTimer) clearInterval(tickTimer)
      tickTimer = null
    }
  }, 250)
}

function markBusyStart() {
  if (busySinceMs.value === 0) {
    busySinceMs.value = Date.now()
    nowMs.value = busySinceMs.value
  }
  ensureTicker()
}

function markIdleIfQuiet() {
  if (inflight.value === 0 && !routeLoading.value) {
    busySinceMs.value = 0
  }
}

export function beginRequest() {
  if (inflight.value === 0) markBusyStart()
  inflight.value += 1
  ensureTicker()
}

export function endRequest() {
  inflight.value = Math.max(0, inflight.value - 1)
  markIdleIfQuiet()
}

export function setRouteLoading(v: boolean) {
  routeLoading.value = v
  if (v) markBusyStart()
  else markIdleIfQuiet()
}

/** 测试/复位用 */
export function resetGlobalLoading() {
  inflight.value = 0
  routeLoading.value = false
  busySinceMs.value = 0
  if (tickTimer) {
    clearInterval(tickTimer)
    tickTimer = null
  }
}

export function useGlobalLoading(opts?: { longThresholdMs?: number }) {
  const longThresholdMs = opts?.longThresholdMs ?? LONG_REQUEST_THRESHOLD_MS
  const busy = computed(() => inflight.value > 0 || routeLoading.value)
  const requestCount = computed(() => inflight.value)
  const elapsedMs = computed(() => {
    // 依赖 nowMs 以驱动刷新；实际用 Date.now 减少 ticker 漂移误差
    void nowMs.value
    if (!busy.value || busySinceMs.value <= 0) return 0
    return Math.max(0, Date.now() - busySinceMs.value)
  })
  const longBusy = computed(() => {
    void nowMs.value
    if (!busy.value || busySinceMs.value <= 0) return false
    return Date.now() - busySinceMs.value >= longThresholdMs
  })

  return {
    busy,
    longBusy,
    elapsedMs,
    requestCount,
    routeLoading,
    beginRequest,
    endRequest,
    setRouteLoading,
  }
}
