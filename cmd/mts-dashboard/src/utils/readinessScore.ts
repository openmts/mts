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
  /**
   * 最近一次管理重操作失败（ops-status last.ok=false）。
   * 计入总分小幅扣减，并写入 reasons；非 busy 时仍提示运维确认。
   */
  adminOpLastFailed?: boolean | null
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
  if (input.adminOpLastFailed) reasons.push('admin_op_last_failed')
  // 最近失败 last：总分扣 5（最低 0），不单独毁掉其它维度均分结构
  let adjustedTotal = total
  if (input.adminOpLastFailed) {
    adjustedTotal = Math.max(0, total - 5)
  }
  return {
    checklist: Math.round(checklist * 100),
    edgeHttps: Math.round(edgeHttps * 100),
    backupSchedule: Math.round(backupSchedule * 100),
    doctor: Math.round(doctor.score * 100),
    total: adjustedTotal,
    reasons,
  }
}

export function readinessLevel(total: number): 'good' | 'warn' | 'bad' {
  if (total >= 80) return 'good'
  if (total >= 50) return 'warn'
  return 'bad'
}

/** 评分原因 code → 展示文案（纯函数） */
export function formatReadinessReason(
  code: string,
  locale: 'zh' | 'en' = 'zh',
): string {
  const c = String(code || '').trim()
  if (!c) return ''
  if (c.startsWith('doctor_warns:')) {
    const n = c.slice('doctor_warns:'.length)
    return locale === 'en' ? `Doctor warnings: ${n}` : `Doctor 警告 ${n} 条`
  }
  const zh: Record<string, string> = {
    doctor_unavailable: 'Doctor 未加载',
    doctor_not_ok: 'Doctor 未通过',
    http_tls_disabled: 'HTTP TLS 未启用',
    checklist_incomplete: '生产清单未完成',
    edge_https_incomplete: '边缘 HTTPS 验收未完成',
    backup_schedule_incomplete: '备份编排清单未完成',
    admin_op_busy: '管理重操作占用中',
    admin_op_last_failed: '最近管理重操作失败',
  }
  const en: Record<string, string> = {
    doctor_unavailable: 'Doctor not loaded',
    doctor_not_ok: 'Doctor not OK',
    http_tls_disabled: 'HTTP TLS disabled',
    checklist_incomplete: 'Production checklist incomplete',
    edge_https_incomplete: 'Edge HTTPS acceptance incomplete',
    backup_schedule_incomplete: 'Backup schedule checklist incomplete',
    admin_op_busy: 'Admin heavy op busy',
    admin_op_last_failed: 'Last admin heavy op failed',
  }
  const map = locale === 'en' ? en : zh
  return map[c] || c
}

export function formatReadinessReasons(
  reasons: string[] | null | undefined,
  locale: 'zh' | 'en' = 'zh',
): string {
  const list = (reasons || []).map((r) => formatReadinessReason(r, locale)).filter(Boolean)
  return list.join(locale === 'en' ? '; ' : '；')
}
