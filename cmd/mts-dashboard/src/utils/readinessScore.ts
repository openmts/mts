/** 可商用就绪评分（纯函数，便于单测） */

export interface ReadinessScoreInput {
  /** 必做生产清单完成比例 0..1 */
  requiredChecklistRatio: number
  /** 边缘 HTTPS 必做完成比例 0..1 */
  edgeHttpsRequiredRatio: number
  /** 备份编排必做完成比例 0..1 */
  backupScheduleRequiredRatio: number
  /** doctor 是否已成功加载 */
  doctorLoaded: boolean
  /** doctor.ok */
  doctorOk?: boolean
  /** doctor warn 条数 */
  doctorWarnCount?: number
  /** HTTP TLS 是否启用（doctor.http_tls_enabled） */
  httpTlsEnabled?: boolean | null
  /**
   * 服务端管理重操作占用（flush/compact/snapshot 等互斥）。
   * 不直接进四维均分，仅作为建议项原因提示；为 true 时在 reasons 标记。
   */
  adminOpBusy?: boolean | null
}

export interface ReadinessScoreBreakdown {
  checklist: number
  edgeHttps: number
  backupSchedule: number
  doctor: number
  total: number
  reasons: string[]
}

function clamp01(n: number): number {
  if (Number.isNaN(n) || !Number.isFinite(n)) return 0
  if (n < 0) return 0
  if (n > 1) return 1
  return n
}

/** doctor 维度：未加载=0.4；ok 且无 warn=1；每个 warn 扣 0.12，最低 0.35；无 TLS 再扣 0.15 */
export function doctorScorePart(input: Pick<
  ReadinessScoreInput,
  'doctorLoaded' | 'doctorOk' | 'doctorWarnCount' | 'httpTlsEnabled'
>): { score: number; reasons: string[] } {
  const reasons: string[] = []
  if (!input.doctorLoaded) {
    reasons.push('doctor_unavailable')
    return { score: 0.4, reasons }
  }
  let score = input.doctorOk === false ? 0.5 : 1
  if (input.doctorOk === false) reasons.push('doctor_not_ok')
  const warns = Math.max(0, input.doctorWarnCount ?? 0)
  if (warns > 0) {
    score -= Math.min(0.48, warns * 0.12)
    reasons.push(`doctor_warns:${warns}`)
  }
  if (input.httpTlsEnabled === false) {
    score -= 0.15
    reasons.push('http_tls_disabled')
  }
  if (score < 0.35) score = 0.35
  return { score: clamp01(score), reasons }
}

export function computeReadinessScore(input: ReadinessScoreInput): ReadinessScoreBreakdown {
  const checklist = clamp01(input.requiredChecklistRatio)
  const edgeHttps = clamp01(input.edgeHttpsRequiredRatio)
  const backupSchedule = clamp01(input.backupScheduleRequiredRatio)
  const doctor = doctorScorePart(input)
  const total = Math.round(((checklist + edgeHttps + backupSchedule + doctor.score) / 4) * 100)
  const reasons = [...doctor.reasons]
  if (checklist < 1) reasons.push('checklist_incomplete')
  if (edgeHttps < 1) reasons.push('edge_https_incomplete')
  if (backupSchedule < 1) reasons.push('backup_schedule_incomplete')
  if (input.adminOpBusy) reasons.push('admin_op_busy')
  return {
    checklist: Math.round(checklist * 100),
    edgeHttps: Math.round(edgeHttps * 100),
    backupSchedule: Math.round(backupSchedule * 100),
    doctor: Math.round(doctor.score * 100),
    total,
    reasons,
  }
}

export function readinessLevel(total: number): 'good' | 'warn' | 'bad' {
  if (total >= 80) return 'good'
  if (total >= 50) return 'warn'
  return 'bad'
}
