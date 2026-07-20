import { computed, onBeforeUnmount, onMounted, ref, type Ref } from 'vue'
import { CLIENT_PREFS_CHANGED_EVENT } from '@/utils/clientPrefs'
import { groupNavItems } from '@/utils/navSections'
import {
  applyNavOrder,
  loadNavOrderPrefs,
  moveNavPath,
  moveNavPathTo,
  resolveSectionOrder,
  saveNavOrderMap,
  setSectionOrder,
  type NavOrderMap,
} from '@/utils/navOrder'

const storage = typeof localStorage !== 'undefined' ? localStorage : null

export function useSidebarNavOrder<T extends { to: string }>(
  roleNavItems: Ref<readonly T[]>,
  canReorder: Ref<boolean>,
) {
  const orderMap = ref<NavOrderMap>(loadNavOrderPrefs(storage))

  const orderedRoleNavItems = computed(() => {
    const groups = groupNavItems(roleNavItems.value)
    const flat: T[] = []
    for (const g of groups) {
      flat.push(...applyNavOrder(g.items, orderMap.value[g.id]))
    }
    return flat
  })

  function orderedGroups(items: readonly T[]) {
    return groupNavItems(items).map((g) => ({
      ...g,
      items: applyNavOrder(g.items, orderMap.value[g.id]),
    }))
  }

  function persistOrder(next: NavOrderMap) {
    orderMap.value = next
    saveNavOrderMap(storage, next)
  }

  function reorder(sectionId: string, path: string, direction: 'up' | 'down') {
    const group = groupNavItems(roleNavItems.value).find((g) => g.id === sectionId)
    if (!group) return
    const current = resolveSectionOrder(
      group.items.map((x) => x.to),
      orderMap.value[sectionId],
    )
    const moved = moveNavPath(current, path, direction)
    persistOrder(setSectionOrder(orderMap.value, sectionId, moved))
  }

  function reorderTo(sectionId: string, fromPath: string, toPath: string) {
    if (!canReorder.value) return
    if (!fromPath || !toPath || fromPath === toPath) return
    const group = groupNavItems(roleNavItems.value).find((g) => g.id === sectionId)
    if (!group) return
    const paths = new Set(group.items.map((x) => x.to))
    if (!paths.has(fromPath) || !paths.has(toPath)) return
    const current = resolveSectionOrder(
      group.items.map((x) => x.to),
      orderMap.value[sectionId],
    )
    const moved = moveNavPathTo(current, fromPath, toPath)
    persistOrder(setSectionOrder(orderMap.value, sectionId, moved))
  }

  function resetSectionOrder(sectionId: string) {
    persistOrder(setSectionOrder(orderMap.value, sectionId, []))
  }

  function resetAllOrder() {
    persistOrder({})
  }

  function canMove(sectionId: string, path: string, direction: 'up' | 'down', groups: { id: string; items: { to: string }[] }[]): boolean {
    if (!canReorder.value) return false
    const group = groups.find((g) => g.id === sectionId)
    if (!group || group.items.length < 2) return false
    const idx = group.items.findIndex((x) => x.to === path)
    if (idx < 0) return false
    return direction === 'up' ? idx > 0 : idx < group.items.length - 1
  }

  function reloadFromStorage() {
    orderMap.value = loadNavOrderPrefs(storage)
  }

  function onPrefsChanged() {
    reloadFromStorage()
  }

  onMounted(() => {
    if (typeof window === 'undefined') return
    window.addEventListener(CLIENT_PREFS_CHANGED_EVENT, onPrefsChanged)
  })
  onBeforeUnmount(() => {
    if (typeof window === 'undefined') return
    window.removeEventListener(CLIENT_PREFS_CHANGED_EVENT, onPrefsChanged)
  })

  return {
    orderMap,
    orderedRoleNavItems,
    orderedGroups,
    reorder,
    reorderTo,
    resetSectionOrder,
    resetAllOrder,
    canMove,
    reloadFromStorage,
  }
}

export function navPathTestSuffix(path: string): string {
  return path === '/' ? 'home' : path.slice(1).split('/').join('-')
}
