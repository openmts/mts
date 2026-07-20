import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { probeReadyz } from '@/api/client'
import { useNetworkStatus } from '@/composables/useNetworkStatus'
import {
  classifyReachability,
  nextFailStreak,
  probeOutcomeFromStatus,
  shouldShowAfterFailures,
  type ProbeOutcome,
  type ReachabilityKind,
} from '@/utils/serverReachability'

const DEFAULT_INTERVAL_MS = 30_000
const FAIL_THRESHOLD = 2

/** 周期性探测 /readyz；与浏览器 offline 区分 */
export function useServerReachability(opts?: {
  intervalMs?: number
  failThreshold?: number
  /** 测试注入：禁用自动定时 */
  auto?: boolean
}) {
  const intervalMs = Math.max(5_000, opts?.intervalMs ?? DEFAULT_INTERVAL_MS)
  const failThreshold = Math.max(1, opts?.failThreshold ?? FAIL_THRESHOLD)
  const auto = opts?.auto !== false

  const { offline, status: browserStatus } = useNetworkStatus()
  const probe = ref<ProbeOutcome>('skipped')
  const failStreak = ref(0)
  const lastStatus = ref(0)
  const checking = ref(false)
  let timer: ReturnType<typeof setInterval> | null = null
  let abort: AbortController | null = null

  const view = computed(() => {
    const browser = offline.value ? 'offline' : 'online'
    return classifyReachability(browser, probe.value)
  })

  const kind = computed<ReachabilityKind>(() => view.value.kind)

  const showUnreachableBanner = computed(() => {
    if (!view.value.showUnreachableBanner) return false
    return shouldShowAfterFailures(failStreak.value, failThreshold)
  })

  async function checkOnce() {
    if (offline.value) {
      probe.value = 'skipped'
      return
    }
    if (checking.value) return
    checking.value = true
    abort?.abort()
    abort = new AbortController()
    try {
      const r = await probeReadyz({ signal: abort.signal })
      lastStatus.value = r.status
      const outcome = r.ok ? 'ok' : probeOutcomeFromStatus(r.status || null)
      probe.value = outcome
      failStreak.value = nextFailStreak(failStreak.value, outcome === 'ok')
    } catch {
      probe.value = 'fail'
      lastStatus.value = 0
      failStreak.value = nextFailStreak(failStreak.value, false)
    } finally {
      checking.value = false
    }
  }

  function start() {
    if (timer) return
    void checkOnce()
    timer = setInterval(() => {
      void checkOnce()
    }, intervalMs)
  }

  function stop() {
    if (timer) {
      clearInterval(timer)
      timer = null
    }
    abort?.abort()
    abort = null
  }

  // 从离线恢复时立刻探测
  watch(offline, (isOff, wasOff) => {
    if (wasOff && !isOff) {
      failStreak.value = 0
      void checkOnce()
    }
    if (isOff) {
      probe.value = 'skipped'
    }
  })

  onMounted(() => {
    if (auto) start()
  })
  onBeforeUnmount(() => {
    stop()
  })

  return {
    kind,
    probe,
    failStreak,
    lastStatus,
    checking,
    showUnreachableBanner,
    browserStatus,
    checkOnce,
    start,
    stop,
  }
}
