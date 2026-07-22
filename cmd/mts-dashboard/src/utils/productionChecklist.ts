/** 可商用上线检查清单（纯数据，便于单测与文档同步） */

import type { LocalizedText } from './localizedText.ts'

export type ChecklistSeverity = 'required' | 'recommended'

export interface ProductionCheckItem {
  id: string
  severity: ChecklistSeverity
  title: LocalizedText
  detail: LocalizedText
  /** 自动化是否已覆盖 */
  automated: boolean
  /**
   * 可选跳转：
   * - 以 / 开头：路由
   * - 以 # 开头：同页锚点
   */
  jump?: string
}

export const PRODUCTION_CHECKLIST: ProductionCheckItem[] = [
  {
    id: 'https-edge',
    severity: 'required',
    title: { zh: '边缘 HTTPS / TLS', en: 'Edge HTTPS / TLS' },
    detail: {
      zh: '边缘终止 TLS/HSTS，或启用 mts-server HTTP TLS（启用后自动发 HSTS）；见 edgeHttpsAcceptance 清单与 doctor API。',
      en: 'Terminate TLS/HSTS at the edge, or enable mts-server HTTP TLS (HSTS is emitted automatically); see edgeHttpsAcceptance and doctor API.',
    },
    automated: true,
    jump: '#edge-https-checklist',
  },
  {
    id: 'security-headers',
    severity: 'required',
    title: { zh: '安全响应头', en: 'Security response headers' },
    detail: {
      zh: 'nosniff / DENY / CSP / Referrer-Policy 等由 wrapHTTP 默认写入。',
      en: 'nosniff / DENY / CSP / Referrer-Policy are written by wrapHTTP by default.',
    },
    automated: true,
    jump: '#doctor-panel',
  },
  {
    id: 'change-default-admin',
    severity: 'required',
    title: { zh: '修改默认 admin 密码', en: 'Change default admin password' },
    detail: {
      zh: 'bootstrap 默认密码登录后 must_change_password 拦截业务 API，直至改密完成。',
      en: 'After bootstrap default-password login, must_change_password blocks business APIs until password change completes.',
    },
    automated: true,
    jump: '/account#account-password',
  },
  {
    id: 'health-ready-metrics',
    severity: 'required',
    title: { zh: '健康与指标接入', en: 'Health and metrics integration' },
    detail: {
      zh: '/healthz /readyz /metrics 接入监控与告警；Dashboard /observability/metrics 可只读浏览。',
      en: 'Wire /healthz /readyz /metrics into monitoring/alerts; Dashboard /observability/metrics provides read-only browse.',
    },
    automated: true,
    jump: '/observability/metrics',
  },
  {
    id: 'backup-snapshot',
    severity: 'recommended',
    title: { zh: '备份与快照演练', en: 'Backup and snapshot drills' },
    detail: {
      zh: 'Storage 演练清单 + data-snapshot/restore-drill API + TestDataDirSidePathRestoreDrill；异地拷贝仍人工。',
      en: 'Storage drill list + data-snapshot/restore-drill API + TestDataDirSidePathRestoreDrill; off-host copy remains manual.',
    },
    automated: true,
    jump: '/storage',
  },
  {
    id: 'smoke-login-query-write',
    severity: 'required',
    title: {
      zh: '核心冒烟：登录-查询-写入-运维',
      en: 'Core smoke: login-query-write-ops',
    },
    detail: {
      zh: 'TestCommercialDashboardSmoke + Playwright commercial-smoke 覆盖登录/改密/写/查/运维。',
      en: 'TestCommercialDashboardSmoke + Playwright commercial-smoke cover login/password change/write/query/ops.',
    },
    automated: true,
    jump: '/query',
  },
  {
    id: 'admin-op-visibility',
    severity: 'recommended',
    title: { zh: '管理重操作 busy/last 可见', en: 'Admin heavy op busy/last visibility' },
    detail: {
      zh: '运维/管理页与全局横幅展示 admin_op_busy 与 last；勾选前请在运维页确认 busy 条与失败 last；fail-last 冒烟覆盖主要管理页芯片。',
      en: 'Ops/admin pages and global banner surface admin_op_busy and last; before checking, confirm busy strip and fail-last on Operations; fail-last smoke covers main admin chips.',
    },
    automated: true,
    jump: '/operations#ops-status-strip',
  },
  {
    id: 'data-restore-ui',
    severity: 'recommended',
    title: { zh: 'data_dir 旁路恢复编排', en: 'data_dir side-path restore orchestration' },
    detail: {
      zh: 'Storage 页 data-snapshot + restore-drill；目标仅限 backups 下旁路目录。',
      en: 'Storage page data-snapshot + restore-drill; targets limited to side-path dirs under backups.',
    },
    automated: true,
    jump: '/storage',
  },
  {
    id: 'readiness-center',
    severity: 'recommended',
    title: { zh: '可商用就绪中心', en: 'Commercial readiness center' },
    detail: {
      zh: 'Dashboard /ops/readiness 聚合清单、HTTPS 验收、备份编排与 doctor；评分含 doctor warn/TLS。',
      en: 'Dashboard /ops/readiness aggregates checklist, HTTPS acceptance, backup schedule and doctor; score includes doctor warn/TLS.',
    },
    automated: true,
    jump: '#readiness-action',
  },
  {
    id: 'admin-doctor',
    severity: 'recommended',
    title: { zh: '部署 Doctor API', en: 'Deploy Doctor API' },
    detail: {
      zh: 'GET /api/v1/admin/doctor + Overview 展示；CLI mts-server doctor 同口径。',
      en: 'GET /api/v1/admin/doctor + Overview display; CLI mts-server doctor uses the same contract.',
    },
    automated: true,
    jump: '#doctor-panel',
  },
  {
    id: 'backup-script',
    severity: 'recommended',
    title: { zh: '备份编排脚本', en: 'Backup orchestration script' },
    detail: {
      zh: 'scripts/mts-backup.sh 支持 data-snapshot / rsync / restore-drill；make backup-script-check 自检。',
      en: 'scripts/mts-backup.sh supports data-snapshot / rsync / restore-drill; make backup-script-check self-checks.',
    },
    automated: true,
    jump: '#backup-schedule-checklist',
  },
  {
    id: 'backup-schedule',
    severity: 'recommended',
    title: { zh: '跨主机定时备份编排', en: 'Cross-host scheduled backup orchestration' },
    detail: {
      zh: 'scripts/mts-backup.sh + 就绪中心指引 + cron/systemd 样例；实际调度在部署侧。',
      en: 'scripts/mts-backup.sh + readiness guidance + cron/systemd samples; real scheduling is deployment-side.',
    },
    automated: true,
    jump: '#backup-schedule-checklist',
  },
  {
    id: 'production-runbook',
    severity: 'recommended',
    title: { zh: '生产 Runbook', en: 'Production runbook' },
    detail: {
      zh: 'docs/ops/dashboard-production-runbook.md 覆盖拓扑、清单、代理与应急。',
      en: 'docs/ops/dashboard-production-runbook.md covers topology, checklist, proxy and incident response.',
    },
    automated: false,
    jump: '#deploy-runbook-drill',
  },
  {
    id: 'rbac-matrix-ui',
    severity: 'recommended',
    title: { zh: '权限矩阵可视化', en: 'RBAC matrix visualization' },
    detail: {
      zh: 'Dashboard /access 能力对照 + /access/grants 实时 grants 汇总。',
      en: 'Dashboard /access capability map + /access/grants live grant summary.',
    },
    automated: true,
    jump: '/access',
  },
  {
    id: 'rbac-review',
    severity: 'recommended',
    title: { zh: '权限矩阵复核', en: 'RBAC matrix review' },
    detail: {
      zh: '确认非 admin 仅可访问授权库的读写能力。',
      en: 'Confirm non-admin users can only read/write granted databases.',
    },
    automated: false,
    jump: '/access/grants',
  },
  {
    id: 'user-disable-revokes-tokens',
    severity: 'recommended',
    title: { zh: '禁用用户撤销会话', en: 'Disable user revokes sessions' },
    detail: {
      zh: '禁用用户（单条/批量）会主动撤销其全部 bearer token；登录失败仍返回 invalid credentials（不泄露账户状态）。可在 Users 状态筛选核对。',
      en: 'Disabling a user (single/batch) actively revokes all of their bearer tokens; failed logins still return invalid credentials (no account-state leak). Verify via Users status filter.',
    },
    automated: true,
    jump: '/users?status=disabled#users-filter-bar',
  },
  {
    id: 'batch-admin-last',
    severity: 'recommended',
    title: { zh: '批量管理写入 last 可见', en: 'Batch admin ops write visible last' },
    detail: {
      zh: '单条/批量用户启停与降采样批量启停应写入轻量 last（不占 heavy 互斥），Users/Downsample 页芯片与全局 last 横幅可核对。',
      en: 'Single/batch user enable/disable and downsample batch enable/disable write a light last (no heavy lock); verify via Users/Downsample chips and global last banner.',
    },
    automated: true,
    jump: '/users#users-filter-bar',
  },
  {
    id: 'downsample-advanced-form',
    severity: 'recommended',
    title: { zh: '降采样高级字段可配置', en: 'Downsample advanced fields configurable' },
    detail: {
      zh: '创建策略表单可配置 retention / refresh / lookback / batch_size；服务端缺省补齐；列表/导出可见关键字段。',
      en: 'Create form exposes retention/refresh/lookback/batch_size; server fills defaults; list/export include key fields.',
    },
    automated: true,
    jump: '/downsample#downsample-filter-bar',
  },
  {
    id: 'downsample-policy-detail',
    severity: 'recommended',
    title: { zh: '降采样策略详情可核对', en: 'Downsample policy detail inspectable' },
    detail: {
      zh: '策略列表可打开详情面板，核对 retention/refresh/lookback/batch_size/functions 与水位摘要。',
      en: 'Open a policy detail panel from the list to verify retention/refresh/lookback/batch_size/functions and watermark summary.',
    },
    automated: true,
    jump: '/downsample#downsample-detail',
  },
  {
    id: 'downsample-policy-deep-link',
    severity: 'recommended',
    title: { zh: '降采样策略详情深链可复用', en: 'Downsample policy detail deep link reusable' },
    detail: {
      zh: '详情支持 ?policy= 与复制 JSON/链接；深链只读打开详情，不自动 run/enable。',
      en: 'Detail supports ?policy= plus copy JSON/link; deep links open read-only detail without auto run/enable.',
    },
    automated: true,
    jump: '/downsample?policy=example#downsample-detail',
  },
  {
    id: 'downsample-status-health',
    severity: 'recommended',
    title: { zh: '降采样状态健康可扫视', en: 'Downsample status health scannable' },
    detail: {
      zh: '状态表支持 error/active/lagging 筛选，服务端 statuses 返回 summary；Overview/Readiness/Metrics 展示健康摘要并可一键深链。',
      en: 'Status table supports error/active/lagging filters; server statuses include summary; Overview/Readiness/Metrics show health with deep-links.',
    },
    automated: true,
    jump: '/downsample?health=error#downsample-status',
  },
  {
    id: 'readiness-downsample-health-card',
    severity: 'recommended',
    title: { zh: '就绪中心降采样健康卡片', en: 'Readiness downsample health card' },
    detail: {
      zh: '就绪页展示 statuses summary 与 error/lagging 深链；归档/验收包可带摘要；只读不自动 run。',
      en: 'Readiness page shows statuses summary and error/lagging deep-links; archive/pack may include summary; read-only, no auto run.',
    },
    automated: true,
    jump: '/ops/readiness#downsample-health-panel',
  },
  {
    id: 'ops-downsample-health-card',
    severity: 'recommended',
    title: { zh: '运维页降采样健康条', en: 'Operations downsample health strip' },
    detail: {
      zh: 'Operations 展示 statuses summary 与 error/lagging 深链；全局 admin 横幅在 error/lagging>0 时提示。',
      en: 'Operations shows statuses summary and error/lagging deep-links; global admin banner when error/lagging > 0.',
    },
    automated: true,
    jump: '/operations',
  },
  {
    id: 'password-policy-public',
    severity: 'required',
    title: { zh: '公开密码策略可对齐', en: 'Public password policy alignment' },
    detail: {
      zh: 'GET /api/v1/auth/password-policy（v2）提供 min_length/禁用默认密码/鉴权 TTL；Dashboard 启动与登录/改密页拉取对齐。',
      en: 'GET /api/v1/auth/password-policy (v2) exposes min_length/forbidden defaults/auth TTL; Dashboard bootstraps on app start, login and password pages.',
    },
    automated: true,
    jump: '/account#account-password-policy',
  },
  {
    id: 'session-remaining-calibration',
    severity: 'required',
    title: { zh: '会话 remaining 服务端校准', en: 'Session remaining server calibration' },
    detail: {
      zh: '写门禁/TopBar/Account/Overview 取 min(本地 expires, 服务端 remaining 推演)；warn/critical 周期探测 /auth/session。',
      en: 'Write guard/TopBar/Account/Overview use min(local expires, projected server remaining); warn/critical probe /auth/session on a cadence.',
    },
    automated: true,
    jump: '/account#account-session',
  },
  {
    id: 'login-session-seed',
    severity: 'required',
    title: { zh: '登录即会话校准种子', en: 'Login seeds session calibration' },
    detail: {
      zh: 'POST /auth/login 返回 remaining_seconds 与 server_time_unix；Dashboard 登录成功后立即种子化，无需等待周期探测。',
      en: 'POST /auth/login returns remaining_seconds and server_time_unix; Dashboard seeds calibration immediately after login without waiting for probes.',
    },
    automated: true,
    jump: '/account#account-session',
  },
  {
    id: 'session-sample-source',
    severity: 'recommended',
    title: { zh: '会话样本来源可区分', en: 'Session sample source distinguishable' },
    detail: {
      zh: 'Account/交接摘要区分 login 种子与 GET /auth/session 探测；API Spec 列出 login/session 的 remaining_seconds 与 server_time_unix。',
      en: 'Account/handoff distinguish login seed vs GET /auth/session probe; API Spec lists remaining_seconds and server_time_unix on login/session.',
    },
    automated: true,
    jump: '/account#account-session',
  },
  {
    id: 'overview-session-server-hint',
    severity: 'recommended',
    title: { zh: 'Overview 会话服务端提示', en: 'Overview session server hint' },
    detail: {
      zh: 'Overview 会话徽章展示校准后 remaining；有服务端样本时始终显示服务端 remaining/时钟偏差（运维扫视）。',
      en: 'Overview session badge shows calibrated remaining; with a server sample always shows server remaining/clock skew (ops sweep).',
    },
    automated: true,
    jump: '/#overview-summary',
  },
  {
    id: 'clock-skew-banner',
    severity: 'recommended',
    title: { zh: '大时钟偏差全局提示', en: 'Large clock skew global banner' },
    detail: {
      zh: '有 server_time_unix 样本且 |skew|>=30s 时 Layout 横幅 + 导出预检 warn + Account 告警；下一步/预检可跳转账户会话。',
      en: 'When server_time_unix is sampled and |skew|>=30s: Layout banner, export preflight warn, Account alert; next-steps/preflight jump to account session.',
    },
    automated: true,
    jump: '/account#account-session',
  },
  {
    id: 'api-spec-password-policy',
    severity: 'recommended',
    title: { zh: '契约可检索 password-policy', en: 'Api-spec searchable password-policy' },
    detail: {
      zh: 'API Spec 含 auth/password-policy 与 TTL 字段说明；命令面板/深链可跳转筛选。',
      en: 'API Spec includes auth/password-policy and TTL fields; command palette/deep-link can filter.',
    },
    automated: true,
    jump: '/api-spec?ns=auth&q=password-policy#api-spec-filters',
  },
  {
    id: 'api-spec-auth-session-seed',
    severity: 'recommended',
    title: { zh: '契约可检索 login/session 校准字段', en: 'Api-spec searchable login/session calibration fields' },
    detail: {
      zh: 'API Spec 可检索 remaining_seconds / server_time_unix（login 与 session）；命令面板深链筛选。',
      en: 'API Spec is searchable for remaining_seconds / server_time_unix (login and session); command palette deep-link filters.',
    },
    automated: true,
    jump: '/api-spec?ns=auth&q=remaining_seconds#api-spec-filters',
  },
  {
    id: 'error-codes-remediation',
    severity: 'required',
    title: { zh: '错误码契约含可操作处置', en: 'Error-code contract is actionable' },
    detail: {
      zh: 'GET /admin/error-codes 返回 retryable/category/remediation/dashboard_path；错误响应附带 remediation；Config 表与 Query/Write 错误横幅可深链对照。',
      en: 'GET /admin/error-codes returns retryable/category/remediation/dashboard_path; error responses include remediation; Config table and Query/Write banners deep-link to the contract.',
    },
    automated: true,
    jump: '/config?error_q=resource_exhausted#config-error-codes',
  },
  {
    id: 'write-accepted-points',
    severity: 'required',
    title: { zh: '写入响应返回 accepted points', en: 'Write response reports accepted points' },
    detail: {
      zh: 'POST /data/write|write/typed|write/points-typed 成功响应含 points/path；Dashboard 成功文案优先使用服务端计数与路径。',
      en: 'Successful POST /data/write|write/typed|write/points-typed responses include points/path; Dashboard success copy prefers server-accepted count and path.',
    },
    automated: true,
    jump: '/write#write-mode-tabs',
  },
  {
    id: 'query-result-meta',
    severity: 'required',
    title: { zh: '查询响应含 path/count/admin_op', en: 'Query responses include path/count/admin_op' },
    detail: {
      zh: 'POST query/rows|columns|explain 返回 path/count/database/measurement 与 admin_op_busy/last；Dashboard 查询台消费并刷新全局管理占用状态。',
      en: 'POST query/rows|columns|explain return path/count/database/measurement plus admin_op_busy/last; Query workbench consumes them and refreshes global admin-op state.',
    },
    automated: true,
    jump: '/query#query-form',
  },
  {
    id: 'query-result-path-visible',
    severity: 'recommended',
    title: { zh: '查询结果展示服务端 path', en: 'Query results show server path' },
    detail: {
      zh: '查询成功后 rows/columns/raw 区展示服务端 path 徽章；计数优先 row_count/series_count；命令面板可深链 ApiSpec 检索 row_count/series_count/points。',
      en: 'After a successful query, rows/columns/raw show server path badges; counts prefer row_count/series_count; command palette deep-links ApiSpec for row_count/series_count/points.',
    },
    automated: true,
    jump: '/query#query-results',
  },
  {
    id: 'query-result-scope',
    severity: 'recommended',
    title: { zh: '查询响应含 database/measurement', en: 'Query responses include database/measurement' },
    detail: {
      zh: 'POST query/rows|columns|explain 与 stream end 返回 database/measurement；Query 结果区徽章展示。',
      en: 'POST query/rows|columns|explain and stream end return database/measurement; Query result badges surface them.',
    },
    automated: true,
    jump: '/query#query-results',
  },
  {
    id: 'query-result-export-meta',
    severity: 'recommended',
    title: { zh: '查询结果导出含服务端 meta', en: 'Query result export includes server meta' },
    detail: {
      zh: '查询结果 JSON 导出 v1 含 path/database/measurement/row_count/series_count 与 query 快照；分享链接可带 mode。',
      en: 'Query result JSON export v1 includes path/database/measurement/row_count/series_count and query snapshot; share links can carry mode.',
    },
    automated: true,
    jump: '/query#query-results',
  },
  {
    id: 'query-stats-path',
    severity: 'recommended',
    title: { zh: '查询 Stats 展示服务端 path', en: 'Query stats show server path' },
    detail: {
      zh: 'GET /api/v1/data/query/stats 返回 path；引擎快照成功后 Stats 区展示 path 徽章；数据面契约含 query_stats_path。',
      en: 'GET /api/v1/data/query/stats returns path; after engine snapshot, stats panel shows path badge; data contract includes query_stats_path.',
    },
    automated: true,
    jump: '/query#query-stats',
  },
  {
    id: 'delete-result-export-meta',
    severity: 'recommended',
    title: { zh: '删除结果导出含 path/scope', en: 'Delete result export includes path/scope' },
    detail: {
      zh: '范围删除成功后可导出 mts.delete.result v1（path/database/measurement/时间窗）。',
      en: 'After range delete, export mts.delete.result v1 with path/database/measurement/time range.',
    },
    automated: true,
    jump: '/query#query-stats',
  },
  {
    id: 'meta-list-path',
    severity: 'recommended',
    title: { zh: '元数据列表含 path/scope', en: 'Meta list responses include path/scope' },
    detail: {
      zh: 'databases/measurements/fields/series 列表响应含 path 与 database/measurement；Query series 区展示 path 徽章；契约 feature meta_list_path。',
      en: 'databases/measurements/fields/series list responses include path and database/measurement; Query series shows path badge; contract feature meta_list_path.',
    },
    automated: true,
    jump: '/query',
  },
  {
    id: 'databases-meta-path',
    severity: 'recommended',
    title: { zh: '库表页展示元数据 path', en: 'Databases page shows meta path' },
    detail: {
      zh: 'Databases 页 series/RP 列表展示服务端 path；RP 客户端解析 path/database。',
      en: 'Databases page surfaces series/RP server path; RP client parses path/database.',
    },
    automated: true,
    jump: '/databases',
  },
  {
    id: 'data-contract-endpoint',
    severity: 'required',
    title: { zh: '数据面契约快照可交接', en: 'Data-plane contract snapshot for handoff' },
    detail: {
      zh: 'GET /api/v1/data/contract 返回 limits + write/query/stream/delete meta 能力；就绪交接/导出预检/验收包纳入 data_contract 摘要。',
      en: 'GET /api/v1/data/contract returns limits plus write/query/stream/delete meta capabilities; readiness handoff/export preflight/acceptance pack include data_contract summary.',
    },
    automated: true,
    jump: '/ops/readiness#commercial-handoff-panel',
  },
  {
    id: 'overview-data-contract',
    severity: 'recommended',
    title: { zh: '概览健康报告含数据面契约', en: 'Overview health report includes data contract' },
    detail: {
      zh: 'Overview 加载 GET /data/contract；健康报告/交接复制与导出预检纳入 data_contract；摘要行可见。',
      en: 'Overview loads GET /data/contract; health report/handoff copy and export preflight include data_contract; summary line is visible.',
    },
    automated: true,
    jump: '/#overview-summary',
  },
  {
    id: 'overview-data-contract-jump',
    severity: 'recommended',
    title: { zh: '概览契约可跳转就绪交接', en: 'Overview contract jumps to readiness handoff' },
    detail: {
      zh: 'Overview 契约芯片支持刷新与跳转就绪交接；导出预检 data-contract 缺口可一键打开 commercial-handoff。',
      en: 'Overview contract chip supports refresh and jump to readiness handoff; export preflight data-contract gaps deep-link to commercial-handoff.',
    },
    automated: true,
    jump: '/#overview-summary',
  },
  {
    id: 'acceptance-data-contract',
    severity: 'required',
    title: { zh: '验收包含顶层 data_contract', en: 'Acceptance pack includes top-level data_contract' },
    detail: {
      zh: '验收包 v2 顶层含 data_contract 摘要（loaded/complete/path/summary_line）；Markdown 独立章节；导出预检纳入契约状态。',
      en: 'Acceptance pack v2 includes top-level data_contract summary (loaded/complete/path/summary_line); Markdown has a dedicated section; export preflight includes contract status.',
    },
    automated: true,
    jump: '/ops/readiness#commercial-handoff-panel',
  },
  {
    id: 'write-empty-aligned',
    severity: 'recommended',
    title: { zh: '写入空态与查询 idle 对齐', en: 'Write empty state aligns with query idle' },
    detail: {
      zh: 'Write 结果区空态提供执行写入 / 回到表单 / TypedBatch CTA；成功后展示服务端 path 与 mode。',
      en: 'Write result empty state offers submit / back-to-form / TypedBatch CTAs; success surfaces server path and mode.',
    },
    automated: true,
    jump: '/write',
  },
  {
    id: 'write-response-mode',
    severity: 'recommended',
    title: { zh: '写入响应含 mode/database', en: 'Write response includes mode/database' },
    detail: {
      zh: 'POST write|typed|points-typed 返回 mode/database/path/points；Dashboard 结果区与结果导出纳入。',
      en: 'POST write|typed|points-typed returns mode/database/path/points; Write page and result export consume them.',
    },
    automated: true,
    jump: '/write',
  },
  {
    id: 'write-response-retention',
    severity: 'recommended',
    title: { zh: '写入响应含 retention_policy', en: 'Write responses include retention_policy' },
    detail: {
      zh: 'write 响应在单 RP 批次返回 retention_policy；Write 结果区徽章与导出优先服务端值；契约 feature write_response_retention。',
      en: 'Write responses include retention_policy for single-policy batches; Write result badges/export prefer server value; contract feature write_response_retention.',
    },
    automated: true,
    jump: '/write#write-result-ok-wrap',
  },
  {
    id: 'databases-meas-path',
    severity: 'recommended',
    title: { zh: '库表页展示 measurements path', en: 'Databases page shows measurements path' },
    detail: {
      zh: '展开 database 后 measurements 列表展示服务端 path 徽章。',
      en: 'Expanded database shows server path badge for measurements list.',
    },
    automated: true,
    jump: '/databases',
  },
  {
    id: 'ops-maintenance-path',
    severity: 'recommended',
    title: { zh: '运维维护响应含 path', en: 'Ops maintenance responses include path' },
    detail: {
      zh: 'flush/compact/retention/apply 响应含 path；运维页成功文案展示服务端 path。',
      en: 'flush/compact/retention/apply responses include path; Operations success messages surface server path.',
    },
    automated: true,
    jump: '/ops#ops-status-strip',
  },
  {
    id: 'session-policy-audit-path',
    severity: 'recommended',
    title: { zh: '会话/策略/审计响应含 path', en: 'Session/policy/audit responses include path' },
    detail: {
      zh: 'login/session/password-policy/authz check、admin/user audit 返回 path；Account/Audit/Access 徽章展示。',
      en: 'login/session/password-policy/authz check, admin/user audit return path; Account/Audit/Access surface badges.',
    },
    automated: true,
    jump: '/account#account-password-policy',
  },
  {
    id: 'ops-config-stats-path',
    severity: 'recommended',
    title: { zh: '运维统计/配置/契约响应含 path', en: 'Ops stats/config/spec responses include path' },
    detail: {
      zh: 'maintenance/ops-status/memory/compaction/errors、config/schema/error-codes、api-spec 返回 path；Operations/Config/ApiSpec 徽章展示。',
      en: 'maintenance/ops-status/memory/compaction/errors, config/schema/error-codes, api-spec return path; Operations/Config/ApiSpec surface badges.',
    },
    automated: true,
    jump: '/operations',
  },

  {
    id: 'storage-auth-path',
    severity: 'recommended',
    title: { zh: '存储快照/导出与 logout 响应含 path', en: 'Storage snapshot/export and logout responses include path' },
    detail: {
      zh: 'storage snapshots list/delete、data-snapshots、export、auth/logout、change-password 返回 path；Storage 删除成功文案展示。',
      en: 'storage snapshots list/delete, data-snapshots, export, auth/logout, change-password return path; Storage delete success surfaces it.',
    },
    automated: true,
    jump: '/storage',
  },
  {
    id: 'storage-overview-path',
    severity: 'recommended',
    title: { zh: '存储演练/Overview/降采样 run 响应含 path', en: 'Storage drill/Overview/downsample run responses include path' },
    detail: {
      zh: 'restore-drill/export/snapshots 列表、Overview stats/health、downsample policies/statuses/run/dry-run 返回 path；Storage/Overview/Downsample 展示。',
      en: 'restore-drill/export/snapshots list, Overview stats/health, downsample policies/statuses/run/dry-run return path; Storage/Overview/Downsample surface it.',
    },
    automated: true,
    jump: '/storage',
  },

  {
    id: 'meta-downsample-path',
    severity: 'recommended',
    title: { zh: '库表/RP/降采样写路径响应含 path', en: 'Database/RP/downsample mutation responses include path' },
    detail: {
      zh: 'create/drop database、create RP、downsample create/delete/enable/disable/reset/list/statuses 返回 path；Databases/Downsample 成功文案展示。',
      en: 'create/drop database, create RP, downsample create/delete/enable/disable/reset/list/statuses return path; Databases/Downsample success messages surface it.',
    },
    automated: true,
    jump: '/databases',
  },
  {
    id: 'readiness-storage-result-path',
    severity: 'recommended',
    title: { zh: '就绪归档 API path + 存储演练结构化结果', en: 'Readiness archive API paths + structured storage drill result' },
    detail: {
      zh: 'Readiness 归档含 doctor/api_paths；Storage data-snapshot/restore-drill 以结构化卡片展示 path/files/fatals。',
      en: 'Readiness archive includes doctor/api_paths; Storage data-snapshot/restore-drill show structured path/files/fatals cards.',
    },
    automated: true,
    jump: '/storage#data-restore',
  },

  {
    id: 'users-doctor-path',
    severity: 'recommended',
    title: { zh: 'Users/批次与 doctor/health 响应含 path', en: 'Users/batch and doctor/health responses include path' },
    detail: {
      zh: 'users 列表/创建/更新/删除/密码/授权/batch-disabled、doctor、admin/health 返回 path；Users/Overview/Readiness 成功文案或徽章展示。',
      en: 'users list/create/update/delete/password/grants/batch-disabled, doctor, admin/health return path; Users/Overview/Readiness surface it.',
    },
    automated: true,
    jump: '/users',
  },
  {
    id: 'storage-validate-metrics-path',
    severity: 'recommended',
    title: { zh: '存储验证结构化 + Metrics 源 path', en: 'Structured storage validate + Metrics source paths' },
    detail: {
      zh: 'Storage validate/snapshot 结构化结果卡；Metrics 展示 /metrics、ops-status、downsample statuses path。',
      en: 'Storage validate/snapshot structured result cards; Metrics shows /metrics, ops-status, downsample statuses paths.',
    },
    automated: true,
    jump: '/storage',
  },

  {
    id: 'storage-export-write-path',
    severity: 'recommended',
    title: { zh: '存储导出摘要 + 写入 API path', en: 'Storage export summary + write API path' },
    detail: {
      zh: 'Storage export 结构化摘要（users/grants/path）；Write 页展示当前写入 API path（typed/points-typed/write）。',
      en: 'Storage export structured summary (users/grants/path); Write page shows active write API path (typed/points-typed/write).',
    },
    automated: true,
    jump: '/storage',
  },

  {
    id: 'config-effective-summary',
    severity: 'recommended',
    title: { zh: '有效配置结构化摘要', en: 'Effective config structured summary' },
    detail: {
      zh: 'Config 有效配置展示顶层分区/叶子数/敏感键命中摘要，原始 JSON 可折叠。',
      en: 'Config effective view shows section/leaf/sensitive-key summary with collapsible raw JSON.',
    },
    automated: true,
    jump: '/config',
  },

  {
    id: 'admin-config-storage-path',
    severity: 'recommended',
    title: { zh: '配置/存储/版本响应含 path', en: 'Config/storage/version responses include path' },
    detail: {
      zh: 'config validate/reload、storage validate、admin/version 返回 path；Config/Storage/About 成功态或徽章展示。',
      en: 'config validate/reload, storage validate, admin/version return path; Config/Storage/About surface it.',
    },
    automated: true,
    jump: '/config',
  },
  {
    id: 'write-result-export-meta',
    severity: 'recommended',
    title: { zh: '写入结果导出含服务端 meta', en: 'Write result export includes server meta' },
    detail: {
      zh: '结果导出 v2 含 path/server_mode/database/points，与 writeResponse 对齐。',
      en: 'Result export v2 includes path/server_mode/database/points aligned with writeResponse.',
    },
    automated: true,
    jump: '/write',
  },
  {
    id: 'stream-delete-meta',
    severity: 'required',
    title: { zh: '流式查询 end 与删除响应含 meta', en: 'Stream end and delete responses include meta' },
    detail: {
      zh: 'POST /data/query/stream 的 end 帧含 path/format/record_count/database/measurement/admin_op；POST /data/delete 返回 path/database/measurement/admin_op；Query 页消费并展示。',
      en: 'POST /data/query/stream end frames include path/format/record_count/database/measurement/admin_op; POST /data/delete returns path/database/measurement/admin_op; Query page consumes and surfaces them.',
    },
    automated: true,
    jump: '/query#query-results',
  },
  {
    id: 'data-limits-endpoint',
    severity: 'required',
    title: { zh: '数据面 limits 可对齐', en: 'Data-plane limits are exposed' },
    detail: {
      zh: 'GET /api/v1/data/limits 返回 max_write_points/default_query_limit/max_query_limit；Write/Query 页展示并做超限提示/裁剪；Config schema 含 limits.* 字段。',
      en: 'GET /api/v1/data/limits returns max_write_points/default_query_limit/max_query_limit; Write/Query pages surface caps and preflight/clamp; Config schema lists limits.* fields.',
    },
    automated: true,
    jump: '/write',
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

/** 解析清单项跳转：路由或同页锚点 */
export function productionChecklistJump(item: Pick<ProductionCheckItem, 'jump'>): string | null {
  const j = String(item.jump || '').trim()
  return j || null
}
