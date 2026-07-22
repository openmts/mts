/** Overview 运维快照导出（纯函数） */

import {
  normalizeDownsampleStatusSummary,
  type DownsampleStatusSummaryInput,
} from './downsampleStatusSummary.ts'


export interface OverviewHealthCheck {
  name: string
  status: string
  reason?: string
}

export interface OverviewDoctorCheck {
  level: string
  code: string
  message: string
}

export function buildOverviewExport(
  input: {
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
    /** 降采样 statuses summary（可商用扫视） */
    downsample_status_summary?: DownsampleStatusSummaryInput | null
  },
  at = new Date(),
): {
  kind: 'mts.overview.snapshot'
  version: 1
  generated_at: string
  connectivity: string
  healthy: boolean | null
  ready: boolean | null
  health_reasons: string[]
  health_checks: OverviewHealthCheck[]
  maintenance_errors: string[]
  memory: object | null
  compaction: object | null
  maintenance: object | null
  doctor_tls: boolean | null
  doctor_checks: OverviewDoctorCheck[]
  readiness_total: number | null
  readiness_level: string
  server_version: { version?: string; commit?: string; built_at?: string } | null
  client: object | null
  last_refreshed: string
  downsample_status_summary: Required<DownsampleStatusSummaryInput> | null
} {
  return {
    kind: 'mts.overview.snapshot',
    version: 1,
    generated_at: at.toISOString(),
    connectivity: input.connectivity || '',
    healthy: input.healthy ?? null,
    ready: input.ready ?? null,
    health_reasons: Array.isArray(input.health_reasons) ? input.health_reasons : [],
    health_checks: Array.isArray(input.health_checks) ? input.health_checks : [],
    maintenance_errors: Array.isArray(input.maintenance_errors) ? input.maintenance_errors : [],
    memory: input.memory ?? null,
    compaction: input.compaction ?? null,
    maintenance: input.maintenance ?? null,
    doctor_tls: input.doctor_tls ?? null,
    doctor_checks: Array.isArray(input.doctor_checks) ? input.doctor_checks : [],
    readiness_total: typeof input.readiness_total === 'number' ? input.readiness_total : null,
    readiness_level: input.readiness_level || '',
    server_version: input.server_version ?? null,
    client: input.client ?? null,
    last_refreshed: input.last_refreshed || '',
    downsample_status_summary: input.downsample_status_summary
      ? normalizeDownsampleStatusSummary(input.downsample_status_summary)
      : null,
  }
}

export function formatOverviewExportPretty(
  input: Parameters<typeof buildOverviewExport>[0],
  at = new Date(),
): string {
  return JSON.stringify(buildOverviewExport(input, at), null, 2)
}
