/** admin_op_busy 轮询辅助（纯函数） */

import { formatElapsedSeconds } from './inFlightStatus.ts'

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
  payload: { admin_op_busy?: unknown; op?: unknown; started_at_unix?: unknown } | null | undefined,
): AdminOpStatus {
  const busy = Boolean(payload?.admin_op_busy)
  const op = typeof payload?.op === 'string' ? payload.op.trim() : ''
  const raw = payload?.started_at_unix
  const started =
    typeof raw === 'number' && Number.isFinite(raw) && raw > 0 ? Math.floor(raw) : null
  if (!busy) {
    return { busy: false, op: '', startedAtUnix: null }
  }
  return { busy: true, op, startedAtUnix: started }
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

/** 运维占用状态条深链（toast / 横幅统一） */
export const ADMIN_OP_BUSY_OPS_PATH = '/operations#ops-status-strip'

/** toast 快捷动作：打开运维状态条 */
export function adminOpBusyOpenAction(label: string): { label: string; path: string } {
  const lab = String(label || '').trim()
  return {
    label: lab || 'Open Operations',
    path: ADMIN_OP_BUSY_OPS_PATH,
  }
}

