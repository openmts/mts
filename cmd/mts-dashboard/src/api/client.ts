import { beginRequest, endRequest } from '@/composables/useGlobalLoading'
import {
  createTimeoutSignal,
  DEFAULT_API_TIMEOUT_MS,
  isAbortError,
  resolveApiTimeoutMs,
} from '@/utils/requestTimeout'
import { readAuthStorageSnapshot } from '@/utils/authStorageSync'
import { parseAdminBusyFromHeaders } from '@/utils/adminOpBusy'
const API_BASE = (import.meta.env.VITE_API_BASE ?? '').replace(/\/$/, '')
const API_TIMEOUT_MS = resolveApiTimeoutMs(import.meta.env.VITE_API_TIMEOUT_MS, DEFAULT_API_TIMEOUT_MS)
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
  admin_op_busy?: boolean
  op?: string
}

export class APIClientError extends Error {
  code: string
  status: number
  adminOpBusy?: boolean
  op?: string

  constructor(
    status: number,
    code: string,
    message: string,
    extras?: { adminOpBusy?: boolean; op?: string },
  ) {
    super(message)
    this.name = 'APIClientError'
    this.code = code
    this.status = status
    if (extras?.adminOpBusy) this.adminOpBusy = true
    if (extras?.op) this.op = extras.op
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

/** 从 localStorage 重载会话到内存（多标签 storage 事件后调用） */
export function reloadAuthFromStorage(): void {
  const snap = readAuthStorageSnapshot((key) => {
    try {
      return localStorage.getItem(key)
    } catch {
      return ''
    }
  })
  bearerToken = snap.token
  currentUserName = snap.user
  currentUserRole = snap.role
  tokenExpiresAt = snap.expiresAt
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
  const hdr = parseAdminBusyFromHeaders((name) => response.headers.get(name))
  if (hdr.busy) {
    err.admin_op_busy = true
    if (hdr.op) err.op = hdr.op
  }
  const text = fallbackText || await response.text().catch(() => '')
  if (!text) return err
  try {
    const parsed = JSON.parse(text) as APIError
    err = {
      ok: false,
      code: parsed.code || err.code,
      message: parsed.message || parsed.error || err.message,
      error: parsed.error,
      admin_op_busy: Boolean(parsed.admin_op_busy) || hdr.busy || Boolean(err.admin_op_busy),
      op: (typeof parsed.op === 'string' && parsed.op.trim()) ? parsed.op.trim() : (hdr.op || err.op),
    }
  } catch (_) {
    err.message = text
  }
  return err
}

function handleAuthFailure(path: string, status: number, code: string) {
  // 登录/改密失败不得清会话：改密旧密码错误仅表示凭证校验失败
  if (path === '/api/v1/auth/login' || path === '/api/v1/auth/password') return
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
  const timeoutHandle = createTimeoutSignal(options.signal, API_TIMEOUT_MS)
  beginRequest()
  try {
    let response: Response
    try {
      response = await fetch(`${API_BASE}${path}`, {
        ...options,
        method,
        headers,
        signal: timeoutHandle.signal,
      })
    } catch (err) {
      if (isAbortError(err)) {
        if (timeoutHandle.didTimeout()) {
          throw new APIClientError(408, 'timeout', 'request timeout')
        }
        throw new APIClientError(499, 'canceled', 'request canceled')
      }
      throw err
    }
    if (!response.ok) {
      const err = await readAPIError(response)
      handleAuthFailure(path, response.status, err.code)
      throw new APIClientError(response.status, err.code, err.message, {
        adminOpBusy: Boolean(err.admin_op_busy),
        op: err.op,
      })
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
    timeoutHandle.cleanup()
    endRequest()
  }
}

export function apiGet<T>(path: string, init: RequestInit = {}): Promise<T> {
  return request<T>(path, { ...init, method: 'GET' })
}

/** 不触发全局 loading 的 GET（会话同步/心跳等后台请求） */
export async function apiGetSilent<T>(path: string, init: RequestInit = {}): Promise<T> {
  if (bearerToken && isTokenExpired()) {
    triggerAuthFailed()
    throw new APIClientError(401, 'unauthenticated', 'session expired')
  }
  const method = 'GET'
  const headers = authHeaders(init.headers as Record<string, string> | undefined, method)
  const timeoutHandle = createTimeoutSignal(init.signal, API_TIMEOUT_MS)
  try {
    let response: Response
    try {
      response = await fetch(`${API_BASE}${path}`, {
        ...init,
        method,
        headers,
        signal: timeoutHandle.signal,
      })
    } catch (err) {
      if (isAbortError(err)) {
        if (timeoutHandle.didTimeout()) {
          throw new APIClientError(408, 'timeout', 'request timeout')
        }
        throw new APIClientError(499, 'canceled', 'request canceled')
      }
      throw err
    }
    if (!response.ok) {
      const err = await readAPIError(response)
      handleAuthFailure(path, response.status, err.code)
      throw new APIClientError(response.status, err.code, err.message, {
        adminOpBusy: Boolean(err.admin_op_busy),
        op: err.op,
      })
    }
    if (response.status === 204) return undefined as T
    const body = await response.text()
    if (!body) return undefined as T
    try {
      return JSON.parse(body) as T
    } catch (_) {
      throw new APIClientError(response.status, 'internal', 'invalid JSON response')
    }
  } finally {
    timeoutHandle.cleanup()
  }
}

/** 轻量可达性探测：不携带鉴权、不触发全局 loading、不因过期 token 强制登出 */
export async function probeReadyz(init: RequestInit = {}): Promise<{ ok: boolean; status: number }> {
  const timeoutHandle = createTimeoutSignal(init.signal, 5_000)
  try {
    const response = await fetch(`${API_BASE}/readyz`, {
      method: 'GET',
      cache: 'no-store',
      ...init,
      headers: {
        Accept: 'application/json, text/plain, */*',
        ...(init.headers as Record<string, string> | undefined),
      },
      signal: timeoutHandle.signal,
    })
    const status = response.status
    return { ok: status >= 200 && status < 300, status }
  } catch {
    return { ok: false, status: 0 }
  } finally {
    timeoutHandle.cleanup()
  }
}

/** 拉取非 JSON 文本响应（如 Prometheus /metrics） */
export async function apiGetText(path: string, init: RequestInit = {}): Promise<string> {
  if (bearerToken && isTokenExpired()) {
    triggerAuthFailed()
    throw new APIClientError(401, 'unauthenticated', 'session expired')
  }
  const method = 'GET'
  const headers = authHeaders(init.headers as Record<string, string> | undefined, method)
  const timeoutHandle = createTimeoutSignal(init.signal, API_TIMEOUT_MS)
  beginRequest()
  try {
    let response: Response
    try {
      response = await fetch(`${API_BASE}${path}`, {
        ...init,
        method,
        headers,
        signal: timeoutHandle.signal,
      })
    } catch (err) {
      if (isAbortError(err)) {
        if (timeoutHandle.didTimeout()) {
          throw new APIClientError(408, 'timeout', 'request timeout')
        }
        throw new APIClientError(499, 'canceled', 'request canceled')
      }
      throw err
    }
    if (!response.ok) {
      const err = await readAPIError(response)
      handleAuthFailure(path, response.status, err.code)
      throw new APIClientError(response.status, err.code, err.message, {
        adminOpBusy: Boolean(err.admin_op_busy),
        op: err.op,
      })
    }
    return await response.text()
  } finally {
    timeoutHandle.cleanup()
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
      if (isAbortError(err)) {
        throw new APIClientError(499, 'canceled', 'request canceled')
      }
      throw err
    }
    if (!response.ok) {
      const err = await readAPIError(response)
      handleAuthFailure(path, response.status, err.code)
      throw new APIClientError(response.status, err.code, err.message, {
        adminOpBusy: Boolean(err.admin_op_busy),
        op: err.op,
      })
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
      if (isAbortError(err)) {
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

export async function apiLogin(
  userName: string,
  password: string,
  opts?: { ttlSeconds?: number; signal?: AbortSignal },
): Promise<LoginResponse> {
  const body: Record<string, unknown> = {
    user_name: userName,
    password,
  }
  if (opts?.ttlSeconds != null && opts.ttlSeconds > 0) {
    body.ttl_seconds = opts.ttlSeconds
  }
  return apiPost<LoginResponse>('/api/v1/auth/login', body, opts?.signal ? { signal: opts.signal } : {})
}

export interface ChangePasswordResponse {
  ok?: boolean
  must_change_password?: boolean
  admin_op_busy?: boolean
  op?: string
  started_at_unix?: number
  last?: unknown
}

export async function apiChangePassword(
  userName: string,
  oldPassword: string,
  newPassword: string,
  init: RequestInit = {},
): Promise<ChangePasswordResponse> {
  return apiPost<ChangePasswordResponse>('/api/v1/auth/password', {
    user_name: userName,
    old_password: oldPassword,
    new_password: newPassword,
  }, init)
}


export interface SessionResponse {
  ok: boolean
  user_name: string
  role?: string
  expires_at: string
  must_change_password?: boolean
  remaining_seconds?: number
  server_time_unix?: number
  admin_op_busy?: boolean
  op?: string
  started_at_unix?: number
  last?: unknown
}

/** 服务端会话检视：校验 token 并回填 role/expires/must_change */
export async function apiGetSession(init: RequestInit = {}): Promise<SessionResponse> {
  return apiGetSilent<SessionResponse>('/api/v1/auth/session', init)
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
