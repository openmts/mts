/** admin_op_busy 轮询辅助（纯函数） */

import { formatElapsedSeconds } from './inFlightStatus.ts'
import {
  type AdminHeavyLast,
  formatAdminHeavyLastSummary,
  formatAdminHeavyLastDetail,
  formatAdminHeavyLastCopyText,
  parseAdminHeavyLast,
  readDismissedAdminOpLastFinishedAt,
  readFailAckedAdminOpLastFinishedAt,
  shouldShowAdminOpLastBanner,
  writeDismissedAdminOpLastFinishedAt,
  writeFailAckedAdminOpLastFinishedAt,
  adminOpLastToneClass,
  adminOpLastBannerSurfaceClass,
  adminOpLastChipSurfaceClass,
  canDismissAdminOpLast,
  commandAdminOpLastDismissFeedback,
  commandAdminOpLastCopyFeedback,
} from './adminOpLast.ts'

export type { AdminHeavyLast, CommandAdminOpLastDismissFeedback, CommandAdminOpLastCopyFeedback } from './adminOpLast.ts'
export {
  formatAdminHeavyLastSummary,
  formatAdminHeavyLastDetail,
  formatAdminHeavyLastCopyText,
  parseAdminHeavyLast,
  readDismissedAdminOpLastFinishedAt,
  readFailAckedAdminOpLastFinishedAt,
  shouldShowAdminOpLastBanner,
  writeDismissedAdminOpLastFinishedAt,
  writeFailAckedAdminOpLastFinishedAt,
  adminOpLastToneClass,
  adminOpLastBannerSurfaceClass,
  adminOpLastChipSurfaceClass,
  canDismissAdminOpLast,
  commandAdminOpLastDismissFeedback,
  commandAdminOpLastCopyFeedback,
} from './adminOpLast.ts'

export type AdminOpKind =
  | 'flush'
  | 'compact'
  | 'retention'
  | 'config_snapshot'
  | 'data_snapshot'
  | 'restore_drill'
  | 'maintenance'
  | 'admin_heavy'
  | 'test'
  | string

export interface AdminOpStatus {
  busy: boolean
  op: string
  startedAtUnix: number | null
  last: AdminHeavyLast | null
}

export function shouldPollAdminOpBusy(
  isAuthenticated: boolean | null | undefined,
  isAdmin: boolean | null | undefined,
): boolean {
  return Boolean(isAuthenticated) && Boolean(isAdmin)
}

export function parseAdminOpBusyPayload(payload: { admin_op_busy?: unknown } | null | undefined): boolean {
  return Boolean(payload?.admin_op_busy)
}

export function parseAdminOpStatusPayload(
  payload: {
    admin_op_busy?: unknown
    op?: unknown
    started_at_unix?: unknown
    last?: unknown
  } | null | undefined,
): AdminOpStatus {
  const busy = Boolean(payload?.admin_op_busy)
  const op = typeof payload?.op === 'string' ? payload.op.trim() : ''
  const raw = payload?.started_at_unix
  const started =
    typeof raw === 'number' && Number.isFinite(raw) && raw > 0 ? Math.floor(raw) : null
  const last = parseAdminHeavyLast(payload?.last)
  if (!busy) {
    return { busy: false, op: '', startedAtUnix: null, last }
  }
  return { busy: true, op, startedAtUnix: started, last }
}



/** i18n key for known ops; unknown falls back to generic banner body */
export function adminOpKindLabelKey(op: string | null | undefined): string {
  switch ((op || '').trim()) {
    case 'flush':
      return 'adminOpKindFlush'
    case 'compact':
      return 'adminOpKindCompact'
    case 'retention':
      return 'adminOpKindRetention'
    case 'config_snapshot':
      return 'adminOpKindConfigSnapshot'
    case 'data_snapshot':
      return 'adminOpKindDataSnapshot'
    case 'restore_drill':
      return 'adminOpKindRestoreDrill'
    case 'maintenance':
      return 'adminOpKindMaintenance'
    default:
      return 'adminOpKindGeneric'
  }
}

/** started_at_unix（秒）→ 人类可读耗时；无效返回 em dash */
export function formatAdminOpElapsed(
  startedAtUnix: number | null | undefined,
  nowMs: number = Date.now(),
): string {
  if (startedAtUnix == null || !Number.isFinite(startedAtUnix) || startedAtUnix <= 0) return '—'
  const ms = Math.max(0, nowMs - Math.floor(startedAtUnix) * 1000)
  return formatElapsedSeconds(ms)
}

/** chip 文案：base 或 base: opLabel */
export function joinAdminOpChip(base: string, opLabel?: string | null): string {
  const label = (opLabel || '').trim()
  if (!label) return base
  return `${base}: ${label}`
}

/** ops-status 轮询失败退避阶梯（ms） */
export const ADMIN_OP_POLL_FAIL_BACKOFF_MS = [5_000, 10_000, 20_000, 30_000] as const
export const ADMIN_OP_POLL_IDLE_MS = 15_000
export const ADMIN_OP_POLL_BUSY_MS = 3_000
export const ADMIN_OP_POLL_MIN_MS = 2_000

