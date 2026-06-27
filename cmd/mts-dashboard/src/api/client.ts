const API_BASE = ''
const TOKEN_KEY = 'mts_bearer_token'
const USER_KEY = 'mts_user_name'

interface APIError {
  ok: boolean
  code: string
  message: string
  error?: string
}

export class APIClientError extends Error {
  code: string
  status: number

  constructor(status: number, code: string, message: string) {
    super(message)
    this.name = 'APIClientError'
    this.code = code
    this.status = status
  }
}

function loadToken(): string {
  try {
    return localStorage.getItem(TOKEN_KEY) ?? ''
  } catch {
    return ''
  }
}

function saveToken(token: string) {
  try {
    localStorage.setItem(TOKEN_KEY, token)
  } catch { /* 非关键 */ }
}

function clearToken() {
  try {
    localStorage.removeItem(TOKEN_KEY)
  } catch { /* 非关键 */ }
}

function loadUser(): string {
  try {
    return localStorage.getItem(USER_KEY) ?? ''
  } catch {
    return ''
  }
}

function saveUser(name: string) {
  try {
    localStorage.setItem(USER_KEY, name)
  } catch { /* 非关键 */ }
}

function clearUser() {
  try {
    localStorage.removeItem(USER_KEY)
  } catch { /* 非关键 */ }
}

let bearerToken = loadToken()
let currentUserName = loadUser()

let pendingAuthRedirect = false
let onAuthFailed: (() => void) | null = null

export function setOnAuthFailed(handler: () => void) {
  onAuthFailed = handler
}

export function setBearerToken(token: string) {
  bearerToken = token
  saveToken(token)
}

export function getBearerToken(): string {
  return bearerToken
}

export function setCurrentUser(name: string) {
  currentUserName = name
  saveUser(name)
}

export function getCurrentUser(): string {
  return currentUserName
}

export function clearAuth() {
  bearerToken = ''
  currentUserName = ''
  clearToken()
  clearUser()
}

export function resetAuthRedirect() {
  pendingAuthRedirect = false
}

async function request<T>(path: string, options: RequestInit = {}): Promise<T> {
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    ...(options.headers as Record<string, string>),
  }
  if (bearerToken) {
    headers['Authorization'] = `Bearer ${bearerToken}`
  }
  const response = await fetch(`${API_BASE}${path}`, {
    ...options,
    headers,
  })
  if (!response.ok) {
    if ((response.status === 401 || response.status === 403) && path !== '/api/v1/auth/login') {
      clearAuth()
      if (!pendingAuthRedirect) {
        pendingAuthRedirect = true
        onAuthFailed?.()
      }
    }
    let err: APIError = { ok: false, code: 'internal', message: response.statusText }
    try {
      err = await response.json()
    } catch (_) {
      // 响应体非 JSON，使用默认错误
    }
    throw new APIClientError(response.status, err.code, err.message)
  }
  return response.json()
}

export function apiGet<T>(path: string): Promise<T> {
  return request<T>(path)
}

export function apiPost<T>(path: string, body?: unknown): Promise<T> {
  return request<T>(path, {
    method: 'POST',
    body: body ? JSON.stringify(body) : undefined,
  })
}

export function apiPut<T>(path: string, body?: unknown): Promise<T> {
  return request<T>(path, {
    method: 'PUT',
    body: body ? JSON.stringify(body) : undefined,
  })
}

export function apiDelete<T>(path: string): Promise<T> {
  return request<T>(path, { method: 'DELETE' })
}

export interface LoginResponse {
  token: {
    token: string
    user_name: string
    expires_at: string
  }
}

export async function apiLogin(userName: string, password: string): Promise<LoginResponse> {
  return apiPost<LoginResponse>('/api/v1/auth/login', {
    user_name: userName,
    password,
  })
}

export async function apiLogout(): Promise<void> {
  try {
    await fetch('/api/v1/auth/logout', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${bearerToken}` },
      body: JSON.stringify({ token: bearerToken }),
    })
  } catch (_) {
    // 登出失败不影响前端清理
  }
}
