# MTS Dashboard 可商用基线（进行中）

## 已具备
- 前后端 API 契约对齐（P0–P3）
- 查询工作台：历史/快捷键/虚拟滚动/列偏好/耗时水线
- 管理页统一结果条与空状态（P9–P10）
- 全局 loading、会话过期提示（P11）
- 安全响应头默认集 + 前端契约（P12）
- 服务侧可商用冒烟 + 生产清单纯数据（P13）
- 404 / 权限拒绝 EmptyState 收口（P13）
- 生产 Runbook + 权限能力矩阵可视化（P14）
- 强制修改 bootstrap 默认密码（P15）
- 实时 grants 总览 + 指标浏览（P16）
- Playwright 浏览器商业冒烟（P17）
- 备份演练引导 + Make/CI 入口（P18）
- 旁路恢复自动化 + TLS/HSTS doctor（P19）
- Admin doctor API + Overview 展示 + 边缘 HTTPS 验收清单（P20）
- data_dir 快照/旁路恢复 API + Storage 编排（P21）
- 可商用就绪中心 + 备份编排指引 + 清单持久化（P22）
- 备份编排脚本 + 就绪快捷动作 + 脚本自检（P23）
- 就绪评分含 doctor + Overview 入口 + CI 备份自检（P24）
- 就绪导出/导入/演练归档 + About 版本 + e2e 加深（P25）
- 会话常显/预警/到期登出 + 账户改密（P26）
- 通知容量去重/错误码映射/Overview 摘要（P27）
- 命令面板导航 + 审计筛选体验（P28）
- 跳过链接/aria + 运维操作历史导出（P29）

## 自动化覆盖（见 `productionChecklist.ts`）
| 项 | 严重度 | 自动化 |
|---|---|---|
| 边缘 HTTPS / TLS | required | 部分（doctor API + TLS 时 HSTS + 验收清单；边缘证书人工） |
| 安全响应头 | required | 是（服务侧测试） |
| 修改默认 admin 密码 | required | 是（must_change 门禁+单测） |
| 健康与指标接入 | required | 是（冒烟 + Dashboard /observability/metrics 浏览） |
| 备份与快照演练 | recommended | 是（清单 + data-snapshot/restore-drill API + 自动化） |
| 登录-查询-写入-运维冒烟 | required | 是（服务侧 smoke + Playwright UI） |
| 权限矩阵复核 | recommended | 是（矩阵页 + /access/grants 实时汇总） |

## 建议上线前再确认
1. 反向代理 HTTPS、HSTS（由边缘层配置）
2. 修改默认 admin 密码；生产禁止长期 `admin/admin`
3. 备份/快照与恢复演练
4. 浏览器冒烟：登录 → 查询 → 写入 → 运维 flush（人工或 Playwright）
5. 监控：`/healthz` `/readyz` `/metrics` 接入告警

## 核心冒烟路径
- `/login`
- `/` 概览
- `/query`
- `/write`
- `/databases`（admin）
- `/operations`（admin）
- `/storage`（admin）
- `/ops/readiness`（admin）
- `/about`
- `/account`

## 服务侧自动化入口
- `go test ./cmd/mts-server -run TestCommercialDashboardSmoke -count=1`
- `cd cmd/mts-dashboard && npm run test:e2e`
- `go test ./cmd/mts-server -run TestDataDirSidePathRestoreDrill -count=1`
- `go test ./cmd/mts-server -run TestAdminDoctorHTTP -count=1`
- `go test ./cmd/mts-server -run TestHTTPStorageDataSnapshotAndRestoreDrill -count=1`

## 运维脚本入口
- `scripts/mts-backup.sh` / `make backup-script-check`
- 文档：`docs/ops/backup-orchestration.md`

## 文档入口
- 生产 Runbook：`docs/ops/dashboard-production-runbook.md`
- 权限矩阵页：Dashboard `/access`


## P25 状态（2026-07-20）
- 就绪状态 JSON 导出/导入（merge/replace）+ 演练归档 JSON/Markdown
- `GET /api/v1/admin/version` + About 页（服务端 version/commit/built_at）
- Playwright：就绪勾选持久化、Storage data-snapshot、About
- **仍不宣称可商用目标完成**：真实边缘证书验收执行、目标环境 cron/systemd 安装与演练归档


## P26 状态（2026-07-20）
- 顶栏会话剩余时间常显；warn/critical 一次性 toast；到期自动登出
- `/account` 自愿改密（策略纯函数）+ 强制改密页复用策略
- Playwright 覆盖账户表单与会话徽章
- **仍不宣称可商用目标完成**：真实边缘证书验收、cron/systemd 与跨主机备份实装


## P27 状态（2026-07-20）
- 全局通知：容量上限、同文案去重、warn 级别
- API 错误码友好映射（formatCaughtError）接入主要管理页
- Overview 会话/客户端/服务端版本摘要条
- **仍不宣称可商用目标完成**：边缘证书验收、cron/systemd、跨主机备份


## P28 状态（2026-07-20）
- Ctrl/⌘+K 命令面板：过滤导航并跳转
- 审计页：快捷时间范围、客户端二次筛选、清空筛选、空导出提示
- **仍不宣称可商用目标完成**：边缘证书验收、cron/systemd、跨主机备份


## P29 状态（2026-07-20）
- 跳过链接到 main#main-content；侧栏 aria-current
- 运维页 session 操作历史 + JSON 导出
- **仍不宣称可商用目标完成**：边缘证书验收、cron/systemd、跨主机备份
