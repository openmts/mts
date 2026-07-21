/** 变更类写操作统一门禁：离线 + 会话 critical/expired */

import { shouldBlockOfflineMutation } from './offlineGuard.ts'
import type { SessionUrgency } from './sessionExpiry.ts'

/** 会话进入 critical/expired 时阻断会改变服务端状态的操作（续期/登录除外） */
export function isSessionWriteBlocked(urgency: SessionUrgency | null | undefined): boolean {
  return urgency === 'critical' || urgency === 'expired'
}

/**
 * 统一判断是否应阻断变更写。
 * sessionUrgency 省略时仅按离线判断（兼容旧调用）。
 */
export function shouldBlockMutation(
  offline: boolean | null | undefined,
  sessionUrgency?: SessionUrgency | null,
): boolean {
  if (shouldBlockOfflineMutation(offline)) return true
  if (sessionUrgency != null && isSessionWriteBlocked(sessionUrgency)) return true
  return false
}

export type MutationBlockReason = 'none' | 'offline' | 'session'

export function mutationBlockReason(
  offline: boolean | null | undefined,
  sessionUrgency?: SessionUrgency | null,
): MutationBlockReason {
  if (shouldBlockOfflineMutation(offline)) return 'offline'
  if (sessionUrgency != null && isSessionWriteBlocked(sessionUrgency)) return 'session'
  return 'none'
}

/** 根据阻断原因选择 i18n key：session 优先，否则用各场景 offline key */
export function mutationBlockedMessageKey(
  reason: MutationBlockReason,
  offlineKey: string,
): string {
  if (reason === 'session') return 'sessionMutationBlocked'
  return offlineKey
}
