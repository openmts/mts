/** 标签页可见性恢复时是否应同步会话/网络 */

export function shouldSyncOnVisibility(
  visibilityState: string | null | undefined,
  documentHidden?: boolean | null,
): boolean {
  if (visibilityState === 'visible') return true
  if (visibilityState == null && documentHidden === false) return true
  return false
}
