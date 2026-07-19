import { ref } from 'vue'

export type NotifyKind = 'success' | 'error' | 'info'

export interface NotifyItem {
  id: number
  kind: NotifyKind
  message: string
}

const items = ref<NotifyItem[]>([])
let seq = 1

export function useNotify() {
  function push(kind: NotifyKind, message: string, ttlMs = 3200) {
    const id = seq++
    items.value = [...items.value, { id, kind, message }]
    if (ttlMs > 0) {
      window.setTimeout(() => {
        items.value = items.value.filter((n) => n.id !== id)
      }, ttlMs)
    }
  }

  function success(message: string) { push('success', message) }
  function error(message: string) { push('error', message, 5000) }
  function info(message: string) { push('info', message) }
  function dismiss(id: number) {
    items.value = items.value.filter((n) => n.id !== id)
  }

  return { items, success, error, info, dismiss, push }
}
