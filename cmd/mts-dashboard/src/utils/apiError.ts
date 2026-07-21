/** API 错误码友好映射（与服务端 error-codes 主码对齐） */

import { isAdminHeavyBusyMessage, parseAdminHeavyBusyOp } from './adminOpBusy.ts'

export type ApiErrorLocale = 'zh' | 'en'

export interface FriendlyApiError {
  code: string
  status?: number
  title: string
  message: string
  /** 适合 toast / banner 的用户主文案（不含裸 code 前缀） */
  display: string
  /** 诊断用 code，可放在 title/aria 次要位置 */
  technicalCode: string
}

const TITLES: Record<string, { zh: string; en: string }> = {
  bad_request: { zh: '请求无效', en: 'Bad request' },
  unauthenticated: { zh: '未认证', en: 'Unauthenticated' },
  permission_denied: { zh: '权限不足', en: 'Permission denied' },
  resource_exhausted: { zh: '资源耗尽', en: 'Resource exhausted' },
  not_found: { zh: '资源不存在', en: 'Not found' },
  already_exists: { zh: '资源已存在', en: 'Already exists' },
  internal: { zh: '服务内部错误', en: 'Internal error' },
  canceled: { zh: '请求已取消', en: 'Canceled' },
  timeout: { zh: '请求超时', en: 'Request timeout' },
  network: { zh: '网络异常', en: 'Network error' },
}

const HINTS: Record<string, { zh: string; en: string }> = {
  bad_request: { zh: '请检查输入参数后重试', en: 'Check request parameters and retry' },
  unauthenticated: { zh: '请重新登录', en: 'Please sign in again' },
  permission_denied: { zh: '当前账号无此操作权限', en: 'Your account lacks permission for this action' },
  resource_exhausted: { zh: '请求超过限制，请缩小范围或稍后重试', en: 'Limit exceeded; narrow the request or retry later' },
  not_found: { zh: '目标不存在或已删除', en: 'Target does not exist or was removed' },
  already_exists: { zh: '同名资源已存在', en: 'A resource with the same name already exists' },
  internal: { zh: '请稍后重试；若持续出现请查看服务日志', en: 'Retry later; check server logs if it persists' },
  canceled: { zh: '操作已取消', en: 'Operation canceled' },
  timeout: { zh: '服务响应过慢，请缩小范围或稍后重试', en: 'Server took too long; narrow the request or retry later' },
  network: { zh: '无法连接服务，请检查网络或服务状态', en: 'Cannot reach server; check network or service status' },
}

const KNOWN_CODES = new Set(Object.keys(TITLES))

export function normalizeErrorCode(code: string | undefined | null): string {
  const c = String(code || '').trim().toLowerCase()
  if (!c) return 'internal'
  return c
}

/** HTTP status → 主错误码（code 缺失或 unknown 时使用） */
export function errorCodeFromStatus(status: number | undefined | null): string | null {
  if (status == null || !Number.isFinite(status)) return null
  const s = Math.trunc(status)
  if (s === 400) return 'bad_request'
  if (s === 401) return 'unauthenticated'
  if (s === 403) return 'permission_denied'
  if (s === 404) return 'not_found'
  if (s === 409) return 'already_exists'
  if (s === 429) return 'resource_exhausted'
  if (s === 408) return 'timeout'
  if (s === 499) return 'canceled'
  if (s >= 500 && s < 600) return 'internal'
  return null
}

export function resolveErrorCode(
  code: string | undefined | null,
  status?: number | null,
): string {
  const normalized = normalizeErrorCode(code)
  if (KNOWN_CODES.has(normalized) && (code != null && String(code).trim() !== '')) {
    return normalized
  }
  // code 为空 / unknown：优先 status
  const fromStatus = errorCodeFromStatus(status ?? undefined)
  if (fromStatus) return fromStatus
  if (KNOWN_CODES.has(normalized)) return normalized
  return 'internal'
}

/** 读取 Dashboard locale（与 useI18n 的 localStorage key 对齐） */
export function resolveApiErrorLocale(explicit?: ApiErrorLocale | null): ApiErrorLocale {
  if (explicit === 'zh' || explicit === 'en') return explicit
  try {
    const v = localStorage.getItem('mts_locale')
    if (v === 'en' || v === 'zh') return v
  } catch {
    /* ignore */
  }
  return 'zh'
}

