/** 可商用交接摘要：密码策略 + 会话校准（纯函数，供就绪归档/验收包） */

import {
  getDefaultAuthTTLSeconds,
  getForbiddenDefaultPasswords,
  getMaxAuthTTLSeconds,
  getMinPasswordLength,
  getPasswordPolicyVersion,
  getRequireChangeBootstrap,
} from './passwordPolicy.ts'
import {
  effectiveSessionRemainingMs,
  formatRemaining,
  parseExpiresAt,
  sessionViewFromRemainingMs,
  type SessionUrgency,
} from './sessionExpiry.ts'

export interface PasswordPolicyHandoffSummary {
  version: number
  min_length: number
  forbidden_defaults: string[]
  require_change_bootstrap: boolean
  default_auth_ttl_seconds: number
  max_auth_ttl_seconds: number
  /** 是否已从服务端同步过（version>0） */
  server_synced: boolean
}

export interface SessionCalibrationHandoffSummary {
  has_local_expiry: boolean
  local_remaining_seconds: number | null
  server_remaining_seconds: number | null
  server_checked_at: string | null
  calibrated_remaining_seconds: number | null
  urgency: SessionUrgency
  calibration_source: 'local_only' | 'merged' | 'unknown'
}

export interface CommercialHandoffSummary {
  password_policy: PasswordPolicyHandoffSummary
  session_calibration: SessionCalibrationHandoffSummary
}

export function buildPasswordPolicyHandoffSummary(): PasswordPolicyHandoffSummary {
  const version = getPasswordPolicyVersion()
  return {
    version,
    min_length: getMinPasswordLength(),
    forbidden_defaults: getForbiddenDefaultPasswords(),
    require_change_bootstrap: getRequireChangeBootstrap(),
    default_auth_ttl_seconds: getDefaultAuthTTLSeconds(),
    max_auth_ttl_seconds: getMaxAuthTTLSeconds(),
    server_synced: version > 0,
  }
}

export function buildSessionCalibrationHandoffSummary(input: {
  expiresAtIso?: string | null
  serverRemainingSec?: number | null
  checkedAtMs?: number | null
  nowMs?: number
}): SessionCalibrationHandoffSummary {
  const now = input.nowMs ?? Date.now()
  const exp = parseExpiresAt(input.expiresAtIso)
  if (exp == null) {
    return {
      has_local_expiry: false,
      local_remaining_seconds: null,
      server_remaining_seconds:
        typeof input.serverRemainingSec === 'number' ? Math.max(0, Math.floor(input.serverRemainingSec)) : null,
      server_checked_at:
        typeof input.checkedAtMs === 'number' && Number.isFinite(input.checkedAtMs)
          ? new Date(input.checkedAtMs).toISOString()
          : null,
      calibrated_remaining_seconds: null,
      urgency: 'unknown',
      calibration_source: 'unknown',
    }
  }
  const localMs = Math.max(0, exp - now)
  const localSec = Math.floor(localMs / 1000)
  const hasServer =
    typeof input.serverRemainingSec === 'number' &&
    Number.isFinite(input.serverRemainingSec) &&
    typeof input.checkedAtMs === 'number' &&
    Number.isFinite(input.checkedAtMs)
  const effectiveMs = effectiveSessionRemainingMs(
    localMs,
    input.serverRemainingSec,
    input.checkedAtMs,
    now,
  )
  const view = sessionViewFromRemainingMs(effectiveMs, true)
  return {
    has_local_expiry: true,
    local_remaining_seconds: localSec,
    server_remaining_seconds: hasServer
      ? Math.max(0, Math.floor(input.serverRemainingSec as number))
      : null,
    server_checked_at: hasServer ? new Date(input.checkedAtMs as number).toISOString() : null,
    calibrated_remaining_seconds: Math.floor(view.remainingMs / 1000),
    urgency: view.urgency,
    calibration_source: hasServer ? 'merged' : 'local_only',
  }
}

export function buildCommercialHandoffSummary(input?: {
  expiresAtIso?: string | null
  serverRemainingSec?: number | null
  checkedAtMs?: number | null
  nowMs?: number
}): CommercialHandoffSummary {
  return {
    password_policy: buildPasswordPolicyHandoffSummary(),
    session_calibration: buildSessionCalibrationHandoffSummary(input ?? {}),
  }
}

export function formatPasswordPolicyHandoffLine(p: PasswordPolicyHandoffSummary): string {
  const sync = p.server_synced ? `v${p.version}` : 'local-default'
  return `min_length: ${p.min_length} · forbidden: ${p.forbidden_defaults.join('|') || '—'} · ttl: ${p.default_auth_ttl_seconds}/${p.max_auth_ttl_seconds}s · ${sync}`
}

export function formatSessionCalibrationHandoffLine(s: SessionCalibrationHandoffSummary): string {
  if (!s.has_local_expiry) return 'no local expiry'
  const cal =
    s.calibrated_remaining_seconds == null
      ? '—'
      : formatRemaining(Math.max(0, s.calibrated_remaining_seconds) * 1000)
  const src = s.calibration_source
  return `urgency: ${s.urgency} · calibrated: ${cal} · source: ${src}`
}
