import { computed, onBeforeUnmount, onMounted, ref, type ComputedRef, type Ref } from 'vue'
import { apiGetSilent } from '@/api/client'
import type { OpsStatusResponse } from '@/api/types'
import { useAuth } from '@/composables/useAuth'
import {
  adminOpPollIntervalMs as resolveAdminOpPollIntervalMs,
  nextAdminOpFailStreak,
  parseAdminOpStatusPayload,
  shouldPollAdminOpBusy,
  type AdminOpStatus,
} from '@/utils/adminOpBusy'

const IDLE_INTERVAL_MS = 15_000
const BUSY_INTERVAL_MS = 3_000
const MIN_INTERVAL_MS = 2_000

interface SharedAdminOpBusy {
  status: Ref<AdminOpStatus>
  lastCheckedAt: Ref<number | null>
  checking: Ref<boolean>
  error: Ref<string>
  failStreak: Ref<number>
  pollIntervalMs: Ref<number>
  refresh: () => Promise<void>
  setBusy: (v: boolean, op?: string) => void
  markBusyAndRefresh: (op?: string) => void
  applyStatus: (s: AdminOpStatus) => void
  retain: () => void
  release: () => void
}

let shared: SharedAdminOpBusy | null = null

function emptyStatus(): AdminOpStatus {
  return { busy: false, op: '', startedAtUnix: null, last: null }
}

function createShared(idleIntervalMs: number): SharedAdminOpBusy {
  const idleMs = Math.max(MIN_INTERVAL_MS, idleIntervalMs)
  const busyMs = Math.max(MIN_INTERVAL_MS, Math.min(BUSY_INTERVAL_MS, idleMs))
  const status = ref<AdminOpStatus>(emptyStatus())
  const lastCheckedAt = ref<number | null>(null)
  const checking = ref(false)
  const error = ref('')
  const failStreak = ref(0)
  const pollIntervalMs = ref(idleMs)
  let timer: ReturnType<typeof setInterval> | null = null
  let subscribers = 0
  let abort: AbortController | null = null
  const { isAdmin, isAuthenticated } = useAuth()

  function desiredInterval(): number {
    return resolveAdminOpPollIntervalMs({
      failStreak: failStreak.value,
      busy: status.value.busy,
      idleMs,
      busyMs,
    })
  }

  function arm() {
    const next = desiredInterval()
    pollIntervalMs.value = next
    if (timer) {
      clearInterval(timer)
      timer = null
    }
    timer = setInterval(() => {
      void refresh()
    }, next)
  }

  function rearmIfNeeded() {
    if (subscribers === 0) return
    if (pollIntervalMs.value !== desiredInterval() || !timer) arm()
  }

  function applyStatus(s: AdminOpStatus) {
    const last = s.last !== undefined ? s.last : status.value.last
    status.value = {
      busy: Boolean(s.busy),
      op: s.busy ? (s.op || '') : '',
      startedAtUnix: s.busy ? s.startedAtUnix : null,
      last: last ?? null,
    }
    rearmIfNeeded()
  }

  function setBusy(v: boolean, op?: string) {
    if (!v) {
      applyStatus({ ...emptyStatus(), last: status.value.last })
      return
    }
    applyStatus({
      busy: true,
      op: (op || status.value.op || '').trim(),
      startedAtUnix: status.value.startedAtUnix ?? Math.floor(Date.now() / 1000),
      last: status.value.last,
    })
  }

  async function refresh() {
    if (!shouldPollAdminOpBusy(isAuthenticated.value, isAdmin.value)) {
      applyStatus(emptyStatus())
      error.value = ''
      failStreak.value = 0
      rearmIfNeeded()
      return
    }
    if (checking.value) return
    checking.value = true
    abort?.abort()
    abort = new AbortController()
    const signal = abort.signal
    try {
      const v = await apiGetSilent<OpsStatusResponse>('/api/v1/admin/ops-status', {
        signal,
      })
      if (signal.aborted) return
      applyStatus(parseAdminOpStatusPayload(v))
      lastCheckedAt.value = Date.now()
      error.value = ''
      const prev = failStreak.value
      failStreak.value = nextAdminOpFailStreak(failStreak.value, true)
      if (prev !== 0) rearmIfNeeded()
    } catch (e) {
      if (signal.aborted) return
      // 轮询失败不闪断 busy；保留上次值，并指数退避降低噪音
      error.value = e instanceof Error ? e.message : String(e)
      failStreak.value = nextAdminOpFailStreak(failStreak.value, false)
      rearmIfNeeded()
    } finally {
      if (!signal.aborted) checking.value = false
    }
  }

  function markBusyAndRefresh(op?: string) {
    setBusy(true, op)
    void refresh()
  }

  function disarm() {
    if (timer) clearInterval(timer)
    timer = null
    abort?.abort()
    abort = null
  }

  function retain() {
    subscribers += 1
    if (subscribers === 1) {
      arm()
      void refresh()
    }
  }

  function release() {
    subscribers = Math.max(0, subscribers - 1)
    if (subscribers === 0) disarm()
  }

  return {
    status,
    lastCheckedAt,
    checking,
    error,
    failStreak,
    pollIntervalMs,
    refresh,
    setBusy,
    markBusyAndRefresh,
    applyStatus,
    retain,
    release,
  }
}

function getShared(intervalMs?: number): SharedAdminOpBusy {
  if (!shared) {
    shared = createShared(intervalMs ?? IDLE_INTERVAL_MS)
  }
  return shared
}

/** 管理员会话内静默轮询 admin_op_busy（不触发全局 loading）。busy 时自动加速到 ~3s。 */
export function useAdminOpBusy(opts?: { intervalMs?: number }): {
  adminOpBusy: ComputedRef<boolean>
  adminOpKind: ComputedRef<string>
  adminOpStartedAtUnix: ComputedRef<number | null>
  adminOpLast: ComputedRef<import('@/utils/adminOpBusy').AdminHeavyLast | null>
  adminOpBusyChecking: Ref<boolean>
  adminOpBusyError: Ref<string>
  adminOpBusyFailStreak: Ref<number>
  adminOpBusyCheckedAt: Ref<number | null>
  adminOpPollIntervalMs: Ref<number>
  refreshAdminOpBusy: () => Promise<void>
  setAdminOpBusy: (v: boolean, op?: string) => void
  markAdminOpBusyAndRefresh: (op?: string) => void
  applyAdminOpStatus: (s: AdminOpStatus) => void
} {
  const s = getShared(opts?.intervalMs)
  onMounted(() => s.retain())
  onBeforeUnmount(() => s.release())
  return {
    adminOpBusy: computed(() => s.status.value.busy),
    adminOpKind: computed(() => s.status.value.op),
    adminOpStartedAtUnix: computed(() => s.status.value.startedAtUnix),
    adminOpLast: computed(() => s.status.value.last),
    adminOpBusyChecking: s.checking,
    adminOpBusyError: s.error,
    adminOpBusyFailStreak: s.failStreak,
    adminOpBusyCheckedAt: s.lastCheckedAt,
    adminOpPollIntervalMs: s.pollIntervalMs,
    refreshAdminOpBusy: s.refresh,
    setAdminOpBusy: s.setBusy,
    markAdminOpBusyAndRefresh: s.markBusyAndRefresh,
    applyAdminOpStatus: s.applyStatus,
  }
}

/** 测试重置共享态 */
export function __resetAdminOpBusyForTests() {
  shared = null
}
