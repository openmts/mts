/** 运维/管理页统一动作结果 */

export type ActionResultKind = 'ok' | 'error' | 'warn' | 'info'

export interface ActionResult {
  kind: ActionResultKind
  message: string
  at: number
}

export function makeActionResult(
  kind: ActionResultKind,
  message: string,
  at = Date.now(),
): ActionResult {
  return { kind, message: String(message || '').trim() || '—', at }
}

export function actionResultClass(kind: ActionResultKind): string {
  switch (kind) {
    case 'ok':
      return 'mts-alert-ok'
    case 'error':
      return 'mts-alert-error'
    case 'warn':
      return 'mts-alert-warn'
    default:
      return 'mts-alert-info'
  }
}

export function actionResultLabel(kind: ActionResultKind): string {
  switch (kind) {
    case 'ok':
      return '成功'
    case 'error':
      return '失败'
    case 'warn':
      return '警告'
    default:
      return '信息'
  }
}
