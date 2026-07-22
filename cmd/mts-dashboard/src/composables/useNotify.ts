import { ref } from 'vue'
import {
  DEFAULT_NOTIFY_CAPACITY,
  DEFAULT_DEDUPE_WINDOW_MS,
  defaultTtlMs,
  dismissNotifyItem,
  pushNotifyItem,
  type NotifyAction,
  type NotifyItem,
  type NotifyKind,
} from '@/utils/notifyQueue'
import {
  clearNotifyHistory,
  loadNotifyHistory,
  recordNotifyHistory,
  type NotifyHistoryEntry,
} from '@/utils/notifyHistory'

const items = ref<NotifyItem[]>([])
const history = ref<NotifyHistoryEntry[]>(loadNotifyHistory())
let seq = 1
const timers = new Map<number, ReturnType<typeof setTimeout>>()

function clearTimer(id: number) {
  const t = timers.get(id)
  if (t) {
    clearTimeout(t)
    timers.delete(id)
  }
}

function scheduleDismiss(id: number, ttlMs: number) {
  clearTimer(id)
  if (ttlMs <= 0) return
  const handle = setTimeout(() => {
    timers.delete(id)
    items.value = dismissNotifyItem(items.value, id)
  }, ttlMs)
  timers.set(id, handle)
}

function refreshHistory() {
  history.value = loadNotifyHistory()
}

export function useNotify() {
  function push(
    kind: NotifyKind,
    message: string,
    ttlOrOpts?: number | { ttlMs?: number; action?: NotifyAction },
  ) {
    const opts = typeof ttlOrOpts === 'object' && ttlOrOpts != null ? ttlOrOpts : undefined
    const ttlMs =
      typeof ttlOrOpts === 'number'
        ? ttlOrOpts
        : opts?.ttlMs != null
          ? opts.ttlMs
          : defaultTtlMs(kind)
    const result = pushNotifyItem(items.value, {
      kind,
      message,
      nextId: seq,
      capacity: DEFAULT_NOTIFY_CAPACITY,
      dedupeWindowMs: DEFAULT_DEDUPE_WINDOW_MS,
      action: opts?.action,
    })
    seq = result.nextId
    items.value = result.items
    const saved = items.value.find((x) => x.id === result.id)
    // 新建 toast 记入历史；窗口内合并不重复刷历史
    if (saved && !result.merged) {
      history.value = recordNotifyHistory({
        kind: saved.kind,
        message: saved.message,
        count: saved.count,
        at: saved.updatedAt,
        actionLabel: saved.action?.label,
        actionPath: saved.action?.path,
      })
    }
    scheduleDismiss(result.id, ttlMs)
    return result.id
  }

  function success(message: string) {
    return push('success', message)
  }
  function error(
    message: string,
    opts?: { action?: NotifyAction; ttlMs?: number },
  ) {
    return push('error', message, {
      ttlMs: opts?.ttlMs ?? defaultTtlMs('error'),
      action: opts?.action,
    })
  }
  function warn(
    message: string,
    opts?: { action?: NotifyAction; ttlMs?: number },
  ) {
    return push('warn', message, {
      ttlMs: opts?.ttlMs ?? defaultTtlMs('warn'),
      action: opts?.action,
    })
  }
  function info(message: string) {
    return push('info', message)
  }
  function dismiss(id: number) {
    clearTimer(id)
    items.value = dismissNotifyItem(items.value, id)
  }

  function clearHistory() {
    clearNotifyHistory()
    history.value = []
  }

  function reloadHistory() {
    refreshHistory()
  }

  return {
    items,
    history,
    success,
    error,
    warn,
    info,
    dismiss,
    push,
    clearHistory,
    reloadHistory,
  }
}

// 兼容旧类型导出
export type { NotifyAction, NotifyItem, NotifyKind }
