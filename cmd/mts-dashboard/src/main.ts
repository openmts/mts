import { createApp } from 'vue'
import App from './App.vue'
import router from './router'
import { setOnAuthFailed, clearAuth } from './api/client'
import { useAuth } from './composables/useAuth'
import './index.css'

const app = createApp(App)
app.use(router)
app.mount('#app')

setOnAuthFailed(() => {
  clearAuth()
  const { syncFromStorage } = useAuth()
  syncFromStorage()
  if (router.currentRoute.value.name !== 'Login') {
    void router.replace({ name: 'Login' })
  }
})

// 多标签会话同步
window.addEventListener('storage', (ev) => {
  if (!ev.key || !ev.key.startsWith('mts_')) return
  const { syncFromStorage, ensureSession } = useAuth()
  syncFromStorage()
  if (!ensureSession() && router.currentRoute.value.name !== 'Login') {
    void router.replace({ name: 'Login' })
  }
})
