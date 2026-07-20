import { beginRequest, endRequest } from '@/composables/useGlobalLoading'
const API_BASE = (import.meta.env.VITE_API_BASE ?? '').replace(/\/$/, '')
const TOKEN_KEY = 'mts_bearer_token'
const USER_KEY = 'mts_user_name'
const ROLE_KEY = 'mts_user_role'
const MUST_CHANGE_KEY = 'mts_must_change_password'
const EXPIRES_KEY = 'mts_token_expires_at'
const ADMIN_TOKEN_KEY = 'mts_admin_token'
const DATA_TOKEN_KEY = 'mts_data_token'

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

function storageGet(key: string): string {
  try {
    return localStorage.getItem(key) ?? ''
  } catch {
    return ''
  }
}

function storageSet(key: string, value: string) {
  try {
    if (value) localStorage.setItem(key, value)
    else localStorage.removeItem(key)
  } catch { /* 非关键 */ }
}

function storageRemove(key: string) {
  try {
    localStorage.removeItem(key)
  } catch { /* 非关键 */ }
}

function sessionGet(key: string): string {
  try {
    return sessionStorage.getItem(key) ?? ''
  } catch {
    return ''
  }
}

function sessionSet(key: string, value: string) {
  try {
    if (value) sessionStorage.setItem(key, value)
    else sessionStorage.removeItem(key)
  } catch { /* 非关键 */ }
}

let bearerToken = storageGet(TOKEN_KEY)
let currentUserName = storageGet(USER_KEY)
let currentUserRole = storageGet(ROLE_KEY)
let tokenExpiresAt = storageGet(EXPIRES_KEY)
let adminToken = sessionGet(ADMIN_TOKEN_KEY)
let dataToken = sessionGet(DATA_TOKEN_KEY)

let pendingAuthRedirect = false
let onAuthFailed: (() => void) | null = null

export function setOnAuthFailed(handler: () => void) {
  onAuthFailed = handler
}

export function setBearerToken(token: string) {
  bearerToken = token
  storageSet(TOKEN_KEY, token)
}

export function getBearerToken(): string {
  return bearerToken
}

export function setTokenExpiresAt(iso: string) {
  tokenExpiresAt = iso
  storageSet(EXPIRES_KEY, iso)
}

export function getTokenExpiresAt(): string {
  return tokenExpiresAt
}

export function isTokenExpired(nowMs = Date.now()): boolean {
  if (!tokenExpiresAt) return false
  const exp = Date.parse(tokenExpiresAt)
  if (Number.isNaN(exp)) return false
  return exp <= nowMs
}

export function setCurrentUser(name: string) {
  currentUserName = name
  storageSet(USER_KEY, name)
}

export function getCurrentUser(): string {
  return currentUserName
}

export function setCurrentUserRole(role: string) {
  currentUserRole = role
  storageSet(ROLE_KEY, role)
}

export function setMustChangePassword(required: boolean) {
  if (required) storageSet(MUST_CHANGE_KEY, '1')
  else storageRemove(MUST_CHANGE_KEY)
}

export function getMustChangePassword(): boolean {
  return storageGet(MUST_CHANGE_KEY) === '1'
}

export function getCurrentUserRole(): string {
  return currentUserRole
}

export function setAdminToken(token: string) {
  adminToken = token.trim()
  sessionSet(ADMIN_TOKEN_KEY, adminToken)
}

export function getAdminToken(): string {
  return adminToken
}

export function setDataToken(token: string) {
  dataToken = token.trim()
  sessionSet(DATA_TOKEN_KEY, dataToken)
}

export function getDataToken(): string {
  return dataToken
}

export function clearAuth() {
  bearerToken = ''
  currentUserName = ''
  currentUserRole = ''
  tokenExpiresAt = ''
  storageSet(TOKEN_KEY, '')
  storageSet(USER_KEY, '')
  storageSet(ROLE_KEY, '')
  storageSet(EXPIRES_KEY, '')
  storageRemove(MUST_CHANGE_KEY)
}

