import { parseNavOrderMap } from './navOrder.ts'

/** 账户会话与资料导出（纯函数，不含密钥） */

export interface AccountPrefsSnapshot {
  landing_path?: string
  density?: string
  sidebar_collapsed?: boolean
  locale?: string
  theme?: string
  /** 侧栏组内导航顺序 sectionId -> paths */
  nav_order?: Record<string, string[]>
}

export function buildAccountExport(
  input: {
    username?: string
    role?: string
    session?: {
      expires_at?: string | null
      remaining?: string
      urgency?: string
    } | null
    prefs?: AccountPrefsSnapshot | null
  },
  at = new Date(),
): {
  kind: 'mts.account.snapshot'
  version: 1
  generated_at: string
  username: string
  role: string
  session: {
    expires_at: string
    remaining: string
    urgency: string
  }
  prefs: {
    landing_path: string
    density: string
    sidebar_collapsed: boolean
    locale: string
    theme: string
    nav_order: Record<string, string[]>
  }
} {
  const session = input.session || {}
  const prefs = input.prefs || {}
  const nav = parseNavOrderMap(prefs.nav_order)
  return {
    kind: 'mts.account.snapshot',
    version: 1,
    generated_at: at.toISOString(),
    username: input.username || '',
    role: input.role || '',
    session: {
      expires_at: session.expires_at || '',
      remaining: session.remaining || '',
      urgency: session.urgency || '',
    },
    prefs: {
      landing_path: prefs.landing_path || '/',
      density: prefs.density || 'comfortable',
      sidebar_collapsed: !!prefs.sidebar_collapsed,
      locale: prefs.locale || 'zh',
      theme: prefs.theme || 'light',
      nav_order: nav,
    },
  }
}

export function formatAccountExportPretty(
  input: Parameters<typeof buildAccountExport>[0],
  at = new Date(),
): string {
  return JSON.stringify(buildAccountExport(input, at), null, 2)
}
