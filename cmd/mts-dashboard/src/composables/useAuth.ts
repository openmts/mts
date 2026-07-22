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
  apiGetSession,
  setMustChangePassword,
  getMustChangePassword,
  reloadAuthFromStorage,
} from '@/api/client'
import { parseAdminOpStatusPayload } from '@/utils/adminOpBusy'

const isAuthenticated = ref(!!getBearerToken() && !isTokenExpired())
const currentUser = ref(getCurrentUser())
const currentRole = ref(getCurrentUserRole())
const mustChangePassword = ref(getMustChangePassword())
const loggingOut = ref(false)
const lastSessionRemainingSeconds = ref<number | null>(null)
const lastSessionCheckedAt = ref<number | null>(null)
const lastSessionServerTimeUnix = ref<number | null>(null)

export function useAuth() {
  const isAdmin = computed(() => currentRole.value === 'admin')

  function syncFromStorage() {
    // 多标签：他页改 localStorage 后，本页须先重载内存再同步 Vue 态
    reloadAuthFromStorage()
    currentUser.value = getCurrentUser()
    currentRole.value = getCurrentUserRole()
    mustChangePassword.value = getMustChangePassword()
    isAuthenticated.value = !!getBearerToken() && !isTokenExpired()
  }

  async function login(
    username: string,
    password: string,
    opts?: { ttlSeconds?: number; signal?: AbortSignal },
  ): Promise<string | null> {
    try {
      const data = await apiLogin(username, password, opts)
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
      if (data.token.expires_at) {
        const exp = Date.parse(data.token.expires_at)
        if (Number.isFinite(exp)) {
          lastSessionRemainingSeconds.value = Math.max(0, Math.floor((exp - Date.now()) / 1000))
          lastSessionCheckedAt.value = Date.now()
        }
      }
      return null
    } catch (e) {
      return formatCaughtError(e)
    }
  }

  async function changePassword(
    oldPassword: string,
    newPassword: string,
    opts?: { signal?: AbortSignal },
  ): Promise<string | null> {
    try {
      const name = currentUser.value || getCurrentUser()
      if (!name) return formatCaughtError({ code: 'unauthenticated', message: 'not logged in' })
      await apiChangePassword(name, oldPassword, newPassword, opts?.signal ? { signal: opts.signal } : {})
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
      lastSessionRemainingSeconds.value = null
      lastSessionCheckedAt.value = null
      lastSessionServerTimeUnix.value = null
      loggingOut.value = false
    }
  }


  async function refreshSession(): Promise<boolean> {
    const token = getBearerToken()
    if (!token || isTokenExpired()) {
      clearAuth()
      isAuthenticated.value = false
      currentUser.value = ''
      currentRole.value = ''
      mustChangePassword.value = false
      lastSessionRemainingSeconds.value = null
      lastSessionCheckedAt.value = null
      lastSessionServerTimeUnix.value = null
      return false
    }
    try {
      const session = await apiGetSession()
      const role = (session.role || '').trim()
      if (role === 'admin' || role === 'user') {
        setCurrentUserRole(role)
        currentRole.value = role
      }
      if (session.user_name) {
        setCurrentUser(session.user_name)
        currentUser.value = session.user_name
      }
      if (session.expires_at) {
        setTokenExpiresAt(session.expires_at)
      }
      if (typeof session.remaining_seconds === 'number' && Number.isFinite(session.remaining_seconds)) {
        lastSessionRemainingSeconds.value = Math.max(0, Math.floor(session.remaining_seconds))
      } else if (session.expires_at) {
        const exp = Date.parse(session.expires_at)
        lastSessionRemainingSeconds.value = Number.isFinite(exp)
          ? Math.max(0, Math.floor((exp - Date.now()) / 1000))
          : null
      } else {
        lastSessionRemainingSeconds.value = null
      }
      if (typeof session.server_time_unix === 'number' && Number.isFinite(session.server_time_unix)) {
        lastSessionServerTimeUnix.value = Math.floor(session.server_time_unix)
      } else {
        lastSessionServerTimeUnix.value = null
      }
      lastSessionCheckedAt.value = Date.now()
      setMustChangePassword(!!session.must_change_password)
      mustChangePassword.value = !!session.must_change_password
      isAuthenticated.value = true
      // 动态 import，避免与 useAdminOpBusy 的循环依赖
      void import('@/composables/useAdminOpBusy').then(({ applyGlobalAdminOpStatus }) => {
        applyGlobalAdminOpStatus(parseAdminOpStatusPayload(session))
      })
      return true
    } catch {
      // 网络抖动时不立即清会话；仅 token 失效由 request 层 triggerAuthFailed
      syncFromStorage()
      return ensureSession()
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
    refreshSession,
    lastSessionRemainingSeconds,
    lastSessionCheckedAt,
    lastSessionServerTimeUnix,
    getTokenExpiresAt,
  }
}
