import type { MessageKey } from '../i18n/messages.ts'

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

/** 默认离线文案 key（管理写操作） */
export const DEFAULT_OFFLINE_ADMIN_KEY: MessageKey = 'offlineAdminBlocked'

/**
 * 根据阻断原因选择 i18n key：session 固定 sessionMutationBlocked，否则用场景 offline key。
 * reason 为 none 时仍返回 offlineKey（调用方应先判断 writeBlocked）。
 */
export function mutationBlockedMessageKey(
  reason: MutationBlockReason | null | undefined,
  offlineKey: MessageKey = DEFAULT_OFFLINE_ADMIN_KEY,
): MessageKey {
  if (reason === 'session') return 'sessionMutationBlocked'
  return offlineKey || DEFAULT_OFFLINE_ADMIN_KEY
}

/**
 * 按钮 title / 提示文案用：未阻断时返回 undefined。
 * translate 由调用方注入（t 或 t.value）。
 */
export function mutationBlockedTitle(
  writeBlocked: boolean | null | undefined,
  reason: MutationBlockReason | null | undefined,
  offlineKey: MessageKey,
  translate: (key: MessageKey) => string,
): string | undefined {
  if (!writeBlocked) return undefined
  return translate(mutationBlockedMessageKey(reason, offlineKey))
}
