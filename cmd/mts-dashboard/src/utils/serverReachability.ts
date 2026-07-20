/** 服务端可达性分类（纯函数；与浏览器 offline 区分） */

export type BrowserNet = 'online' | 'offline'
export type ProbeOutcome = 'ok' | 'fail' | 'skipped'

export type ReachabilityKind =
  | 'ok'
  | 'unreachable'
  | 'offline'
  | 'unknown'

export interface ReachabilityView {
  kind: ReachabilityKind
  /** 是否展示「服务不可达」条（offline 时为 false） */
  showUnreachableBanner: boolean
}

/**
 * 综合浏览器网络与 /readyz 探测结果。
 * - offline：主因是浏览器离线
 * - fail + online：服务不可达
 * - ok：正常
 * - skipped/unknown：尚无结论
 */
export function classifyReachability(
  browser: BrowserNet,
  probe: ProbeOutcome,
): ReachabilityView {
  if (browser === 'offline') {
    return { kind: 'offline', showUnreachableBanner: false }
  }
  if (probe === 'ok') {
    return { kind: 'ok', showUnreachableBanner: false }
  }
  if (probe === 'fail') {
    return { kind: 'unreachable', showUnreachableBanner: true }
  }
  return { kind: 'unknown', showUnreachableBanner: false }
}

/** 连续失败达到阈值才展示，避免瞬抖 */
export function shouldShowAfterFailures(failStreak: number, threshold = 2): boolean {
  const t = Math.max(1, Math.trunc(threshold))
  return Math.max(0, Math.trunc(failStreak)) >= t
}

export function nextFailStreak(prev: number, ok: boolean): number {
  if (ok) return 0
  return Math.max(0, Math.trunc(prev)) + 1
}

/** 将 HTTP 状态归为探测成败（2xx 为 ok） */
export function probeOutcomeFromStatus(status: number | null | undefined): ProbeOutcome {
  if (status == null || !Number.isFinite(status)) return 'fail'
  const s = Math.trunc(status)
  if (s >= 200 && s < 300) return 'ok'
  return 'fail'
}
