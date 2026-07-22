/** Overview 一键健康报告（聚合 overview + downsample + 可商用交接 + 可选 ops，纯函数） */

import {
  buildOverviewExport,
  type OverviewDoctorCheck,
  type OverviewHealthCheck,
} from './overviewExport.ts'
import {
  downsampleStatusSummaryTone,
  formatDownsampleStatusSummaryLine,
  type DownsampleStatusSummaryInput,
} from './downsampleStatusSummary.ts'
import {
  buildCommercialHandoffSummary,
  formatPasswordPolicyHandoffLine,
  formatSessionCalibrationHandoffLine,
  type CommercialHandoffSummary,
} from './commercialHandoffSummary.ts'

export const HEALTH_REPORT_KIND = 'mts.health.report' as const
/** v2：附带 commercial_handoff（密码策略 + 会话校准） */
export const HEALTH_REPORT_VERSION = 2 as const

export interface HealthReportInput {
  connectivity?: string
  healthy?: boolean | null
  ready?: boolean | null
  health_reasons?: string[]
  health_checks?: OverviewHealthCheck[]
  maintenance_errors?: string[]
  memory?: object | null
  compaction?: object | null
  maintenance?: object | null
  doctor_tls?: boolean | null
  doctor_checks?: OverviewDoctorCheck[]
  readiness_total?: number
  readiness_level?: string
  server_version?: { version?: string; commit?: string; built_at?: string } | null
  client?: object | null
  last_refreshed?: string
  downsample_status_summary?: DownsampleStatusSummaryInput | null
  /** 可选：运维统计快照片段 */
  ops_stats?: object | null
  /** 可选：直接传入交接摘要；否则按会话字段构造 */
  commercial_handoff?: CommercialHandoffSummary | null
  session_expires_at?: string | null
  session_remaining_seconds?: number | null
  session_checked_at_ms?: number | null
}

export function buildHealthReport(input: HealthReportInput, at = new Date()) {
  const overview = buildOverviewExport(
    {
      connectivity: input.connectivity,
      healthy: input.healthy,
      ready: input.ready,
      health_reasons: input.health_reasons,
      health_checks: input.health_checks,
      maintenance_errors: input.maintenance_errors,
      memory: input.memory,
      compaction: input.compaction,
      maintenance: input.maintenance,
      doctor_tls: input.doctor_tls,
      doctor_checks: input.doctor_checks,
      readiness_total: input.readiness_total,
      readiness_level: input.readiness_level,
      server_version: input.server_version,
      client: input.client,
      last_refreshed: input.last_refreshed,
      downsample_status_summary: input.downsample_status_summary,
    },
    at,
  )
  const ds = overview.downsample_status_summary
  const tone = ds ? downsampleStatusSummaryTone(ds) : 'ok'
  const commercial_handoff =
    input.commercial_handoff ??
    buildCommercialHandoffSummary({
      expiresAtIso: input.session_expires_at,
      serverRemainingSec: input.session_remaining_seconds,
      checkedAtMs: input.session_checked_at_ms,
      nowMs: at.getTime(),
    })
  return {
    kind: HEALTH_REPORT_KIND,
    version: HEALTH_REPORT_VERSION,
    generated_at: at.toISOString(),
    disclaimer:
      'Dashboard 健康报告汇总，用于交接与扫视；不代表边缘证书、异地备份或人工验收已完成。',
    overview,
    downsample_status_summary: ds,
    downsample_tone: tone,
    ops_stats: input.ops_stats ?? null,
    commercial_handoff,
  }
}

export function formatHealthReportPretty(input: HealthReportInput, at = new Date()): string {
  return JSON.stringify(buildHealthReport(input, at), null, 2)
}

export function healthReportFilename(at = new Date()): string {
  const stamp = at.toISOString().replace(/[:.]/g, '-').slice(0, 19)
  return `mts-health-report-${stamp}.json`
}

export function formatHealthReportMarkdown(
  input: HealthReportInput,
  at = new Date(),
): string {
  const r = buildHealthReport(input, at)
  const o = r.overview
  const lines: string[] = [
    '# MTS health report',
    '',
    `- generated_at: ${r.generated_at}`,
    `- connectivity: ${o.connectivity || '—'}`,
    `- healthy: ${o.healthy == null ? '—' : String(o.healthy)}`,
    `- ready: ${o.ready == null ? '—' : String(o.ready)}`,
    `- readiness: ${o.readiness_total == null ? '—' : `${o.readiness_total}%`} (${o.readiness_level || '—'})`,
  ]
  if (o.server_version?.version) {
    lines.push(
      `- server: ${o.server_version.version}${o.server_version.commit ? ` @ ${o.server_version.commit}` : ''}`,
    )
  }
  if (r.downsample_status_summary) {
    lines.push('', '## Downsample', '')
    lines.push(`- tone: ${r.downsample_tone}`)
    lines.push(`- ${formatDownsampleStatusSummaryLine(r.downsample_status_summary)}`)
  }
  if (r.commercial_handoff) {
    lines.push('', '## Commercial handoff', '')
    lines.push(`- password_policy: ${formatPasswordPolicyHandoffLine(r.commercial_handoff.password_policy)}`)
    lines.push(
      `- session_calibration: ${formatSessionCalibrationHandoffLine(r.commercial_handoff.session_calibration)}`,
    )
  }
  if (Array.isArray(o.health_reasons) && o.health_reasons.length) {
    lines.push('', '## Health reasons', '')
    for (const x of o.health_reasons.slice(0, 12)) lines.push(`- ${x}`)
  }
  if (Array.isArray(o.maintenance_errors) && o.maintenance_errors.length) {
    lines.push('', '## Maintenance errors', '')
    for (const x of o.maintenance_errors.slice(0, 12)) lines.push(`- ${x}`)
  }
  lines.push('', '---', '', `_${r.disclaimer}_`, '')
  return lines.join('\n')
}

export function formatCommercialHandoffClipboardText(
  handoff: CommercialHandoffSummary,
  at = new Date(),
): string {
  return [
    'MTS commercial handoff',
    `generated_at: ${at.toISOString()}`,
    `password_policy: ${formatPasswordPolicyHandoffLine(handoff.password_policy)}`,
    `session_calibration: ${formatSessionCalibrationHandoffLine(handoff.session_calibration)}`,
    '',
  ].join('\n')
}

export function healthReportFilenames(at = new Date()): { json: string; md: string } {
  const stamp = at.toISOString().replace(/[:.]/g, '-').slice(0, 19)
  return {
    json: `mts-health-report-${stamp}.json`,
    md: `mts-health-report-${stamp}.md`,
  }
}
