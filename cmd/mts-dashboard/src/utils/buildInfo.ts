/** 前端构建元信息（不依赖服务端） */

export interface ClientBuildInfo {
  name: string
  version: string
  mode: string
  base: string
  apiBase: string
}

function envString(key: string, fallback = ''): string {
  try {
    const env = (import.meta as ImportMeta & { env?: Record<string, unknown> }).env
    const v = env?.[key]
    return v == null ? fallback : String(v)
  } catch {
    return fallback
  }
}

export function clientBuildInfo(): ClientBuildInfo {
  return {
    name: 'mts-dashboard',
    version: envString('VITE_DASHBOARD_VERSION', '0.0.0'),
    mode: envString('MODE', 'production'),
    base: envString('BASE_URL', '/'),
    apiBase: envString('VITE_API_BASE', ''),
  }
}
