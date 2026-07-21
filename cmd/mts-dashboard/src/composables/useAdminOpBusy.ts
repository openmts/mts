import { computed, onBeforeUnmount, onMounted, ref, type ComputedRef, type Ref } from 'vue'
import { apiGetSilent } from '@/api/client'
import type { OpsStatusResponse } from '@/api/types'
import { useAuth } from '@/composables/useAuth'
import { parseAdminOpBusyPayload, shouldPollAdminOpBusy } from '@/utils/adminOpBusy'

const DEFAULT_INTERVAL_MS = 15_000

interface SharedAdminOpBusy {
  busy: Ref<boolean>
  lastCheckedAt: Ref<number | null>
  checking: Ref<boolean>
  error: Ref<string>
  refresh: () => Promise<void>
  setBusy: (v: boolean) => void
  markBusyAndRefresh: () => void
  retain: () => void
  release: () => void
}

let shared: SharedAdminOpBusy | null = null

function createShared(intervalMs: number): SharedAdminOpBusy {
  const busy = ref(false)
  const lastCheckedAt = ref<number | null>(null)
  const checking = ref(false)
  const error = ref('')
  let timer: ReturnType<typeof setInterval> | null = null
  let subscribers = 0
  let abort: AbortController | null = null
  const { isAdmin, isAuthenticated } = useAuth()

  function setBusy(v: boolean) {
    busy.value = Boolean(v)
  }

  async function refresh() {
    if (!shouldPollAdminOpBusy(isAuthenticated.value, isAdmin.value)) {
      busy.value = false
      error.value = ''
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
      busy.value = parseAdminOpBusyPayload(v)
      lastCheckedAt.value = Date.now()
      error.value = ''
    } catch (e) {
      if (signal.aborted) return
      // 轮询失败不闪断 busy；保留上次值
      error.value = e instanceof Error ? e.message : String(e)
    } finally {
      if (!signal.aborted) checking.value = false
    }
  }

  function markBusyAndRefresh() {
    setBusy(true)
    void refresh()
  }

  function arm() {
    if (timer) return
    timer = setInterval(() => {
      void refresh()
    }, intervalMs)
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
    busy,
    lastCheckedAt,
    checking,
    error,
    refresh,
    setBusy,
    markBusyAndRefresh,
    retain,
    release,
  }
}

function getShared(intervalMs?: number): SharedAdminOpBusy {
  if (!shared) {
    shared = createShared(Math.max(5_000, intervalMs ?? DEFAULT_INTERVAL_MS))
  }
  return shared
}

/** 管理员会话内静默轮询 admin_op_busy（不触发全局 loading）。 */
export function useAdminOpBusy(opts?: { intervalMs?: number }): {
  adminOpBusy: ComputedRef<boolean>
  adminOpBusyChecking: Ref<boolean>
  adminOpBusyError: Ref<string>
  adminOpBusyCheckedAt: Ref<number | null>
  refreshAdminOpBusy: () => Promise<void>
  setAdminOpBusy: (v: boolean) => void
  markAdminOpBusyAndRefresh: () => void
} {
  const s = getShared(opts?.intervalMs)
  onMounted(() => s.retain())
  onBeforeUnmount(() => s.release())
  return {
    adminOpBusy: computed(() => s.busy.value),
    adminOpBusyChecking: s.checking,
    adminOpBusyError: s.error,
    adminOpBusyCheckedAt: s.lastCheckedAt,
    refreshAdminOpBusy: s.refresh,
    setAdminOpBusy: s.setBusy,
    markAdminOpBusyAndRefresh: s.markBusyAndRefresh,
  }
}

/** 测试重置共享态 */
export function __resetAdminOpBusyForTests() {
  shared = null
}
