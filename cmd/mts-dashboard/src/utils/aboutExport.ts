/** About 构建信息导出（纯函数） */

import type { ClientBuildInfo } from './buildInfo.ts'

export interface ServerVersionInfo {
  version?: string
  commit?: string
  built_at?: string
  path?: string
}

export function buildAboutExport(
  input: {
    client: ClientBuildInfo
    server?: ServerVersionInfo | null
    user?: string
  },
  at = new Date(),
): {
  kind: 'mts.about'
  version: 1
  generated_at: string
  client: ClientBuildInfo
  server: ServerVersionInfo | null
  user: string
} {
  return {
    kind: 'mts.about',
    version: 1,
    generated_at: at.toISOString(),
    client: input.client,
    server: input.server
      ? {
          version: input.server.version || '',
          commit: input.server.commit || '',
          built_at: input.server.built_at || '',
          path: input.server.path || '',
        }
      : null,
    user: input.user || '',
  }
}

export function formatAboutExportPretty(
  input: {
    client: ClientBuildInfo
    server?: ServerVersionInfo | null
    user?: string
  },
  at = new Date(),
): string {
  return JSON.stringify(buildAboutExport(input, at), null, 2)
}
