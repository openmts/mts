/** TopBar 连通性 badge 样式与文案 key */

export type ConnectivityKind = 'ok' | 'unreachable' | 'offline' | 'unknown' | string

export function connectivityBadgeClass(kind: ConnectivityKind): string {
  switch (kind) {
    case 'ok':
      return 'bg-emerald-50 text-emerald-800 dark:bg-emerald-950/40 dark:text-emerald-200'
    case 'unreachable':
      return 'bg-red-100 text-red-800 dark:bg-red-950/50 dark:text-red-200'
    case 'offline':
      return 'bg-amber-100 text-amber-900 dark:bg-amber-950/50 dark:text-amber-100'
    default:
      return 'bg-slate-100 text-slate-600 dark:bg-slate-800 dark:text-slate-300'
  }
}

export function connectivityBadgeLabelKey(
  kind: ConnectivityKind,
): 'connectivityOk' | 'connectivityUnreachable' | 'connectivityOffline' | 'connectivityUnknown' {
  switch (kind) {
    case 'ok':
      return 'connectivityOk'
    case 'unreachable':
      return 'connectivityUnreachable'
    case 'offline':
      return 'connectivityOffline'
    default:
      return 'connectivityUnknown'
  }
}

export function sessionUrgencyBadgeClass(urgency: string): string {
  switch (urgency) {
    case 'critical':
      return 'bg-red-100 text-red-700 dark:bg-red-950/50 dark:text-red-200'
    case 'warn':
      return 'bg-amber-100 text-amber-800 dark:bg-amber-950/50 dark:text-amber-200'
    case 'expired':
      return 'bg-red-200 text-red-900 dark:bg-red-900 dark:text-red-100'
    case 'ok':
      return 'bg-emerald-50 text-emerald-800 dark:bg-emerald-950/40 dark:text-emerald-200'
    default:
      return 'bg-slate-100 text-slate-600 dark:bg-slate-800 dark:text-slate-300'
  }
}
