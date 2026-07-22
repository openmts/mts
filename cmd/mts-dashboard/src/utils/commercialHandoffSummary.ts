/** 可商用交接摘要：密码策略 + 会话校准 + 数据面契约（纯函数，供就绪归档/验收包） */

import {
  buildDataContractView,
  formatDataContractHandoffLine,
  type DataContractInput,
  type DataContractView,
} from './dataContractView.ts'
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

export type SessionSampleSource = 'login' | 'session'

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
  server_time_unix: number | null
  /** 客户端校验时刻 - 服务端时间（秒）；正值表示本机时钟偏快 */
  clock_skew_seconds: number | null
  calibrated_remaining_seconds: number | null
  urgency: SessionUrgency
  /**
   * remaining 合成来源：
   * - login/session：有服务端 remaining 样本（样本端点见 sample_source）
   * - local_only：仅本地 expires
   * - unknown：无本地 expires
   */
  calibration_source: 'local_only' | 'login' | 'session' | 'merged' | 'unknown'
  /** 服务端样本端点来源（login 响应 vs GET /auth/session） */
  sample_source: SessionSampleSource | null
}

export interface CommercialHandoffSummary {
  password_policy: PasswordPolicyHandoffSummary
  session_calibration: SessionCalibrationHandoffSummary
  /** 数据面契约快照（limits + write/query/stream/delete meta 能力） */
  data_contract: DataContractView
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

function resolveServerTimeUnix(input: {
  serverTimeUnix?: number | null
  checkedAtMs?: number | null
}): { serverTimeUnix: number | null; clockSkewSeconds: number | null } {
  const hasServerTime =
    typeof input.serverTimeUnix === 'number' && Number.isFinite(input.serverTimeUnix)
  const hasChecked =
    typeof input.checkedAtMs === 'number' && Number.isFinite(input.checkedAtMs)
  if (!hasServerTime) return { serverTimeUnix: null, clockSkewSeconds: null }
  const serverTimeUnix = Math.floor(input.serverTimeUnix as number)
  if (!hasChecked) return { serverTimeUnix, clockSkewSeconds: null }
  return {
    serverTimeUnix,
    clockSkewSeconds: Math.round((input.checkedAtMs as number) / 1000 - serverTimeUnix),
  }
}

function resolveSampleSource(
  sampleSource: SessionSampleSource | null | undefined,
  hasServer: boolean,
): SessionSampleSource | null {
  if (!hasServer) return null
  if (sampleSource === 'login' || sampleSource === 'session') return sampleSource
  return 'session'
}

function resolveCalibrationSource(
  hasServer: boolean,
  sampleSource: SessionSampleSource | null,
): SessionCalibrationHandoffSummary['calibration_source'] {
  if (!hasServer) return 'local_only'
  if (sampleSource === 'login') return 'login'
  if (sampleSource === 'session') return 'session'
  return 'merged'
}

function emptySessionCalibration(
  input: {
    serverRemainingSec?: number | null
    checkedAtMs?: number | null
    sampleSource?: SessionSampleSource | null
  },
  timeMeta: { serverTimeUnix: number | null; clockSkewSeconds: number | null },
): SessionCalibrationHandoffSummary {
  const hasServer =
    typeof input.serverRemainingSec === 'number' && Number.isFinite(input.serverRemainingSec)
  const sample = resolveSampleSource(input.sampleSource, hasServer)
  return {
    has_local_expiry: false,
    local_remaining_seconds: null,
    server_remaining_seconds: hasServer
      ? Math.max(0, Math.floor(input.serverRemainingSec as number))
      : null,
    server_checked_at:
      typeof input.checkedAtMs === 'number' && Number.isFinite(input.checkedAtMs)
        ? new Date(input.checkedAtMs).toISOString()
        : null,
    server_time_unix: timeMeta.serverTimeUnix,
    clock_skew_seconds: timeMeta.clockSkewSeconds,
    calibrated_remaining_seconds: null,
    urgency: 'unknown',
    calibration_source: 'unknown',
    sample_source: sample,
  }
}

function filledSessionCalibration(
  exp: number,
  input: {
    serverRemainingSec?: number | null
    checkedAtMs?: number | null
    sampleSource?: SessionSampleSource | null
  },
  timeMeta: { serverTimeUnix: number | null; clockSkewSeconds: number | null },
  now: number,
): SessionCalibrationHandoffSummary {
  const localMs = Math.max(0, exp - now)
  const hasServer =
    typeof input.serverRemainingSec === 'number' &&
    Number.isFinite(input.serverRemainingSec) &&
    typeof input.checkedAtMs === 'number' &&
    Number.isFinite(input.checkedAtMs)
  const sample = resolveSampleSource(input.sampleSource, hasServer)
  const view = sessionViewFromRemainingMs(
    effectiveSessionRemainingMs(localMs, input.serverRemainingSec, input.checkedAtMs, now),
    true,
  )
  return {
    has_local_expiry: true,
    local_remaining_seconds: Math.floor(localMs / 1000),
    server_remaining_seconds: hasServer
      ? Math.max(0, Math.floor(input.serverRemainingSec as number))
      : null,
    server_checked_at: hasServer ? new Date(input.checkedAtMs as number).toISOString() : null,
    server_time_unix: timeMeta.serverTimeUnix,
    clock_skew_seconds: timeMeta.clockSkewSeconds,
    calibrated_remaining_seconds: Math.floor(view.remainingMs / 1000),
    urgency: view.urgency,
    calibration_source: resolveCalibrationSource(hasServer, sample),
    sample_source: sample,
  }
}

export function buildSessionCalibrationHandoffSummary(input: {
  expiresAtIso?: string | null
  serverRemainingSec?: number | null
  checkedAtMs?: number | null
  serverTimeUnix?: number | null
  sampleSource?: SessionSampleSource | null
  nowMs?: number
}): SessionCalibrationHandoffSummary {
  const now = input.nowMs ?? Date.now()
  const exp = parseExpiresAt(input.expiresAtIso)
  const timeMeta = resolveServerTimeUnix(input)
  if (exp == null) return emptySessionCalibration(input, timeMeta)
  return filledSessionCalibration(exp, input, timeMeta, now)
}

export function buildCommercialHandoffSummary(input?: {
  expiresAtIso?: string | null
  serverRemainingSec?: number | null
  checkedAtMs?: number | null
  serverTimeUnix?: number | null
  sampleSource?: SessionSampleSource | null
  nowMs?: number
  dataContract?: DataContractInput | null
}): CommercialHandoffSummary {
  return {
    password_policy: buildPasswordPolicyHandoffSummary(),
    session_calibration: buildSessionCalibrationHandoffSummary(input ?? {}),
    data_contract: buildDataContractView(input?.dataContract ?? null),
  }
}

export function formatPasswordPolicyHandoffLine(p: PasswordPolicyHandoffSummary): string {
  const sync = p.server_synced ? `v${p.version}` : 'local-default'
  return `min_length: ${p.min_length} · forbidden: ${p.forbidden_defaults.join('|') || '—'} · ttl: ${p.default_auth_ttl_seconds}/${p.max_auth_ttl_seconds}s · ${sync}`
}

export function formatSessionCalibrationHandoffLine(s: SessionCalibrationHandoffSummary): string {
  if (!s.has_local_expiry) {
    const skewOnly =
      s.clock_skew_seconds == null
        ? ''
        : ` · skew: ${s.clock_skew_seconds >= 0 ? '+' : ''}${s.clock_skew_seconds}s`
    const sample = s.sample_source ? ` · sample: ${s.sample_source}` : ''
    return `no local expiry${sample}${skewOnly}`
  }
  const cal =
    s.calibrated_remaining_seconds == null
      ? '—'
      : formatRemaining(Math.max(0, s.calibrated_remaining_seconds) * 1000)
  const skew =
    s.clock_skew_seconds == null
      ? ''
      : ` · skew: ${s.clock_skew_seconds >= 0 ? '+' : ''}${s.clock_skew_seconds}s`
  const sample = s.sample_source ? ` · sample: ${s.sample_source}` : ''
  return `urgency: ${s.urgency} · calibrated: ${cal} · source: ${s.calibration_source}${sample}${skew}`
}

export { formatDataContractHandoffLine }
