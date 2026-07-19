/** 路由离开脏检查：多页面注册 isDirty 源 */

export type DirtyChecker = () => boolean

const checkers = new Map<string, DirtyChecker>()

export function registerDirtyChecker(id: string, checker: DirtyChecker): () => void {
  checkers.set(id, checker)
  return () => {
    if (checkers.get(id) === checker) checkers.delete(id)
  }
}

export function clearDirtyCheckers(): void {
  checkers.clear()
}

export function anyRouteDirty(): boolean {
  for (const check of checkers.values()) {
    try {
      if (check()) return true
    } catch {
      /* ignore faulty checker */
    }
  }
  return false
}

export function dirtyCheckerCount(): number {
  return checkers.size
}

/** window.confirm 包装，便于单测注入 */
export function confirmLeaveDirty(
  message: string,
  confirmFn: (msg: string) => boolean = (msg) =>
    typeof window !== 'undefined' && typeof window.confirm === 'function'
      ? window.confirm(msg)
      : true,
): boolean {
  return confirmFn(message)
}

/**
 * 路由守卫辅助：若脏则 confirm；返回 true 允许离开。
 */
export function allowNavigationWhenDirty(
  isDirty: boolean,
  message: string,
  confirmFn?: (msg: string) => boolean,
): boolean {
  if (!isDirty) return true
  return confirmLeaveDirty(message, confirmFn)
}
