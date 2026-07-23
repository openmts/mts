/** Overview 页 doctor/contract/健康路径扫视摘要（纯函数） */

export interface OverviewDoctorScan {
  path: string
  check_count: number
  ok_count: number
  warn_count: number
  error_count: number
  http_tls_enabled: boolean | null
  tone: 'ok' | 'warn' | 'bad' | 'unknown'
}

export interface OverviewContractScan {
  path: string
  loaded: boolean
  complete: boolean
  enabled_count: number
  total_features: number
  missing_required: string[]
  tone: 'ok' | 'warn' | 'bad' | 'unknown'
}

export interface OverviewPathsScan {
  doctor_path: string
  contract_path: string
  health_path: string
  memory_path: string
  compaction_path: string
  maintenance_path: string
  version_path: string
  path_count: number
  tone: 'ok' | 'warn' | 'unknown'
}

export interface OverviewScanSummary {
  doctor: OverviewDoctorScan
  contract: OverviewContractScan
  paths: OverviewPathsScan
  tone: 'ok' | 'warn' | 'bad' | 'unknown'
}

export function buildOverviewDoctorScan(input: {
  path?: string | null
  checks?: Array<{ level?: string }> | null
  httpTlsEnabled?: boolean | null
}): OverviewDoctorScan {
  const path = String(input.path || '').trim() || '/api/v1/admin/doctor'
  const checks = Array.isArray(input.checks) ? input.checks : []
  let ok_count = 0
  let warn_count = 0
  let error_count = 0
  for (const c of checks) {
    const level = String(c?.level || '').toLowerCase()
    if (level === 'error' || level === 'fail' || level === 'critical') error_count += 1
    else if (level === 'warn' || level === 'warning') warn_count += 1
    else ok_count += 1
  }
  const check_count = checks.length
  const http_tls_enabled =
    input.httpTlsEnabled === true || input.httpTlsEnabled === false
      ? input.httpTlsEnabled
      : null
  let tone: OverviewDoctorScan['tone'] = 'unknown'
  if (check_count === 0) tone = 'unknown'
  else if (error_count > 0) tone = 'bad'
  else if (warn_count > 0) tone = 'warn'
  else tone = 'ok'
  return {
    path,
    check_count,
    ok_count,
    warn_count,
    error_count,
    http_tls_enabled,
    tone,
  }
}

export function buildOverviewContractScan(input: {
  path?: string | null
  loaded?: boolean
  complete?: boolean
  enabledCount?: number
  totalFeatures?: number
  missingRequired?: string[] | null
}): OverviewContractScan {
  const path = String(input.path || '').trim() || '/api/v1/data/contract'
  const loaded = Boolean(input.loaded)
  const complete = Boolean(input.complete)
  const enabled_count = finiteNonNeg(input.enabledCount)
  const total_features = finiteNonNeg(input.totalFeatures)
  const missing_required = Array.isArray(input.missingRequired)
    ? input.missingRequired.map((x) => String(x || '').trim()).filter(Boolean)
    : []
  let tone: OverviewContractScan['tone'] = 'unknown'
  if (!loaded) tone = 'unknown'
  else if (!complete || missing_required.length) tone = 'warn'
  else tone = 'ok'
  return {
    path,
    loaded,
    complete,
    enabled_count,
    total_features,
    missing_required,
    tone,
  }
}

export function buildOverviewPathsScan(input: {
  doctorPath?: string | null
  contractPath?: string | null
  healthPath?: string | null
  memoryPath?: string | null
  compactionPath?: string | null
  maintenancePath?: string | null
  versionPath?: string | null
}): OverviewPathsScan {
  const doctor_path = strPath(input.doctorPath, '/api/v1/admin/doctor')
  const contract_path = strPath(input.contractPath, '/api/v1/data/contract')
  const health_path = strPath(input.healthPath, '/api/v1/admin/health')
  const memory_path = strPath(input.memoryPath, '/api/v1/admin/stats/storage-memory')
  const compaction_path = strPath(input.compactionPath, '/api/v1/admin/stats/compaction')
  const maintenance_path = strPath(input.maintenancePath, '/api/v1/admin/stats/maintenance')
  const version_path = strPath(input.versionPath, '/api/v1/admin/version')
  const list = [
    doctor_path,
    contract_path,
    health_path,
    memory_path,
    compaction_path,
    maintenance_path,
    version_path,
  ]
  const path_count = list.filter((p) => p.includes('/api/')).length
  let tone: OverviewPathsScan['tone'] = 'unknown'
  if (path_count >= 5) tone = 'ok'
  else if (path_count > 0) tone = 'warn'
  return {
    doctor_path,
    contract_path,
    health_path,
    memory_path,
    compaction_path,
    maintenance_path,
    version_path,
    path_count,
    tone,
  }
}

export function buildOverviewScanSummary(input: {
  doctor?: Parameters<typeof buildOverviewDoctorScan>[0]
  contract?: Parameters<typeof buildOverviewContractScan>[0]
  paths?: Parameters<typeof buildOverviewPathsScan>[0]
}): OverviewScanSummary {
  const doctor = buildOverviewDoctorScan(input.doctor || {})
  const contract = buildOverviewContractScan(input.contract || {})
  const paths = buildOverviewPathsScan(input.paths || {})
  const tones = [doctor.tone, contract.tone, paths.tone]
  let tone: OverviewScanSummary['tone'] = 'ok'
  if (tones.includes('bad')) tone = 'bad'
  else if (tones.includes('warn')) tone = 'warn'
  else if (tones.every((t) => t === 'unknown')) tone = 'unknown'
  return { doctor, contract, paths, tone }
}

function finiteNonNeg(v: unknown): number {
  if (!Number.isFinite(Number(v))) return 0
  return Math.max(0, Math.trunc(Number(v)))
}

function strPath(v: unknown, fallback: string): string {
  const s = String(v || '').trim()
  return s || fallback
}
