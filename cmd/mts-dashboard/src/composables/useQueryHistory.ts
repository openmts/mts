import { computed, ref } from 'vue'
import type { QueryMode } from '@/composables/useQueryWorkbench'
import {
  historyItemTitle,
  mergeHistoryCap,
  normalizeHistoryItems,
  sortHistoryItems,
  type QueryHistoryForm,
  type QueryHistoryRecord,
} from '@/utils/queryHistory'
import {
  buildHistoryExport,
  mergeImportedHistory,
  parseHistoryImport,
  type HistoryExportPayload,
} from '@/utils/queryHistoryIO'

const KEY = 'mts_query_history'
/** 查询历史上限（与通知/运维日志对齐） */
export const QUERY_HISTORY_MAX = 200
const MAX = QUERY_HISTORY_MAX

export type { QueryHistoryForm }
export interface QueryHistoryItem extends QueryHistoryRecord {
  mode: QueryMode
}

function load(): QueryHistoryItem[] {
  try {
    const raw = localStorage.getItem(KEY)
    if (!raw) return []
    const arr = normalizeHistoryItems(JSON.parse(raw)) as QueryHistoryItem[]
    return sortHistoryItems(arr)
  } catch {
    return []
  }
}

const items = ref<QueryHistoryItem[]>(load())

function persist() {
  try {
    localStorage.setItem(KEY, JSON.stringify(items.value))
  } catch {
    /* ignore quota / private mode */
  }
}

export function useQueryHistory() {
  const sortedItems = computed(() => sortHistoryItems(items.value))

  function push(
    item: Omit<QueryHistoryItem, 'id' | 'at'> & { name?: string; pinned?: boolean },
  ) {
    const next: QueryHistoryItem = {
      ...item,
      name: item.name?.trim() || undefined,
      pinned: Boolean(item.pinned),
      id: `${Date.now()}-${Math.random().toString(36).slice(2, 8)}`,
      at: Date.now(),
    }
    items.value = mergeHistoryCap([next, ...items.value], MAX)
    persist()
  }

  function clear(opts?: { keepPinned?: boolean }) {
    if (opts?.keepPinned) {
      items.value = items.value.filter((x) => x.pinned)
    } else {
      items.value = []
    }
    persist()
  }

  function remove(id: string) {
    items.value = items.value.filter((x) => x.id !== id)
    persist()
  }

  function rename(id: string, name: string) {
    const n = name.trim()
    items.value = items.value.map((x) =>
      x.id === id ? { ...x, name: n || undefined } : x,
    )
    persist()
  }

  function togglePin(id: string) {
    items.value = sortHistoryItems(
      items.value.map((x) => (x.id === id ? { ...x, pinned: !x.pinned } : x)),
    )
    persist()
  }

  function titleOf(item: QueryHistoryItem): string {
    return historyItemTitle(item)
  }

  function exportPayload(): HistoryExportPayload {
    return buildHistoryExport(items.value)
  }

  function importPayload(
    raw: unknown,
    opts?: { merge?: boolean },
  ): { ok: true; count: number } | { ok: false; error: string } {
    const parsed = parseHistoryImport(raw)
    if (!parsed.ok) return parsed
    const merge = opts?.merge !== false
    const next = mergeImportedHistory(items.value, parsed.items as QueryHistoryRecord[], {
      merge,
      max: MAX,
    }) as QueryHistoryItem[]
    items.value = next
    persist()
    return { ok: true, count: parsed.items.length }
  }

  return {
    items: sortedItems,
    push,
    clear,
    remove,
    rename,
    togglePin,
    titleOf,
    exportPayload,
    importPayload,
  }
}
