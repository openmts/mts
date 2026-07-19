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

export function authHeaders(extra: Record<string, string> = {}): Record<string, string> {
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    ...extra,
  }
  if (bearerToken) {
    headers.Authorization = `Bearer ${bearerToken}`
  }
  return headers
}

async function request<T>(path: string, options: RequestInit = {}): Promise<T> {
  const headers = authHeaders(options.headers as Record<string, string> | undefined)
  let response: Response
  try {
    response = await fetch(`${API_BASE}${path}`, {
      ...options,
      headers,
    })
  } catch (err) {
    if (err instanceof DOMException && err.name === 'AbortError') {
      throw new APIClientError(499, 'canceled', '请求已取消')
    }
    throw err
  }
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
  if (response.status === 204) {
    return undefined as T
  }
  const text = await response.text()
  if (!text) {
    return undefined as T
  }
  return JSON.parse(text) as T
}

export function apiGet<T>(path: string, init: RequestInit = {}): Promise<T> {
  return request<T>(path, init)
}

export function apiPost<T>(path: string, body?: unknown, init: RequestInit = {}): Promise<T> {
  return request<T>(path, {
    ...init,
    method: 'POST',
    body: body === undefined ? undefined : JSON.stringify(body),
  })
}

export function apiPut<T>(path: string, body?: unknown, init: RequestInit = {}): Promise<T> {
  return request<T>(path, {
    ...init,
    method: 'PUT',
    body: body === undefined ? undefined : JSON.stringify(body),
  })
}

export function apiDelete<T>(path: string, init: RequestInit = {}): Promise<T> {
  return request<T>(path, { ...init, method: 'DELETE' })
}

export async function apiPostText(
  path: string,
  body?: unknown,
  init: RequestInit = {},
): Promise<{ status: number; text: string }> {
  const headers = authHeaders(init.headers as Record<string, string> | undefined)
  let response: Response
  try {
    response = await fetch(`${API_BASE}${path}`, {
      ...init,
      method: 'POST',
      headers,
      body: body === undefined ? undefined : JSON.stringify(body),
    })
  } catch (err) {
    if (err instanceof DOMException && err.name === 'AbortError') {
      throw new APIClientError(499, 'canceled', '请求已取消')
    }
    throw err
  }
  const text = await response.text()
  if (!response.ok) {
    if ((response.status === 401 || response.status === 403) && path !== '/api/v1/auth/login') {
      clearAuth()
      if (!pendingAuthRedirect) {
        pendingAuthRedirect = true
        onAuthFailed?.()
      }
    }
    let message = response.statusText
    try {
      const err = JSON.parse(text) as APIError
      message = err.message || message
    } catch (_) {
      if (text) message = text
    }
    throw new APIClientError(response.status, 'bad_request', message)
  }
  return { status: response.status, text }
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
      headers: authHeaders(),
      body: JSON.stringify({ token: bearerToken }),
    })
  } catch (_) {
    // 登出失败不影响前端清理
  }
}
