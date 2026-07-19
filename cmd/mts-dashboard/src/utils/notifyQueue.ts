/** 全局通知队列：容量、去重、warn 支持（纯函数） */

export type NotifyKind = 'success' | 'error' | 'info' | 'warn'

export interface NotifyItem {
  id: number
  kind: NotifyKind
  message: string
  count: number
  createdAt: number
  updatedAt: number
}

export const DEFAULT_NOTIFY_CAPACITY = 5
export const DEFAULT_DEDUPE_WINDOW_MS = 4000

export interface PushNotifyInput {
  kind: NotifyKind
  message: string
  now?: number
  capacity?: number
  dedupeWindowMs?: number
  nextId: number
}

export interface PushNotifyResult {
  items: NotifyItem[]
  nextId: number
  /** 被更新的 id（合并）或新建 id */
  id: number
  merged: boolean
}

function normalizeMessage(message: string): string {
  return String(message || '').trim() || '—'
}

/** 推入通知：同 kind+message 在窗口内合并 count++ */
export function pushNotifyItem(
  items: NotifyItem[],
  input: PushNotifyInput,
): PushNotifyResult {
  const now = input.now ?? Date.now()
  const capacity = Math.max(1, input.capacity ?? DEFAULT_NOTIFY_CAPACITY)
  const windowMs = Math.max(0, input.dedupeWindowMs ?? DEFAULT_DEDUPE_WINDOW_MS)
  const message = normalizeMessage(input.message)
  const kind = input.kind

  let nextId = input.nextId
  let merged = false
  let id = nextId
  let next = items.slice()

  const existingIdx = next.findIndex(
    (n) => n.kind === kind && n.message === message && now - n.updatedAt <= windowMs,
  )
  if (existingIdx >= 0) {
    const cur = next[existingIdx]
    const updated: NotifyItem = {
      ...cur,
      count: cur.count + 1,
      updatedAt: now,
    }
    next = [...next.slice(0, existingIdx), updated, ...next.slice(existingIdx + 1)]
    id = cur.id
    merged = true
  } else {
    id = nextId++
    next = [
      ...next,
      {
        id,
        kind,
        message,
        count: 1,
        createdAt: now,
        updatedAt: now,
      },
    ]
  }

  if (next.length > capacity) {
    // 丢弃最旧（按 createdAt）
    next = [...next].sort((a, b) => a.createdAt - b.createdAt).slice(next.length - capacity)
    // 保持相对更新顺序：按 updatedAt 再排一次展示
    next = next.sort((a, b) => a.updatedAt - b.updatedAt)
  }

  return { items: next, nextId, id, merged }
}

export function dismissNotifyItem(items: NotifyItem[], id: number): NotifyItem[] {
  return items.filter((n) => n.id !== id)
}

export function notifyDisplayText(item: NotifyItem): string {
  if (item.count > 1) return `${item.message} (×${item.count})`
  return item.message
}

export function defaultTtlMs(kind: NotifyKind): number {
  switch (kind) {
    case 'error':
      return 6000
    case 'warn':
      return 5000
    case 'success':
      return 3200
    default:
      return 3600
  }
}
