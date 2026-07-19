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
  apiGet,
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

  async function resolveRole(userName: string): Promise<string> {
    try {
      const data = await apiGet<{ users: { name: string; role?: string }[] }>('/api/v1/users')
      const self = (data.users ?? []).find((u) => u.name === userName)
      if (self?.role) return self.role
      return 'admin'
    } catch (_) {
      return 'user'
    }
  }

  async function login(username: string, password: string): Promise<string | null> {
    try {
      const data = await apiLogin(username, password)
      if (!data.token?.token) {
        return '登录失败：服务端未返回有效 token'
      }
      setBearerToken(data.token.token)
      setCurrentUser(data.token.user_name)
      setTokenExpiresAt(data.token.expires_at || '')
      const role = await resolveRole(data.token.user_name)
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
