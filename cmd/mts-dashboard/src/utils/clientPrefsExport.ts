/** 本机偏好独立导出（纯函数，不含密钥） */

import type { ClientPrefs } from './clientPrefs.ts'
import { normalizeClientPrefs } from './clientPrefs.ts'

export interface ClientPrefsExportPayload {
  kind: 'mts.client.prefs'
  version: 1
  generated_at: string
  prefs: ClientPrefs
}

export function buildClientPrefsExport(
  prefs: Partial<ClientPrefs> | null | undefined,
  now = new Date(),
): ClientPrefsExportPayload {
  return {
    kind: 'mts.client.prefs',
    version: 1,
    generated_at: now.toISOString(),
    prefs: normalizeClientPrefs(prefs || {}),
  }
}

export function formatClientPrefsExportPretty(
  prefs: Partial<ClientPrefs> | null | undefined,
  now = new Date(),
): string {
  return JSON.stringify(buildClientPrefsExport(prefs, now), null, 2)
}
