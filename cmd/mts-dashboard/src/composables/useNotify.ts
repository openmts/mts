import { ref } from 'vue'
import {
  DEFAULT_NOTIFY_CAPACITY,
  DEFAULT_DEDUPE_WINDOW_MS,
  defaultTtlMs,
  dismissNotifyItem,
  pushNotifyItem,
  type NotifyItem,
  type NotifyKind,
} from '@/utils/notifyQueue'

const items = ref<NotifyItem[]>([])
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

export function useNotify() {
  function push(kind: NotifyKind, message: string, ttlMs = defaultTtlMs(kind)) {
    const result = pushNotifyItem(items.value, {
      kind,
      message,
      nextId: seq,
      capacity: DEFAULT_NOTIFY_CAPACITY,
      dedupeWindowMs: DEFAULT_DEDUPE_WINDOW_MS,
    })
    seq = result.nextId
    items.value = result.items
    scheduleDismiss(result.id, ttlMs)
    return result.id
  }

  function success(message: string) {
    return push('success', message)
  }
  function error(message: string) {
    return push('error', message, defaultTtlMs('error'))
  }
  function warn(message: string) {
    return push('warn', message, defaultTtlMs('warn'))
  }
  function info(message: string) {
    return push('info', message)
  }
  function dismiss(id: number) {
    clearTimer(id)
    items.value = dismissNotifyItem(items.value, id)
  }

  return { items, success, error, warn, info, dismiss, push }
}

// 兼容旧类型导出
export type { NotifyItem, NotifyKind }
