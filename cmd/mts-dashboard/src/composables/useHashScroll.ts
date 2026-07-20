import { onBeforeUnmount, onMounted, watch } from 'vue'
import { useRoute } from 'vue-router'
import { scheduleScrollToHash } from '@/utils/hashScroll'

/** 路由 hash 变更时滚动到对应锚点 */
export function useHashScroll() {
  const route = useRoute()

  function scrollToCurrentHash() {
    const hash = typeof window !== 'undefined' ? window.location.hash : route.hash
    scheduleScrollToHash(hash, typeof document !== 'undefined' ? document : null)
  }

  onMounted(() => {
    scrollToCurrentHash()
    if (typeof window !== 'undefined') {
      window.addEventListener('hashchange', scrollToCurrentHash)
    }
  })

  onBeforeUnmount(() => {
    if (typeof window !== 'undefined') {
      window.removeEventListener('hashchange', scrollToCurrentHash)
    }
  })

  watch(
    () => route.hash,
    () => {
      scrollToCurrentHash()
    },
  )

  return { scrollToCurrentHash }
}
