/** 可商用验收材料一键导出包（纯函数，便于单测） */

import type { ReadinessArchivePayload } from './readinessArchive.ts'
import type { OpsActionEntry } from './opsActionLog.ts'
import type { ClientBuildInfo } from './buildInfo.ts'

export const ACCEPTANCE_PACK_KIND = 'mts.acceptance.pack' as const
export const ACCEPTANCE_PACK_VERSION = 1 as const

export interface ServerVersionInfo {
  version: string
  commit?: string
  built_at?: string
}

export interface AcceptancePackInput {
  archive: ReadinessArchivePayload
  client: ClientBuildInfo
  server?: ServerVersionInfo | null
  opsActions?: OpsActionEntry[]
  operator?: string
  note?: string
  now?: string
}

export interface AcceptancePackPayload {
  version: typeof ACCEPTANCE_PACK_VERSION
  kind: typeof ACCEPTANCE_PACK_KIND
  exported_at: string
  operator: string
  note: string
  disclaimer: string
  client: ClientBuildInfo
  server: ServerVersionInfo | null
  readiness: ReadinessArchivePayload
  ops_actions: OpsActionEntry[]
}

export function buildAcceptancePack(input: AcceptancePackInput): AcceptancePackPayload {
  const exported_at = input.now ?? new Date().toISOString()
  const server = input.server
    ? {
        version: String(input.server.version || ''),
        commit: input.server.commit ? String(input.server.commit) : undefined,
        built_at: input.server.built_at ? String(input.server.built_at) : undefined,
      }
    : null
  return {
    version: ACCEPTANCE_PACK_VERSION,
    kind: ACCEPTANCE_PACK_KIND,
    exported_at,
    operator: (input.operator ?? input.archive.operator ?? '').trim() || 'unknown',
    note: (input.note ?? '').trim(),
    disclaimer:
      '本包由 Dashboard 就绪中心生成，用于交接与自检材料汇总，不代表生产人工验收、边缘证书或异地备份已完成。',
    client: { ...input.client },
    server,
    readiness: input.archive,
    ops_actions: (input.opsActions ?? []).map((x) => ({ ...x })),
  }
}

export function formatAcceptancePackMarkdown(pack: AcceptancePackPayload): string {
  const lines: string[] = [
    '# MTS 可商用验收材料包',
    '',
    `- 导出时间：${pack.exported_at}`,
    `- 操作者：${pack.operator}`,
    `- 就绪评分：${pack.readiness.score.total}%`,
    `- 客户端：${pack.client.name} ${pack.client.version} (${pack.client.mode})`,
  ]
  if (pack.server) {
    lines.push(
      `- 服务端：${pack.server.version}${pack.server.commit ? ` @ ${pack.server.commit}` : ''}${
        pack.server.built_at ? ` · built ${pack.server.built_at}` : ''
      }`,
    )
  } else {
    lines.push('- 服务端：未加载')
  }
  if (pack.note) lines.push(`- 备注：${pack.note}`)
  lines.push('', '## 就绪分项', '')
  lines.push(
    `- 清单 ${pack.readiness.score.checklist}% / HTTPS ${pack.readiness.score.edgeHttps}% / 备份 ${pack.readiness.score.backupSchedule}% / Doctor ${pack.readiness.score.doctor}%`,
  )
  if (pack.readiness.score.reasons.length) {
    lines.push(`- 扣分原因：${pack.readiness.score.reasons.join(', ')}`)
  }
  lines.push('', '## Doctor', '')
  if (!pack.readiness.doctor.loaded) {
    lines.push(`- 未加载${pack.readiness.doctor.error ? `：${pack.readiness.doctor.error}` : ''}`)
  } else {
    lines.push(`- ok：${pack.readiness.doctor.ok === true ? 'yes' : 'no'}`)
    lines.push(`- http_tls_enabled：${String(pack.readiness.doctor.http_tls_enabled)}`)
    lines.push(`- warn_count：${pack.readiness.doctor.warn_count ?? 0}`)
  }
  lines.push('', '## 运维操作历史（会话）', '')
  if (!pack.ops_actions.length) {
    lines.push('- （空）')
  } else {
    for (const a of pack.ops_actions.slice(0, 20)) {
      lines.push(`- [${a.status}] ${a.kind} @ ${new Date(a.at).toISOString()}: ${a.message}`)
    }
    if (pack.ops_actions.length > 20) {
      lines.push(`- … 共 ${pack.ops_actions.length} 条`)
    }
  }
  lines.push('', '---', '', `_${pack.disclaimer}_`, '')
  return lines.join('\n')
}

export function acceptancePackFilenames(at = new Date()): { json: string; md: string } {
  const stamp = at.toISOString().replace(/[:.]/g, '-').slice(0, 19)
  return {
    json: `mts-acceptance-pack-${stamp}.json`,
    md: `mts-acceptance-pack-${stamp}.md`,
  }
}
