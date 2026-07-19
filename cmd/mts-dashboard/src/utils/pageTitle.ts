/** 文档标题：route titleKey + app 名 */

import { messages, type Locale, type MessageKey } from '../i18n/messages.ts'

export function resolveRouteTitleKey(name: string | symbol | null | undefined): MessageKey | null {
  if (name == null || typeof name !== 'string') return null
  const map: Record<string, MessageKey> = {
    Login: 'login',
    ForceChangePassword: 'forceChangePassword',
    Overview: 'overview',
    Databases: 'databases',
    Users: 'users',
    Config: 'config',
    Operations: 'operations',
    Downsample: 'downsample',
    Query: 'query',
    Audit: 'audit',
    ApiSpec: 'apiSpec',
    Storage: 'storage',
    Readiness: 'readiness',
    About: 'about',
    Account: 'account',
    Write: 'write',
    AccessMatrix: 'accessMatrix',
    AccessGrants: 'accessGrants',
    Metrics: 'metrics',
    NotFound: 'notFound',
  }
  return map[name] ?? null
}

export function formatDocumentTitle(
  pageLabel: string | null | undefined,
  appName: string,
): string {
  const page = String(pageLabel || '').trim()
  const app = String(appName || '').trim() || 'MTS'
  if (!page) return app
  return `${page} · ${app}`
}

export function documentTitleForRoute(
  routeName: string | symbol | null | undefined,
  locale: Locale = 'zh',
): string {
  const table = messages[locale]
  const key = resolveRouteTitleKey(routeName)
  const page = key ? table[key] : ''
  return formatDocumentTitle(page, table.appName)
}
