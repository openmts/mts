/** 就绪/演练归档模板（JSON + Markdown），供运维交接下载 */

import type { ReadinessScoreBreakdown } from './readinessScore.ts'
import type { ReadinessState } from './readinessState.ts'
import { completedIds } from './readinessState.ts'

export interface DoctorArchiveSummary {
  loaded: boolean
  ok?: boolean
  http_tls_enabled?: boolean | null
  warn_count?: number
  checks?: { level: string; code: string; message: string }[]
  error?: string
}

export interface ReadinessArchiveInput {
  operator?: string
  note?: string
  state: ReadinessState
  score: ReadinessScoreBreakdown
  doctor: DoctorArchiveSummary
  now?: string
}

export interface ReadinessArchivePayload {
  version: 1
  kind: 'mts.readiness.archive'
  archived_at: string
  operator: string
  note: string
  score: ReadinessScoreBreakdown
  doctor: DoctorArchiveSummary
  checklist: {
    production: string[]
    edgeHttps: string[]
    backupSchedule: string[]
  }
  state: ReadinessState
}

export function buildReadinessArchive(input: ReadinessArchiveInput): ReadinessArchivePayload {
  const archived_at = input.now ?? new Date().toISOString()
  return {
    version: 1,
    kind: 'mts.readiness.archive',
    archived_at,
    operator: (input.operator ?? '').trim() || 'unknown',
    note: (input.note ?? '').trim(),
    score: { ...input.score, reasons: [...input.score.reasons] },
    doctor: {
      loaded: input.doctor.loaded,
      ok: input.doctor.ok,
      http_tls_enabled: input.doctor.http_tls_enabled ?? null,
      warn_count: input.doctor.warn_count ?? 0,
      checks: (input.doctor.checks ?? []).map((c) => ({ ...c })),
      error: input.doctor.error,
    },
    checklist: {
      production: completedIds(input.state.production).sort(),
      edgeHttps: completedIds(input.state.edgeHttps).sort(),
      backupSchedule: completedIds(input.state.backupSchedule).sort(),
    },
    state: {
      production: { ...input.state.production },
      edgeHttps: { ...input.state.edgeHttps },
      backupSchedule: { ...input.state.backupSchedule },
      updatedAt: input.state.updatedAt,
    },
  }
}

export function formatReadinessArchiveMarkdown(a: ReadinessArchivePayload): string {
  const lines: string[] = [
    '# MTS 可商用就绪演练归档',
    '',
    `- 时间：${a.archived_at}`,
    `- 操作者：${a.operator}`,
    `- 就绪评分：${a.score.total}%`,
    `- 分项：清单 ${a.score.checklist}% / HTTPS ${a.score.edgeHttps}% / 备份 ${a.score.backupSchedule}% / Doctor ${a.score.doctor}%`,
  ]
  if (a.score.reasons.length) {
    lines.push(`- 扣分原因：${a.score.reasons.join(', ')}`)
  }
  if (a.note) {
    lines.push(`- 备注：${a.note}`)
  }
  lines.push('', '## Doctor', '')
  if (!a.doctor.loaded) {
    lines.push(`- 未加载${a.doctor.error ? `：${a.doctor.error}` : ''}`)
  } else {
    lines.push(`- ok：${a.doctor.ok === true ? 'yes' : 'no'}`)
    lines.push(`- http_tls_enabled：${String(a.doctor.http_tls_enabled)}`)
    lines.push(`- warn_count：${a.doctor.warn_count ?? 0}`)
    for (const c of a.doctor.checks ?? []) {
      lines.push(`- [${c.level}] ${c.code}: ${c.message}`)
    }
  }
  lines.push('', '## 已完成清单', '')
  lines.push(`- production: ${a.checklist.production.join(', ') || '—'}`)
  lines.push(`- edgeHttps: ${a.checklist.edgeHttps.join(', ') || '—'}`)
  lines.push(`- backupSchedule: ${a.checklist.backupSchedule.join(', ') || '—'}`)
  lines.push('', '---', '', '_本文件由 Dashboard 就绪中心生成，不代表生产人工验收已完成。_', '')
  return lines.join('\n')
}

export function archiveFilenames(at = new Date()): { json: string; md: string } {
  const stamp = at.toISOString().replace(/[:.]/g, '-').slice(0, 19)
  return {
    json: `mts-readiness-archive-${stamp}.json`,
    md: `mts-readiness-archive-${stamp}.md`,
  }
}