export function resetAuthRedirect() {
  pendingAuthRedirect = false
}

export function authHeaders(extra: Record<string, string> = {}, method = 'GET'): Record<string, string> {
  const headers: Record<string, string> = { ...extra }
  const upper = method.toUpperCase()
  if (upper !== 'GET' && upper !== 'HEAD' && !headers['Content-Type'] && !headers['content-type']) {
    headers['Content-Type'] = 'application/json'
  }
  if (bearerToken) {
    headers.Authorization = `Bearer ${bearerToken}`
  }
  if (adminToken) {
    headers['X-MTS-Admin-Token'] = adminToken
  }
  if (dataToken) {
    headers['X-MTS-Data-Token'] = dataToken
  }
  return headers
}

function shouldClearSession(status: number, code: string): boolean {
  if (status === 401) return true
  if (code === 'unauthenticated') return true
  return false
}

function triggerAuthFailed() {
  clearAuth()
  if (!pendingAuthRedirect) {
    pendingAuthRedirect = true
    onAuthFailed?.()
  }
}

async function readAPIError(response: Response, fallbackText = ''): Promise<APIError> {
  let err: APIError = { ok: false, code: 'internal', message: response.statusText || 'request failed' }
  const text = fallbackText || await response.text().catch(() => '')
  if (!text) return err
  try {
    const parsed = JSON.parse(text) as APIError
    err = {
      ok: false,
      code: parsed.code || err.code,
      message: parsed.message || parsed.error || err.message,
      error: parsed.error,
    }
  } catch (_) {
    err.message = text
  }
  return err
}

function handleAuthFailure(path: string, status: number, code: string) {
  if (path === '/api/v1/auth/login') return
  if (shouldClearSession(status, code)) {
    triggerAuthFailed()
  }
}

async function request<T>(path: string, options: RequestInit = {}): Promise<T> {
  if (bearerToken && isTokenExpired()) {
    triggerAuthFailed()
    throw new APIClientError(401, 'unauthenticated', 'session expired')
  }
  const method = (options.method || 'GET').toUpperCase()
  const headers = authHeaders(options.headers as Record<string, string> | undefined, method)
  beginRequest()
  try {
    let response: Response
    try {
      response = await fetch(`${API_BASE}${path}`, {
        ...options,
        method,
        headers,
      })
    } catch (err) {
      if (err instanceof DOMException && err.name === 'AbortError') {
        throw new APIClientError(499, 'canceled', 'request canceled')
      }
      throw err
    }
    if (!response.ok) {
      const err = await readAPIError(response)
      handleAuthFailure(path, response.status, err.code)
      throw new APIClientError(response.status, err.code, err.message)
    }
    if (response.status === 204) {
      return undefined as T
    }
    const text = await response.text()
    if (!text) {
      return undefined as T
    }
    try {
      return JSON.parse(text) as T
    } catch (_) {
      throw new APIClientError(response.status, 'internal', 'invalid JSON response')
    }
  } finally {
    endRequest()
  }
}

export function apiGet<T>(path: string, init: RequestInit = {}): Promise<T> {
  return request<T>(path, { ...init, method: 'GET' })
}

