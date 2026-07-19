import { computed, ref } from 'vue'
import {
  setBearerToken,
  getBearerToken,
  setCurrentUser,
  getCurrentUser,
  setCurrentUserRole,
  getCurrentUserRole,
  setTokenExpiresAt,
  getTokenExpiresAt,
  isTokenExpired,
  clearAuth,
  resetAuthRedirect,
  apiLogin,
  apiLogout,
} from '@/api/client'

const isAuthenticated = ref(!!getBearerToken() && !isTokenExpired())
const currentUser = ref(getCurrentUser())
const currentRole = ref(getCurrentUserRole())
const loggingOut = ref(false)

export function useAuth() {
  const isAdmin = computed(() => currentRole.value === 'admin')

  function syncFromStorage() {
    currentUser.value = getCurrentUser()
    currentRole.value = getCurrentUserRole()
    isAuthenticated.value = !!getBearerToken() && !isTokenExpired()
  }

  async function login(username: string, password: string): Promise<string | null> {
    try {
      const data = await apiLogin(username, password)
      if (!data.token?.token) {
        return '登录失败：服务端未返回有效 token'
      }
      const role = (data.token.role || '').trim()
      if (role !== 'admin' && role !== 'user') {
        return '登录失败：服务端未返回可信角色'
      }
      setBearerToken(data.token.token)
      setCurrentUser(data.token.user_name)
      setTokenExpiresAt(data.token.expires_at || '')
      setCurrentUserRole(role)
      currentUser.value = data.token.user_name
      currentRole.value = role
      isAuthenticated.value = true
      resetAuthRedirect()
      return null
    } catch (e) {
      return e instanceof Error ? e.message : '登录失败'
    }
  }

  async function logout(): Promise<void> {
    if (loggingOut.value) return
    loggingOut.value = true
    try {
      await apiLogout()
    } finally {
      clearAuth()
      currentUser.value = ''
      currentRole.value = ''
      isAuthenticated.value = false
      loggingOut.value = false
    }
  }

  function ensureSession(): boolean {
    if (!getBearerToken() || isTokenExpired()) {
      clearAuth()
      isAuthenticated.value = false
      currentUser.value = ''
      currentRole.value = ''
      return false
    }
    isAuthenticated.value = true
    return true
  }

  return {
    isAuthenticated,
    currentUser,
    currentRole,
    isAdmin,
    login,
    logout,
    loggingOut,
    syncFromStorage,
    ensureSession,
    getTokenExpiresAt,
  }
}
