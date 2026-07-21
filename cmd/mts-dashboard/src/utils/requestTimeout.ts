/** fetch 请求超时与 AbortSignal 合并 */

export const DEFAULT_API_TIMEOUT_MS = 30_000

/** 解析超时毫秒：非法/非正数回退 defaultMs */
export function resolveApiTimeoutMs(
  raw: unknown,
  defaultMs: number = DEFAULT_API_TIMEOUT_MS,
): number {
  if (raw == null || raw === '') return defaultMs
  const n = typeof raw === 'number' ? raw : Number(String(raw).trim())
  if (!Number.isFinite(n) || n <= 0) return defaultMs
  // 上限 10 分钟，避免误配无限挂起伪装
  return Math.min(Math.trunc(n), 600_000)
}

export interface TimeoutSignalHandle {
  signal: AbortSignal
  /** 是否因超时 abort */
  didTimeout: () => boolean
  cleanup: () => void
}

/**
 * 合并用户 AbortSignal 与超时。
 * timeoutMs <= 0 时不启用超时，仅透传用户 signal（无用户 signal 时创建永不自动 abort 的 controller）。
 */
export function createTimeoutSignal(
  userSignal: AbortSignal | null | undefined,
  timeoutMs: number = DEFAULT_API_TIMEOUT_MS,
): TimeoutSignalHandle {
  if (timeoutMs <= 0) {
    if (userSignal) {
      return { signal: userSignal, didTimeout: () => false, cleanup: () => {} }
    }
    const c = new AbortController()
    return { signal: c.signal, didTimeout: () => false, cleanup: () => {} }
  }

  const controller = new AbortController()
  let timedOut = false
  const timer = setTimeout(() => {
    timedOut = true
    try {
      controller.abort()
    } catch {
      /* ignore */
    }
  }, timeoutMs)

  const onUserAbort = () => {
    try {
      controller.abort()
    } catch {
      /* ignore */
    }
  }

  if (userSignal) {
    if (userSignal.aborted) onUserAbort()
    else userSignal.addEventListener('abort', onUserAbort, { once: true })
  }

  return {
    signal: controller.signal,
    didTimeout: () => timedOut,
    cleanup: () => {
      clearTimeout(timer)
      if (userSignal) userSignal.removeEventListener('abort', onUserAbort)
    },
  }
}

export function isAbortError(err: unknown): boolean {
  if (err instanceof DOMException && err.name === 'AbortError') return true
  if (err && typeof err === 'object' && (err as { name?: string }).name === 'AbortError') return true
  return false
}
