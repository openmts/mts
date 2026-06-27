import { ref } from 'vue'
import { setBearerToken, getBearerToken, setCurrentUser, getCurrentUser, clearAuth, resetAuthRedirect, apiLogin, apiLogout } from '@/api/client'

const isAuthenticated = ref(!!getBearerToken())
const currentUser = ref(getCurrentUser())
const loggingOut = ref(false)

export function useAuth() {
  async function login(username: string, password: string): Promise<string | null> {
    try {
      const data = await apiLogin(username, password)
      if (!data.token?.token) {
        return '登录失败：服务端未返回有效 token'
      }
      setBearerToken(data.token.token)
      setCurrentUser(data.token.user_name)
      currentUser.value = data.token.user_name
      isAuthenticated.value = true
      resetAuthRedirect()
      return null
    } catch (e) {
      return e instanceof Error ? e.message : '登录失败'
    }
  }

  async function logout() {
    if (loggingOut.value) return
    loggingOut.value = true
    try {
      await apiLogout()
    } finally {
      clearAuth()
      currentUser.value = ''
      isAuthenticated.value = false
      loggingOut.value = false
    }
  }

  return { isAuthenticated, currentUser, login, logout, loggingOut }
}