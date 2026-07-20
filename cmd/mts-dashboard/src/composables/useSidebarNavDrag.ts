import { ref, type Ref } from 'vue'

export function useSidebarNavDrag(
  canReorder: Ref<boolean>,
  reorderTo: (sectionId: string, fromPath: string, toPath: string) => void,
) {
  const dragFrom = ref<{ sectionId: string; path: string } | null>(null)
  const dragOverPath = ref<string | null>(null)

  function onDragStart(sectionId: string, path: string, e: DragEvent) {
    if (!canReorder.value) {
      e.preventDefault()
      return
    }
    dragFrom.value = { sectionId, path }
    dragOverPath.value = null
    try {
      e.dataTransfer?.setData('text/plain', `${sectionId}:${path}`)
      if (e.dataTransfer) e.dataTransfer.effectAllowed = 'move'
    } catch {
      /* ignore */
    }
  }

  function onDragOver(sectionId: string, path: string, e: DragEvent) {
    if (!canReorder.value || !dragFrom.value) return
    if (dragFrom.value.sectionId !== sectionId) return
    e.preventDefault()
    if (e.dataTransfer) e.dataTransfer.dropEffect = 'move'
    dragOverPath.value = path
  }

  function onDragLeave(path: string) {
    if (dragOverPath.value === path) dragOverPath.value = null
  }

  function onDrop(sectionId: string, path: string, e: DragEvent) {
    e.preventDefault()
    const from = dragFrom.value
    dragFrom.value = null
    dragOverPath.value = null
    if (!from || from.sectionId !== sectionId) return
    reorderTo(sectionId, from.path, path)
  }

  function onDragEnd() {
    dragFrom.value = null
    dragOverPath.value = null
  }

  return {
    dragFrom,
    dragOverPath,
    onDragStart,
    onDragOver,
    onDragLeave,
    onDrop,
    onDragEnd,
  }
}
