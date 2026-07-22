/** 与 mts-server error-codes 契约对齐的可操作元数据（纯函数） */

export interface ErrorCodeContract {
  code: string
  http_status: number
  grpc_code: string
  description: string
  retryable?: boolean
  category?: string
  remediation?: string
  dashboard_path?: string
}

export interface ErrorActionLink {
  labelKey: 'errorCodeOpenContract' | 'errorCodeOpenRemediation'
  path: string
}

const FALLBACK_PATHS: Record<string, string> = {
  bad_request: '/config?error_q=bad_request#config-error-codes',
  unauthenticated: '/account#account-session',
  permission_denied: '/access-matrix',
  resource_exhausted: '/operations',
  not_found: '/databases',
  already_exists: '/databases',
  internal: '/operations',
  timeout: '/config?error_q=resource_exhausted#config-error-codes',
  canceled: '/config?error_q=#config-error-codes',
}

/** 规范化服务端/本地错误码规格 */
export function normalizeErrorCodeContract(raw: unknown): ErrorCodeContract | null {
  if (!raw || typeof raw !== 'object') return null
  const o = raw as Record<string, unknown>
  const code = String(o.code ?? '').trim()
  if (!code) return null
  const http = Number(o.http_status ?? o.httpStatus ?? 0)
  return {
    code,
    http_status: Number.isFinite(http) ? http : 0,
    grpc_code: String(o.grpc_code ?? o.grpcCode ?? ''),
    description: String(o.description ?? ''),
    retryable: Boolean(o.retryable),
    category: String(o.category ?? '') || undefined,
    remediation: String(o.remediation ?? '') || undefined,
    dashboard_path: String(o.dashboard_path ?? o.dashboardPath ?? '') || undefined,
  }
}

export function indexErrorCodeContracts(list: unknown[] | null | undefined): Map<string, ErrorCodeContract> {
  const map = new Map<string, ErrorCodeContract>()
  if (!Array.isArray(list)) return map
  for (const item of list) {
    const n = normalizeErrorCodeContract(item)
    if (n) map.set(n.code.toLowerCase(), n)
  }
  return map
}

export function lookupErrorCodeContract(
  code: string | null | undefined,
  index: Map<string, ErrorCodeContract> | null | undefined,
): ErrorCodeContract | null {
  const key = String(code ?? '').trim().toLowerCase()
  if (!key) return null
  if (index?.has(key)) return index.get(key) || null
  return null
}

/** 配置页深链：按错误码筛选 error-codes 表 */
export function configErrorCodeDeepLink(code: string | null | undefined): string {
  const c = String(code ?? '').trim()
  if (!c) return '/config#config-error-codes'
  return `/config?error_q=${encodeURIComponent(c)}#config-error-codes`
}

/** 优先服务端 dashboard_path，否则本地回落 */
export function remediationPathForCode(
  code: string | null | undefined,
  contract?: ErrorCodeContract | null,
): string {
  const fromServer = (contract?.dashboard_path || '').trim()
  if (fromServer) return fromServer
  const key = String(code ?? '').trim().toLowerCase()
  return FALLBACK_PATHS[key] || configErrorCodeDeepLink(key || null)
}

export function errorActionLinks(
  code: string | null | undefined,
  contract?: ErrorCodeContract | null,
): ErrorActionLink[] {
  const links: ErrorActionLink[] = []
  const contractPath = configErrorCodeDeepLink(code)
  links.push({ labelKey: 'errorCodeOpenContract', path: contractPath })
  const rem = remediationPathForCode(code, contract)
  if (rem && rem !== contractPath) {
    links.push({ labelKey: 'errorCodeOpenRemediation', path: rem })
  }
  return links
}

export function isContractRetryable(
  code: string | null | undefined,
  contract?: ErrorCodeContract | null,
  adminOpBusy = false,
): boolean {
  if (adminOpBusy) return true
  if (contract && typeof contract.retryable === 'boolean') return contract.retryable
  const key = String(code ?? '').trim().toLowerCase()
  return key === 'resource_exhausted' || key === 'internal' || key === 'timeout'
}

export function formatRemediationHint(
  contract: ErrorCodeContract | null | undefined,
  locale: 'zh' | 'en' = 'zh',
): string {
  const rem = (contract?.remediation || '').trim()
  if (!rem) return ''
  if (locale === 'en') return rem
  // 服务端 remediation 为英文运维说明；中文环境加前缀便于扫视
  return `建议：${rem}`
}