/** 拉取非 JSON 文本响应（如 Prometheus /metrics） */
export async function apiGetText(path: string, init: RequestInit = {}): Promise<string> {
  if (bearerToken && isTokenExpired()) {
    triggerAuthFailed()
    throw new APIClientError(401, 'unauthenticated', 'session expired')
  }
  const method = 'GET'
  const headers = authHeaders(init.headers as Record<string, string> | undefined, method)
  beginRequest()
  try {
    let response: Response
    try {
      response = await fetch(`${API_BASE}${path}`, { ...init, method, headers })
    } catch (err) {
      if (err instanceof DOMException && err.name === 'AbortError') {
        throw new APIClientError(499, 'canceled', 'request canceled')
      }
      throw err
    }
    if (!response.ok) {
      const err = await readAPIError(response)
      handleAuthFailure(path, response.status, err.code)
      throw new APIClientError(response.status, err.code, err.message)
    }
    return await response.text()
  } finally {
    endRequest()
  }
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

export type NDJSONHandler = (line: string, record: unknown | null, parseError: boolean) => void

/** 真流式 NDJSON：按行回调，支持 AbortSignal */
export async function apiPostNDJSONStream(
  path: string,
  body: unknown,
  onLine: NDJSONHandler,
  init: RequestInit = {},
): Promise<{ status: number; lines: number }> {
  if (bearerToken && isTokenExpired()) {
    triggerAuthFailed()
    throw new APIClientError(401, 'unauthenticated', 'session expired')
  }
  const headers = authHeaders(init.headers as Record<string, string> | undefined, 'POST')
  beginRequest()
  try {
    let response: Response
    try {
      response = await fetch(`${API_BASE}${path}`, {
        ...init,
        method: 'POST',
        headers,
        body: JSON.stringify(body),
      })
    } catch (err) {
      if (err instanceof DOMException && err.name === 'AbortError') {
        throw new APIClientError(499, 'canceled', 'request canceled')
      }
      throw err
    }
    if (!response.ok) {
      const err = await readAPIError(response)
      handleAuthFailure(path, response.status, err.code)
      throw new APIClientError(response.status, err.code, err.message)
    }
    if (!response.body) {
      const text = await response.text()
      let lines = 0
      for (const line of text.split('\n')) {
        const trimmed = line.trim()
        if (!trimmed) continue
        lines += 1
        try {
          onLine(trimmed, JSON.parse(trimmed), false)
        } catch {
          onLine(trimmed, null, true)
        }
      }
      return { status: response.status, lines }
    }
    const reader = response.body.getReader()
    const decoder = new TextDecoder()
    let buffer = ''
    let lines = 0
    try {
      while (true) {
        const { done, value } = await reader.read()
        if (done) break
        buffer += decoder.decode(value, { stream: true })
        let idx = buffer.indexOf('\n')
        while (idx >= 0) {
          const line = buffer.slice(0, idx).trim()
          buffer = buffer.slice(idx + 1)
          if (line) {
            lines += 1
            try {
              onLine(line, JSON.parse(line), false)
            } catch {
              onLine(line, null, true)
            }
          }
          idx = buffer.indexOf('\n')
        }
      }
      buffer += decoder.decode()
      const tail = buffer.trim()
      if (tail) {
        lines += 1
        try {
          onLine(tail, JSON.parse(tail), false)
        } catch {
          onLine(tail, null, true)
        }
      }
    } catch (err) {
      if (err instanceof DOMException && err.name === 'AbortError') {
        throw new APIClientError(499, 'canceled', 'request canceled')
      }
      throw err
    } finally {
      try {
        reader.releaseLock()
      } catch (_) { /* ignore */ }
    }
    return { status: response.status, lines }
  } finally {
    endRequest()
  }
}

export interface LoginResponse {
  token: {
    token: string
    user_name: string
    role?: string
    expires_at: string
  }
  must_change_password?: boolean
}

export async function apiLogin(userName: string, password: string): Promise<LoginResponse> {
  return apiPost<LoginResponse>('/api/v1/auth/login', {
    user_name: userName,
    password,
  })
}

export async function apiChangePassword(userName: string, oldPassword: string, newPassword: string): Promise<void> {
  await apiPost('/api/v1/auth/password', {
    user_name: userName,
    old_password: oldPassword,
    new_password: newPassword,
  })
}

export async function apiLogout(): Promise<void> {
  try {
    await fetch('/api/v1/auth/logout', {
      method: 'POST',
      headers: authHeaders({}, 'POST'),
      body: JSON.stringify({ token: bearerToken }),
    })
  } catch (_) {
    // 登出失败不影响前端清理
  }
}
