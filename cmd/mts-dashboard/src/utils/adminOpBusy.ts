/** admin_op_busy 轮询辅助（纯函数） */

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
