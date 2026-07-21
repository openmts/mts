/** 进行中请求的耗时展示与长耗时阈值 */

export function formatElapsedSeconds(ms: number): string {
  if (!Number.isFinite(ms) || ms < 0) return '0s'
  const total = Math.floor(ms / 1000)
  const m = Math.floor(total / 60)
  const s = total % 60
  if (m <= 0) return `${s}s`
  return `${m}m${s.toString().padStart(2, '0')}s`
}

export function isLongRunning(ms: number, thresholdMs = 5_000): boolean {
  return Number.isFinite(ms) && ms >= thresholdMs
}

export function elapsedSince(startedAtMs: number | null | undefined, nowMs = Date.now()): number {
  if (startedAtMs == null || !Number.isFinite(startedAtMs) || startedAtMs <= 0) return 0
  return Math.max(0, nowMs - startedAtMs)
}

/** 客户端默认 API 超时（与 requestTimeout 对齐，仅用于 UI 提示） */
export function defaultApiTimeoutHintMs(envMs?: number | null): number {
  if (envMs != null && Number.isFinite(envMs) && envMs > 0) return Math.trunc(envMs)
  return 30_000
}
