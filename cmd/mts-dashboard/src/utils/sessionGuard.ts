/** 会话预警/到期动作决策（纯函数，便于单测） */

import { sessionExpiryView, type SessionUrgency } from './sessionExpiry.ts'

export type SessionGuardAction =
  | { type: 'none' }
  | { type: 'toast'; urgency: 'warn' | 'critical'; remainingLabel: string }
  | { type: 'expire' }

export interface SessionGuardState {
  warnedWarn: boolean
  warnedCritical: boolean
  expiredHandled: boolean
}

export function emptySessionGuardState(): SessionGuardState {
  return { warnedWarn: false, warnedCritical: false, expiredHandled: false }
}

/**
 * 根据 expiresAt 与已提示状态，决定本轮动作。
 * 同一 urgency 只 toast 一次；expired 只触发一次 expire。
 */
export function nextSessionGuardAction(
  expiresAtMs: number | null,
  nowMs: number,
  state: SessionGuardState,
  warnMs = 10 * 60_000,
  criticalMs = 2 * 60_000,
): { action: SessionGuardAction; state: SessionGuardState } {
  const view = sessionExpiryView(expiresAtMs, nowMs, warnMs, criticalMs)
  const next: SessionGuardState = { ...state }

  if (view.urgency === 'expired') {
    if (state.expiredHandled) {
      return { action: { type: 'none' }, state: next }
    }
    next.expiredHandled = true
    return { action: { type: 'expire' }, state: next }
  }

  if (view.urgency === 'critical' && !state.warnedCritical) {
    next.warnedCritical = true
    // critical 也覆盖 warn，避免随后再弹 warn
    next.warnedWarn = true
    return {
      action: { type: 'toast', urgency: 'critical', remainingLabel: view.label },
      state: next,
    }
  }

  if (view.urgency === 'warn' && !state.warnedWarn) {
    next.warnedWarn = true
    return {
      action: { type: 'toast', urgency: 'warn', remainingLabel: view.label },
      state: next,
    }
  }

  return { action: { type: 'none' }, state: next }
}

export function shouldShowSessionBadge(urgency: SessionUrgency): boolean {
  return urgency === 'ok' || urgency === 'warn' || urgency === 'critical' || urgency === 'expired'
}
