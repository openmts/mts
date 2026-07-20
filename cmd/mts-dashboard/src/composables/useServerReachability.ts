import { computed, onBeforeUnmount, onMounted, ref, type ComputedRef, type Ref } from 'vue'
import { probeReadyz } from '@/api/client'
import {
  classifyReachability,
  nextFailStreak,
  probeOutcomeFromStatus,
  shouldShowAfterFailures,
  type ProbeOutcome,
  type ReachabilityKind,
} from '@/utils/serverReachability'
import { readNavigatorOnline } from '@/utils/networkStatus'

const DEFAULT_INTERVAL_MS = 30_000
const FAIL_THRESHOLD = 2

interface SharedReachability {
  kind: ComputedRef<ReachabilityKind>
  probe: Ref<ProbeOutcome>
  failStreak: Ref<number>
  lastStatus: Ref<number>
  checking: Ref<boolean>
  offline: Ref<boolean>
  showUnreachableBanner: ComputedRef<boolean>
  checkOnce: () => Promise<void>
  retain: () => void
  release: () => void
  stop: () => void
}

let shared: SharedReachability | null = null

function createShared(opts?: { intervalMs?: number; failThreshold?: number }): SharedReachability {
  const intervalMs = Math.max(5_000, opts?.intervalMs ?? DEFAULT_INTERVAL_MS)
  const failThreshold = Math.max(1, opts?.failThreshold ?? FAIL_THRESHOLD)

  const offline = ref(!readNavigatorOnline())
  const probe = ref<ProbeOutcome>('skipped')
  const failStreak = ref(0)
  const lastStatus = ref(0)
  const checking = ref(false)
  let timer: ReturnType<typeof setInterval> | null = null
  let abort: AbortController | null = null
  let subscribers = 0
  let netBound = false

  const view = computed(() =>
    classifyReachability(offline.value ? 'offline' : 'online', probe.value),
  )
  const kind = computed<ReachabilityKind>(() => view.value.kind)
  const showUnreachableBanner = computed(() => {
    if (!view.value.showUnreachableBanner) return false
    return shouldShowAfterFailures(failStreak.value, failThreshold)
  })

  function onOnline() {
    offline.value = false
    failStreak.value = 0
    void checkOnce()
  }
  function onOffline() {
    offline.value = true
    probe.value = 'skipped'
  }

  function bindNet() {
    if (netBound || typeof window === 'undefined') return
    window.addEventListener('online', onOnline)
    window.addEventListener('offline', onOffline)
    netBound = true
    offline.value = !readNavigatorOnline()
  }

  function unbindNet() {
    if (!netBound || typeof window === 'undefined') return
    window.removeEventListener('online', onOnline)
    window.removeEventListener('offline', onOffline)
    netBound = false
  }

  async function checkOnce() {
    offline.value = !readNavigatorOnline()
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
    bindNet()
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
    unbindNet()
  }

  function retain() {
    subscribers += 1
    if (subscribers === 1) start()
  }

  function release() {
    subscribers = Math.max(0, subscribers - 1)
    if (subscribers === 0) stop()
  }

  return {
    kind,
    probe,
    failStreak,
    lastStatus,
    checking,
    offline,
    showUnreachableBanner,
    checkOnce,
    retain,
    release,
    stop,
  }
}

/** 共享服务可达性状态；多组件只保留一个定时器 */
export function useServerReachability(opts?: {
  intervalMs?: number
  failThreshold?: number
  auto?: boolean
}) {
  if (!shared) shared = createShared(opts)
  const auto = opts?.auto !== false

  onMounted(() => {
    if (auto) shared?.retain()
  })
  onBeforeUnmount(() => {
    if (auto) shared?.release()
  })

  return {
    kind: shared.kind,
    probe: shared.probe,
    failStreak: shared.failStreak,
    lastStatus: shared.lastStatus,
    checking: shared.checking,
    offline: shared.offline,
    showUnreachableBanner: shared.showUnreachableBanner,
    checkOnce: shared.checkOnce,
  }
}

/** 测试辅助：重置单例 */
export function __resetServerReachabilityForTests() {
  shared?.stop()
  shared = null
}
