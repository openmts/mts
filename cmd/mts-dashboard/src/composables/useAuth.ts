import { ref } from 'vue'
import { setAdminToken, getAdminToken } from '@/api/client'

const isAuthenticated = ref(!!getAdminToken())
const currentUser = ref('')

export function useAuth() {
  function login(username: string, password: string) {
    // stub: 预留认证接口，当前直接放行
    void password
    setAdminToken(username)
    currentUser.value = username
    isAuthenticated.value = true
  }

  function logout() {
    setAdminToken('')
    currentUser.value = ''
    isAuthenticated.value = false
  }

  return { isAuthenticated, currentUser, login, logout }
}
