const API_BASE = ''

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

let adminToken = ''

export function setAdminToken(token: string) {
  adminToken = token
}

export function getAdminToken(): string {
  return adminToken
}

async function request<T>(path: string, options: RequestInit = {}): Promise<T> {
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    ...(options.headers as Record<string, string>),
  }
  if (adminToken) {
    headers['X-MTS-Admin-Token'] = adminToken
  }
  const response = await fetch(`${API_BASE}${path}`, {
    ...options,
    headers,
  })
  if (!response.ok) {
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