export function friendlyApiError(
  input: { code?: string; message?: string; status?: number } | null | undefined,
  locale: ApiErrorLocale = 'zh',
): FriendlyApiError {
  const technicalCode = normalizeErrorCode(input?.code)
  const code = resolveErrorCode(input?.code, input?.status)
  const titleMap = TITLES[code] ?? TITLES.internal
  const hintMap = HINTS[code] ?? HINTS.internal
  let title = titleMap[locale]
  let hint = hintMap[locale]
  const raw = String(input?.message || '').trim()
  // 管理重操作互斥：resource_exhausted 但语义是 admin busy，文案对齐运维占用
  if (code === 'resource_exhausted' && isAdminHeavyBusyMessage(raw)) {
    title = locale === 'en' ? 'Admin operation busy' : '管理重操作占用中'
    const busyOp = parseAdminHeavyBusyOp(raw)
    hint =
      locale === 'en'
        ? busyOp
          ? `Another admin heavy op is running (${busyOp}); wait or open Operations`
          : 'Another admin heavy op (flush/compact/snapshot/restore) is running; wait or open Operations'
        : busyOp
          ? `另一管理重操作进行中（${busyOp}），请等待或到运维页查看状态`
          : '另一管理重操作（flush/compact/快照/恢复）进行中，请等待或到运维页查看状态'
  }
  // 服务端 message 若仅为 code/snake 或与 title 重复，则不拼入主文案
  let message = hint
  if (raw && raw !== hint && !raw.includes(hint)) {
    const rawLower = raw.toLowerCase()
    const looksLikeBareCode =
      rawLower === code ||
      rawLower === technicalCode ||
      (/^[a-z][a-z0-9_]+$/.test(rawLower) && KNOWN_CODES.has(rawLower))
    const looksLikeTransportNoise =
      /^(request )?(canceled|cancelled|timeout|timed out)$/i.test(rawLower) ||
      rawLower === 'the user aborted a request.' ||
      rawLower === 'aborterror'
    if (!looksLikeBareCode && !looksLikeTransportNoise && raw !== title) {
      message = locale === 'en' ? `${hint} (${raw})` : `${hint}（${raw}）`
    }
  }
  if (code === 'resource_exhausted' && isAdminHeavyBusyMessage(raw)) {
    message = hint
  }
  // 主文案不以 [code] 开头；诊断 code 放 technicalCode
  const display = `${title}：${message}`
  return {
    code,
    status: input?.status,
    title,
    message,
    display,
    technicalCode: technicalCode === 'internal' && code !== 'internal' ? code : technicalCode,
  }
}

/** 从未知错误解析主错误码（与 formatCaughtError 规则对齐） */
export function resolveCaughtErrorCode(err: unknown): string {
  if (typeof err === 'string') {
    if (/cancel|abort|取消/i.test(err)) return 'canceled'
    if (/timeout|timed out|超时/i.test(err)) return 'timeout'
    return 'internal'
  }
  if (err && typeof err === 'object') {
    const e = err as { code?: string; message?: string; status?: number; name?: string }
    if (e.name === 'AbortError') return 'canceled'
    if (e.name === 'APIClientError' || e.code || e.status) {
      return resolveErrorCode(e.code, e.status)
    }
    if (err instanceof Error && err.message) {
      if (/failed to fetch|network|load failed|networkerror/i.test(err.message)) {
        return 'network'
      }
      if (/timeout|timed out|超时/i.test(err.message)) {
        return 'timeout'
      }
      if (/cancel|abort|取消/i.test(err.message)) {
        return 'canceled'
      }
      return 'internal'
    }
    if (e.message) {
      if (/cancel|abort|取消/i.test(e.message)) return 'canceled'
      if (/timeout|timed out|超时/i.test(e.message)) return 'timeout'
    }
  }
  return 'internal'
}

export function isCanceledError(err: unknown): boolean {
  return resolveCaughtErrorCode(err) === 'canceled'
}

export function isTimeoutError(err: unknown): boolean {
  return resolveCaughtErrorCode(err) === 'timeout'
}

export function formatCaughtError(err: unknown, locale?: ApiErrorLocale | null): string {
  const loc = resolveApiErrorLocale(locale)
  if (err && typeof err === 'object') {
    const e = err as { code?: string; message?: string; status?: number; name?: string }
    if (e.name === 'APIClientError' || e.code || e.status) {
      return friendlyApiError(
        { code: e.code, message: e.message, status: e.status },
        loc,
      ).display
    }
    if (err instanceof Error && err.message) {
      if (/failed to fetch|network|load failed|networkerror/i.test(err.message)) {
        return friendlyApiError({ code: 'network', message: err.message }, loc).display
      }
      if (e.name === 'AbortError') {
        return friendlyApiError({ code: 'canceled', message: err.message }, loc).display
      }
      if (/timeout|timed out/i.test(err.message)) {
        return friendlyApiError({ code: 'timeout', message: err.message }, loc).display
      }
      return friendlyApiError({ code: 'internal', message: err.message }, loc).display
    }
    if (e.name === 'AbortError') {
      return friendlyApiError({ code: 'canceled', message: String(e.message || '') }, loc).display
    }
  }
  return friendlyApiError({ code: 'internal', message: String(err ?? '') }, loc).display
}
