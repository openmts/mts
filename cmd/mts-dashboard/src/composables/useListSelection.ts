import { computed, ref, type Ref } from 'vue'
import {
  clearSelection,
  isAllSelected,
  isSomeSelected,
  resolveExportIds,
  toggleSelectAll,
  toggleSelection,
} from '@/utils/listSelection'

export function useListSelection(visibleIds: Ref<readonly string[]>) {
  const selectedIds = ref<string[]>([])

  const selectedCount = computed(() => selectedIds.value.length)
  const allVisibleSelected = computed(() =>
    isAllSelected(selectedIds.value, visibleIds.value),
  )
  const someVisibleSelected = computed(() =>
    isSomeSelected(selectedIds.value, visibleIds.value),
  )
  const exportIds = computed(() =>
    resolveExportIds(selectedIds.value, visibleIds.value),
  )

  function isSelected(id: string): boolean {
    return selectedIds.value.includes(id)
  }

  function toggle(id: string, checked: boolean) {
    selectedIds.value = toggleSelection(selectedIds.value, id, checked)
  }

  function toggleAllVisible(checked: boolean) {
    selectedIds.value = toggleSelectAll(selectedIds.value, visibleIds.value, checked)
  }

  function clear() {
    selectedIds.value = clearSelection()
  }

  /** 移除已不存在的 id（列表刷新后） */
  function pruneTo(existing: readonly string[]) {
    const set = new Set(existing.map(String))
    selectedIds.value = selectedIds.value.filter((id) => set.has(id))
  }

  return {
    selectedIds,
    selectedCount,
    allVisibleSelected,
    someVisibleSelected,
    exportIds,
    isSelected,
    toggle,
    toggleAllVisible,
    clear,
    pruneTo,
  }
}
