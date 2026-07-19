/** 边缘 HTTPS / HSTS 人工验收步骤（纯数据，便于单测与 Runbook 同步） */

export type EdgeHttpsStepSeverity = 'required' | 'recommended'

export interface EdgeHttpsAcceptanceStep {
  id: string
  severity: EdgeHttpsStepSeverity
  title: string
  detail: string
  /** 是否可由服务侧自动化部分覆盖 */
  partialAutomated: boolean
}

export const EDGE_HTTPS_ACCEPTANCE_STEPS: EdgeHttpsAcceptanceStep[] = [
  {
    id: 'tls-terminate',
    severity: 'required',
    title: '边缘 TLS 终止',
    detail: '反向代理/LB 配置有效证书并对外提供 HTTPS；或启用 mts-server HTTP TLS。',
    partialAutomated: true,
  },
  {
    id: 'hsts-header',
    severity: 'required',
    title: 'HSTS 响应头',
    detail: '确认全站 HTTPS 后设置 Strict-Transport-Security（本机 TLS 时 mts-server 自动写入）。',
    partialAutomated: true,
  },
  {
    id: 'http-redirect',
    severity: 'required',
    title: 'HTTP 跳转 HTTPS',
    detail: '明文 80/HTTP 请求应 301/308 到 HTTPS，避免混合内容与会话泄露。',
    partialAutomated: false,
  },
  {
    id: 'doctor-check',
    severity: 'recommended',
    title: 'doctor 检查',
    detail: '运行 mts-server doctor 或 GET /api/v1/admin/doctor，确认 TLS/鉴权提示已处理。',
    partialAutomated: true,
  },
  {
    id: 'smoke-https',
    severity: 'recommended',
    title: 'HTTPS 冒烟',
    detail: '浏览器经 HTTPS 完成登录 → 查询 → 运维动作，确认无证书告警与混合内容。',
    partialAutomated: false,
  },
]

export function edgeHttpsProgress(doneIds: string[], steps = EDGE_HTTPS_ACCEPTANCE_STEPS): {
  requiredTotal: number
  requiredDone: number
  total: number
  done: number
} {
  const done = new Set(doneIds)
  const required = steps.filter((s) => s.severity === 'required')
  const requiredDone = required.filter((s) => done.has(s.id)).length
  const doneCount = steps.filter((s) => done.has(s.id)).length
  return {
    requiredTotal: required.length,
    requiredDone,
    total: steps.length,
    done: doneCount,
  }
}
