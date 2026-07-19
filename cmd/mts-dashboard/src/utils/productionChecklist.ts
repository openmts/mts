/** 可商用上线检查清单（纯数据，便于单测与文档同步） */

export type ChecklistSeverity = 'required' | 'recommended'

export interface ProductionCheckItem {
  id: string
  severity: ChecklistSeverity
  title: string
  detail: string
  /** 自动化是否已覆盖 */
  automated: boolean
}

export const PRODUCTION_CHECKLIST: ProductionCheckItem[] = [
  {
    id: 'https-edge',
    severity: 'required',
    title: '边缘 HTTPS / TLS',
    detail: '由反向代理或负载均衡终止 TLS；mts-server 也可启用 HTTP TLS。',
    automated: false,
  },
  {
    id: 'security-headers',
    severity: 'required',
    title: '安全响应头',
    detail: 'nosniff / DENY / CSP / Referrer-Policy 等由 wrapHTTP 默认写入。',
    automated: true,
  },
  {
    id: 'change-default-admin',
    severity: 'required',
    title: '修改默认 admin 密码',
    detail: 'bootstrap 默认密码登录后 must_change_password 拦截业务 API，直至改密完成。',
    automated: true,
  },
  {
    id: 'health-ready-metrics',
    severity: 'required',
    title: '健康与指标接入',
    detail: '/healthz /readyz /metrics 接入监控与告警；Dashboard /metrics 可只读浏览。',
    automated: true,
  },
  {
    id: 'backup-snapshot',
    severity: 'recommended',
    title: '备份与快照演练',
    detail: '验证 storage snapshot/export 与恢复流程。',
    automated: false,
  },
  {
    id: 'smoke-login-query-write',
    severity: 'required',
    title: '核心冒烟：登录-查询-写入-运维',
    detail: 'TestCommercialDashboardSmoke 覆盖服务侧闭环；浏览器 UI 仍建议人工/Playwright。',
    automated: true,
  },
  {
    id: 'production-runbook',
    severity: 'recommended',
    title: '生产 Runbook',
    detail: 'docs/ops/dashboard-production-runbook.md 覆盖拓扑、清单、代理与应急。',
    automated: false,
  },
  {
    id: 'rbac-matrix-ui',
    severity: 'recommended',
    title: '权限矩阵可视化',
    detail: 'Dashboard /access 能力对照 + /access/grants 实时 grants 汇总。',
    automated: true,
  },
  {
    id: 'rbac-review',
    severity: 'recommended',
    title: '权限矩阵复核',
    detail: '确认非 admin 仅可访问授权库的读写能力。',
    automated: false,
  },
]

export function requiredChecklist(items = PRODUCTION_CHECKLIST): ProductionCheckItem[] {
  return items.filter((x) => x.severity === 'required')
}

export function automatedCoverage(items = PRODUCTION_CHECKLIST): {
  total: number
  automated: number
  ratio: number
} {
  const total = items.length
  const automated = items.filter((x) => x.automated).length
  return { total, automated, ratio: total === 0 ? 0 : automated / total }
}
