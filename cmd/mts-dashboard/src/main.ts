import { createApp } from 'vue'
import App from './App.vue'
import router from './router'
import { setOnAuthFailed, clearAuth } from './api/client'
import { useAuth } from './composables/useAuth'
import { useNotify } from './composables/useNotify'
import './index.css'

const app = createApp(App)
app.use(router)
app.mount('#app')

setOnAuthFailed(() => {
  clearAuth()
  const { syncFromStorage } = useAuth()
  syncFromStorage()
  try {
    const { error } = useNotify()
    error('登录已过期或会话失效，请重新登录')
  } catch {
    /* notify 在 SSR/无 window 时忽略 */
  }
  if (router.currentRoute.value.name !== 'Login') {
    void router.replace({
      name: 'Login',
      query: { redirect: router.currentRoute.value.fullPath, reason: 'session' },
    })
  }
})

// 多标签会话同步
window.addEventListener('storage', (ev) => {
  if (!ev.key || !ev.key.startsWith('mts_')) return
  const { syncFromStorage, ensureSession } = useAuth()
  syncFromStorage()
  if (!ensureSession() && router.currentRoute.value.name !== 'Login') {
    try {
      const { error } = useNotify()
      error('会话已在其他标签页退出')
    } catch { /* ignore */ }
    void router.replace({ name: 'Login', query: { reason: 'storage' } })
  }
})
