import { ref } from 'vue'
import type { QueryMode } from '@/composables/useQueryWorkbench'

const KEY = 'mts_query_history'
const MAX = 30

export interface QueryHistoryItem {
  id: string
  at: number
  mode: QueryMode
  form: {
    database: string
    retention_policy: string
    measurement: string
    start_time: string
    end_time: string
    fields: string
    limit: string
  }
}

function load(): QueryHistoryItem[] {
  try {
    const raw = localStorage.getItem(KEY)
    if (!raw) return []
    const arr = JSON.parse(raw) as QueryHistoryItem[]
    return Array.isArray(arr) ? arr : []
  } catch {
    return []
  }
}

const items = ref<QueryHistoryItem[]>(load())

function persist() {
  try { localStorage.setItem(KEY, JSON.stringify(items.value)) } catch { /* ignore */ }
}

export function useQueryHistory() {
  function push(item: Omit<QueryHistoryItem, 'id' | 'at'>) {
    const next: QueryHistoryItem = {
      ...item,
      id: `${Date.now()}-${Math.random().toString(36).slice(2, 8)}`,
      at: Date.now(),
    }
    items.value = [next, ...items.value].slice(0, MAX)
    persist()
  }
  function clear() {
    items.value = []
    persist()
  }
  function remove(id: string) {
    items.value = items.value.filter((x) => x.id !== id)
    persist()
  }
  return { items, push, clear, remove }
}
