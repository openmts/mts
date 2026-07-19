/// <reference types="vite/client" />

interface ImportMetaEnv {
  readonly BASE_URL: string
  readonly VITE_BASE?: string
  /** 可选 API 根前缀；默认空字符串，即相对站点根请求 /api/... */
  readonly VITE_API_BASE?: string
  readonly VITE_DASHBOARD_VERSION?: string
}

interface ImportMeta {
  readonly env: ImportMetaEnv
}
