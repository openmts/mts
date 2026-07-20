/** 本机客户端偏好：解析 / 默认 / 规范化（纯函数，不含密钥） */

import { normalizeLandingPath } from './landingPrefs.ts'
import { normalizeUiDensity, type UiDensity } from './densityPrefs.ts'
import type { AccountPrefsSnapshot } from './accountExport.ts'

export type ClientLocale = 'zh' | 'en'
export type ClientTheme = 'light' | 'dark'

export interface ClientPrefs {
  landing_path: string
  density: UiDensity
  sidebar_collapsed: boolean
  locale: ClientLocale
  theme: ClientTheme
}

export const DEFAULT_CLIENT_PREFS: ClientPrefs = {
  landing_path: '/',
  density: 'comfortable',
  sidebar_collapsed: false,
  locale: 'zh',
  theme: 'light',
}

export const CLIENT_PREFS_CHANGED_EVENT = 'mts-dashboard-prefs-changed'

export function normalizeClientLocale(raw: unknown): ClientLocale {
  return raw === 'en' ? 'en' : 'zh'
}

export function normalizeClientTheme(raw: unknown): ClientTheme {
  return raw === 'dark' ? 'dark' : 'light'
}

export function normalizeClientPrefs(raw: AccountPrefsSnapshot | null | undefined): ClientPrefs {
  const p = raw || {}
  return {
    landing_path: normalizeLandingPath(p.landing_path),
    density: normalizeUiDensity(p.density),
    sidebar_collapsed: !!p.sidebar_collapsed,
    locale: normalizeClientLocale(p.locale),
    theme: normalizeClientTheme(p.theme),
  }
}

export type ParseClientPrefsResult =
  | { ok: true; prefs: ClientPrefs }
  | { ok: false; error: 'empty' | 'invalid_json' | 'invalid_shape' }

/**
 * 解析导入文本：支持完整账户快照（含 prefs）或仅 prefs 对象。
 */
export function parseClientPrefsImport(text: string): ParseClientPrefsResult {
  const s = String(text ?? '').trim()
  if (!s) return { ok: false, error: 'empty' }
  let parsed: unknown
  try {
    parsed = JSON.parse(s)
  } catch {
    return { ok: false, error: 'invalid_json' }
  }
  if (parsed == null || typeof parsed !== 'object') {
    return { ok: false, error: 'invalid_shape' }
  }
  const o = parsed as Record<string, unknown>
  // 完整快照
  if (o.prefs && typeof o.prefs === 'object') {
    return { ok: true, prefs: normalizeClientPrefs(o.prefs as AccountPrefsSnapshot) }
  }
  // 直接 prefs 或扁平字段
  if (
    'landing_path' in o ||
    'density' in o ||
    'sidebar_collapsed' in o ||
    'locale' in o ||
    'theme' in o
  ) {
    return { ok: true, prefs: normalizeClientPrefs(o as AccountPrefsSnapshot) }
  }
  return { ok: false, error: 'invalid_shape' }
}

export function clientPrefsEqual(a: ClientPrefs, b: ClientPrefs): boolean {
  return (
    a.landing_path === b.landing_path &&
    a.density === b.density &&
    a.sidebar_collapsed === b.sidebar_collapsed &&
    a.locale === b.locale &&
    a.theme === b.theme
  )
}
