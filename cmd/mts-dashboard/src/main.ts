import { createApp } from 'vue'
import App from './App.vue'
import router from './router'
import { setOnAuthFailed, clearAuth } from './api/client'
import { useAuth } from './composables/useAuth'
import { useNotify } from './composables/useNotify'
import { loginReasonMessage } from './utils/authReason'
import { isAuthStorageKey } from './utils/authStorageSync'
import { shouldSyncOnVisibility } from './utils/pageVisibilitySync'
import './index.css'
import { bootstrapPasswordPolicy } from './utils/passwordPolicyBootstrap'

function currentLocale(): 'zh' | 'en' {
  try {
    const v = localStorage.getItem('mts_locale')
    if (v === 'en' || v === 'zh') return v
  } catch {
    /* ignore */
  }
  return 'zh'
}

const app = createApp(App)
app.use(router)
app.mount('#app')
void bootstrapPasswordPolicy()

setOnAuthFailed(() => {
  clearAuth()
  const { syncFromStorage } = useAuth()
  syncFromStorage()
  try {
    const { error } = useNotify()
    error(loginReasonMessage('session', currentLocale()))
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
  if (!isAuthStorageKey(ev.key)) return
  const { syncFromStorage, ensureSession, refreshSession } = useAuth()
  syncFromStorage()
  if (!ensureSession() && router.currentRoute.value.name !== 'Login') {
    try {
      const { error } = useNotify()
      error(loginReasonMessage('storage', currentLocale()))
    } catch { /* ignore */ }
    void router.replace({ name: 'Login', query: { reason: 'storage' } })
    return
  }
  void refreshSession()
})

// 标签页重新可见：重载本地会话并请求服务端 session 校验
document.addEventListener('visibilitychange', () => {
  if (!shouldSyncOnVisibility(document.visibilityState, document.hidden)) return
  const { syncFromStorage, ensureSession, refreshSession } = useAuth()
  syncFromStorage()
  if (!ensureSession() && router.currentRoute.value.name !== 'Login') {
    try {
      const { error } = useNotify()
      error(loginReasonMessage('session', currentLocale()))
    } catch { /* ignore */ }
    void router.replace({
      name: 'Login',
      query: { redirect: router.currentRoute.value.fullPath, reason: 'session' },
    })
    return
  }
  void refreshSession()
})

