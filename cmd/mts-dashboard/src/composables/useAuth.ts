import { computed, ref } from 'vue'
import { formatCaughtError } from '@/utils/apiError'
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
  apiChangePassword,
  setMustChangePassword,
  getMustChangePassword,
} from '@/api/client'

const isAuthenticated = ref(!!getBearerToken() && !isTokenExpired())
const currentUser = ref(getCurrentUser())
const currentRole = ref(getCurrentUserRole())
const mustChangePassword = ref(getMustChangePassword())
const loggingOut = ref(false)

export function useAuth() {
  const isAdmin = computed(() => currentRole.value === 'admin')

  function syncFromStorage() {
    currentUser.value = getCurrentUser()
    currentRole.value = getCurrentUserRole()
    mustChangePassword.value = getMustChangePassword()
    isAuthenticated.value = !!getBearerToken() && !isTokenExpired()
  }

  async function login(username: string, password: string): Promise<string | null> {
    try {
      const data = await apiLogin(username, password)
      if (!data.token?.token) {
        return formatCaughtError({ code: 'internal', message: 'missing token' })
      }
      const role = (data.token.role || '').trim()
      if (role !== 'admin' && role !== 'user') {
        return formatCaughtError({ code: 'internal', message: 'invalid role' })
      }
      setBearerToken(data.token.token)
      setCurrentUser(data.token.user_name)
      setTokenExpiresAt(data.token.expires_at || '')
      setCurrentUserRole(role)
      setMustChangePassword(!!data.must_change_password)
      currentUser.value = data.token.user_name
      currentRole.value = role
      mustChangePassword.value = !!data.must_change_password
      isAuthenticated.value = true
      resetAuthRedirect()
      return null
    } catch (e) {
      return formatCaughtError(e)
    }
  }

  async function changePassword(oldPassword: string, newPassword: string): Promise<string | null> {
    try {
      const name = currentUser.value || getCurrentUser()
      if (!name) return formatCaughtError({ code: 'unauthenticated', message: 'not logged in' })
      await apiChangePassword(name, oldPassword, newPassword)
      // ChangePassword 会撤销 token，前端清会话并要求重新登录
      clearAuth()
      currentUser.value = ''
      currentRole.value = ''
      mustChangePassword.value = false
      isAuthenticated.value = false
      return null
    } catch (e) {
      return formatCaughtError(e)
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
      mustChangePassword.value = false
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
      mustChangePassword.value = false
      return false
    }
    mustChangePassword.value = getMustChangePassword()
    isAuthenticated.value = true
    return true
  }

  return {
    isAuthenticated,
    currentUser,
    currentRole,
    isAdmin,
    mustChangePassword,
    login,
    changePassword,
    logout,
    loggingOut,
    syncFromStorage,
    ensureSession,
    getTokenExpiresAt,
  }
}
