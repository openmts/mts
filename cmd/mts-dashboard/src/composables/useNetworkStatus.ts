import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import {
  isOfflineStatus,
  networkStatusFromOnlineFlag,
  readNavigatorOnline,
  type NetworkStatus,
} from '@/utils/networkStatus'

/** 订阅浏览器 online/offline 事件 */
export function useNetworkStatus() {
  const online = ref(readNavigatorOnline())
  const status = computed<NetworkStatus>(() => networkStatusFromOnlineFlag(online.value))
  const offline = computed(() => isOfflineStatus(status.value))

  function sync() {
    online.value = readNavigatorOnline()
  }

  function onOnline() {
    online.value = true
  }
  function onOffline() {
    online.value = false
  }

  onMounted(() => {
    sync()
    window.addEventListener('online', onOnline)
    window.addEventListener('offline', onOffline)
  })
  onBeforeUnmount(() => {
    window.removeEventListener('online', onOnline)
    window.removeEventListener('offline', onOffline)
  })

  return { online, status, offline, sync }
}
