/** 路由离开脏检查：多页面注册 isDirty 源 */

export type DirtyChecker = () => boolean

/** form=未提交服务端草稿；local=本地已自动落盘的会话变更 */
export type DirtyKind = 'form' | 'local'

interface DirtyRegistration {
  checker: DirtyChecker
  kind: DirtyKind
}

const checkers = new Map<string, DirtyRegistration>()

export function registerDirtyChecker(
  id: string,
  checker: DirtyChecker,
  kind: DirtyKind = 'form',
): () => void {
  checkers.set(id, { checker, kind })
  return () => {
    const cur = checkers.get(id)
    if (cur?.checker === checker) checkers.delete(id)
  }
}

export function clearDirtyCheckers(): void {
  checkers.clear()
}

export function anyRouteDirty(): boolean {
  for (const { checker } of checkers.values()) {
    try {
      if (checker()) return true
    } catch {
      /* ignore faulty checker */
    }
  }
  return false
}

/** 当前脏源中是否包含 local 类（用于离开文案分流） */
export function anyLocalRouteDirty(): boolean {
  for (const { checker, kind } of checkers.values()) {
    if (kind !== 'local') continue
    try {
      if (checker()) return true
    } catch {
      /* ignore */
    }
  }
  return false
}

/** 当前脏源中是否包含 form 类 */
export function anyFormRouteDirty(): boolean {
  for (const { checker, kind } of checkers.values()) {
    if (kind !== 'form') continue
    try {
      if (checker()) return true
    } catch {
      /* ignore */
    }
  }
  return false
}

/**
 * 选择离开确认文案：有 form 脏优先「未保存」；仅 local 脏用本地落盘提示。
 */
export function leaveDirtyMessage(
  messages: { unsavedLeaveConfirm: string; localDirtyLeaveConfirm: string },
): string {
  if (anyFormRouteDirty()) return messages.unsavedLeaveConfirm
  if (anyLocalRouteDirty()) return messages.localDirtyLeaveConfirm
  return messages.unsavedLeaveConfirm
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
