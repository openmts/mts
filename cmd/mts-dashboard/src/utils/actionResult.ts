/** 运维/管理页统一动作结果 */

export type ActionResultKind = 'ok' | 'error' | 'warn' | 'info'
export type ActionResultLocale = 'zh' | 'en'

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

/** 种类标签：须传 locale，避免英文界面泄漏中文硬编码 */
export function actionResultLabel(
  kind: ActionResultKind,
  locale: ActionResultLocale = 'zh',
): string {
  if (locale === 'en') {
    switch (kind) {
      case 'ok':
        return 'Success'
      case 'error':
        return 'Failed'
      case 'warn':
        return 'Warning'
      default:
        return 'Info'
    }
  }
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