/** 失败 streak → 下次轮询间隔；成功时 streak=0 回 idle/busy */
export function adminOpPollIntervalMs(opts: {
  failStreak: number
  busy: boolean
  idleMs?: number
  busyMs?: number
}): number {
  const idleMs = Math.max(ADMIN_OP_POLL_MIN_MS, opts.idleMs ?? ADMIN_OP_POLL_IDLE_MS)
  const busyMs = Math.max(
    ADMIN_OP_POLL_MIN_MS,
    Math.min(opts.busyMs ?? ADMIN_OP_POLL_BUSY_MS, idleMs),
  )
  const streak = Math.max(0, Math.trunc(opts.failStreak || 0))
  if (streak > 0) {
    const idx = Math.min(streak, ADMIN_OP_POLL_FAIL_BACKOFF_MS.length) - 1
    return ADMIN_OP_POLL_FAIL_BACKOFF_MS[Math.max(0, idx)]
  }
  return opts.busy ? busyMs : idleMs
}

export function nextAdminOpFailStreak(current: number, ok: boolean): number {
  if (ok) return 0
  return Math.min(ADMIN_OP_POLL_FAIL_BACKOFF_MS.length, Math.max(0, Math.trunc(current || 0)) + 1)
}

/** 服务端 admin heavy 互斥错误文案识别（resource_exhausted） */
export function isAdminHeavyBusyMessage(message: string | null | undefined): boolean {
  const m = String(message || '').toLowerCase()
  if (!m) return false
  return (
    m.includes('admin heavy') ||
    m.includes('already in progress') ||
    m.includes('engine busy') ||
    m.includes('管理重操作') ||
    m.includes('运维占用')
  )
}

export function isAdminHeavyBusyError(err: unknown): boolean {
  if (!err || typeof err !== 'object') return false
  const e = err as {
    code?: string
    message?: string
    status?: number
    name?: string
    adminOpBusy?: boolean
    admin_op_busy?: boolean
    op?: string
  }
  if (e.adminOpBusy || e.admin_op_busy) return true
  const code = String(e.code || '').toLowerCase()
  if (code !== 'resource_exhausted' && e.status !== 429) return false
  if ((e.op || '').trim()) return true
  return isAdminHeavyBusyMessage(e.message)
}

/** 从互斥错误 message 解析当前占用 op（如 "... in progress: flush"） */
export function parseAdminHeavyBusyOp(message: string | null | undefined): string {
  const m = String(message || '').trim()
  if (!m) return ''
  const idx = m.toLowerCase().lastIndexOf('in progress:')
  if (idx >= 0) {
    return m.slice(idx + 'in progress:'.length).trim()
  }
  const colon = m.lastIndexOf(':')
  if (colon >= 0 && /admin heavy|already in progress/i.test(m)) {
    const tail = m.slice(colon + 1).trim()
    if (tail && !/\s/.test(tail) && tail.length < 40) return tail
  }
  return ''
}

export function adminHeavyBusyOpFromError(err: unknown): string {
  if (!isAdminHeavyBusyError(err)) return ''
  const e = err as { message?: string; op?: string }
  const structured = String(e.op || '').trim()
  if (structured) return structured
  return parseAdminHeavyBusyOp(e.message)
}

export type AdminBusyNotifyDecision =
  | { kind: 'admin_busy'; op: string }
  | { kind: 'plain' }

/** 是否用 admin-busy toast（含 action / 乐观置 busy） */
export function resolveAdminBusyNotify(
  err?: unknown,
  localBusy = false,
): AdminBusyNotifyDecision {
  if (err != null && isAdminHeavyBusyError(err)) {
    return { kind: 'admin_busy', op: adminHeavyBusyOpFromError(err) }
  }
  if (localBusy) {
    return { kind: 'admin_busy', op: '' }
  }
  return { kind: 'plain' }
}

/** 运维占用状态条深链（toast / 横幅统一） */
export const ADMIN_OP_BUSY_OPS_PATH = '/operations#ops-status-strip'

/** 从 HTTP 响应头解析 admin busy（大小写不敏感） */
export function parseAdminBusyFromHeaders(getHeader: (name: string) => string | null | undefined): {
  busy: boolean
  op: string
} {
  const busyRaw = String(
    getHeader('X-MTS-Admin-Op-Busy') || getHeader('x-mts-admin-op-busy') || '',
  )
    .trim()
    .toLowerCase()
  const busy = busyRaw === 'true' || busyRaw === '1' || busyRaw === 'yes'
  const op = String(getHeader('X-MTS-Admin-Op') || getHeader('x-mts-admin-op') || '').trim()
  return { busy, op }
}


/** toast 快捷动作：打开运维状态条 */
export function adminOpBusyOpenAction(label: string): { label: string; path: string } {
  const lab = String(label || '').trim()
  return {
    label: lab || 'Open Operations',
    path: ADMIN_OP_BUSY_OPS_PATH,
  }
}

/** 供 useNotify.error 的 action 选项（调用方传入已翻译 label） */
export function buildAdminBusyNotifyOptions(label: string): { action: { label: string; path: string } } {
  return { action: adminOpBusyOpenAction(label) }
}

export type CommandAdminOpRefreshFeedback =
  | { kind: 'denied' }
  | { kind: 'ok' }
  | { kind: 'error'; message: string; adminBusy: boolean }

/** 命令面板「刷新管理占用」结果反馈（纯函数） */
export function commandAdminOpRefreshFeedback(opts: {
  isAdmin: boolean
  errorMessage?: string | null
  error?: unknown
}): CommandAdminOpRefreshFeedback {
  if (!opts.isAdmin) return { kind: 'denied' }
  const msg = String(opts.errorMessage || '').trim()
  if (!msg && opts.error == null) return { kind: 'ok' }
  const err = opts.error ?? (msg ? { message: msg, code: 'internal' } : null)
  const busy = err != null && resolveAdminBusyNotify(err).kind === 'admin_busy'
  return {
    kind: 'error',
    message: msg || (err instanceof Error ? err.message : String(err || 'error')),
    adminBusy: busy,
  }
}

