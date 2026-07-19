/** API 错误码友好映射（与服务端 error-codes 主码对齐） */

export type ApiErrorLocale = 'zh' | 'en'

export interface FriendlyApiError {
  code: string
  status?: number
  title: string
  message: string
  /** 适合 toast / banner 的单行文案 */
  display: string
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
  network: { zh: '无法连接服务，请检查网络或服务状态', en: 'Cannot reach server; check network or service status' },
}

export function normalizeErrorCode(code: string | undefined | null): string {
  const c = String(code || '').trim().toLowerCase()
  if (!c) return 'internal'
  return c
}

export function friendlyApiError(
  input: { code?: string; message?: string; status?: number } | null | undefined,
  locale: ApiErrorLocale = 'zh',
): FriendlyApiError {
  const code = normalizeErrorCode(input?.code)
  const titleMap = TITLES[code] ?? TITLES.internal
  const hintMap = HINTS[code] ?? HINTS.internal
  const title = titleMap[locale]
  const hint = hintMap[locale]
  const raw = String(input?.message || '').trim()
  // 若服务端 message 已足够友好且非空，拼在 hint 后；避免重复
  let message = hint
  if (raw && raw !== hint && !raw.includes(hint)) {
    message = `${hint}（${raw}）`
  }
  const display = `[${code}] ${title}：${message}`
  return {
    code,
    status: input?.status,
    title,
    message,
    display,
  }
}

export function formatCaughtError(err: unknown, locale: ApiErrorLocale = 'zh'): string {
  if (err && typeof err === 'object') {
    const e = err as { code?: string; message?: string; status?: number; name?: string }
    if (e.name === 'APIClientError' || e.code || e.status) {
      return friendlyApiError(
        { code: e.code, message: e.message, status: e.status },
        locale,
      ).display
    }
    if (err instanceof Error && err.message) {
      // 网络类
      if (/failed to fetch|network|load failed/i.test(err.message)) {
        return friendlyApiError({ code: 'network', message: err.message }, locale).display
      }
      return friendlyApiError({ code: 'internal', message: err.message }, locale).display
    }
  }
  return friendlyApiError({ code: 'internal', message: String(err ?? '') }, locale).display
}
