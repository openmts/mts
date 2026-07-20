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
- 路由脏态离开确认 + 用户/数据库筛选（P30）
- 验收导出包 + 降采样筛选/批量启停（P31）
- Overview 就绪评分 + e2e 加深（P32）
- 降采样 repair + 文案 i18n（P33）
- 降采样 run-range/状态明细/区间校验（P34）
- 文档标题与对话框 a11y（P35）
- 快捷键帮助与最近访问（P36）
- 查询/写入/配置/存储 i18n 收口（P37）
- 用户/数据库/运维 i18n 收口（P38）
- Metrics/Access/404 i18n 与 e2e 加深（P39）

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


## P30 状态（2026-07-20）
- 查询/写入脏表单路由离开确认
- 用户与数据库列表筛选 + 空态
- **仍不宣称可商用目标完成**：边缘证书验收、cron/systemd、跨主机备份


## P31 状态（2026-07-20）
- 就绪中心：验收材料一键导出包（JSON + Markdown，含 readiness/评分/doctor、客户端与可选服务端版本、会话运维操作历史）
- 降采样：名称/路径筛选、启用状态筛选、多选批量启停（ConfirmDialog）
- **仍不宣称可商用目标完成**：边缘证书验收、cron/systemd、跨主机备份


## P32 状态（2026-07-20）
- Overview 展示本地就绪评分摘要与跳转
- Playwright：验收包、降采样筛选/批量、Overview 评分
- **仍不宣称可商用目标完成**：边缘证书验收、cron/systemd、跨主机备份


## P33 状态（2026-07-20）
- 降采样：区间 repair 对话框 + `/repair` API
- 页面标题/创建等文案 i18n
- **仍不宣称可商用目标完成**：边缘证书验收、cron/systemd、跨主机备份


## P34 状态（2026-07-20）
- 统一区间对话框：repair / run-range / dry-run + advance_watermark
- statuses 明细表；区间校验纯函数与契约测试
- **仍不宣称可商用目标完成**：边缘证书验收、cron/systemd、跨主机备份


## P35 状态（2026-07-20）
- document.title 随路由/语言更新
- 降采样创建/区间对话框焦点陷阱
- 脏离开确认 i18n
- **仍不宣称可商用目标完成**：边缘证书验收、cron/systemd、跨主机备份


## P36 状态（2026-07-20）
- 快捷键帮助（? / 顶栏）+ 焦点陷阱
- 最近访问路由条（sessionStorage）
- TopBar 标题与 document.title 共用映射
- **仍不宣称可商用目标完成**：边缘证书验收、cron/systemd、跨主机备份


## P37 状态（2026-07-20）
- Query/Write/Config/Storage 用户可见文案 i18n + formatMessage
- **仍不宣称可商用目标完成**：边缘证书验收、cron/systemd、跨主机备份


## P38 状态（2026-07-20）
- Users/Databases/Operations 用户可见文案 i18n
- **仍不宣称可商用目标完成**：边缘证书验收、cron/systemd、跨主机备份


## P39 状态（2026-07-20）
- Metrics / AccessMatrix / AccessGrants / ApiSpec / NotFound i18n
- 权限等级标签随语言切换；Playwright 覆盖矩阵/授权/指标/404
- **仍不宣称可商用目标完成**：边缘证书验收、cron/systemd、跨主机备份

## P40 状态（2026-07-20）
- 权限矩阵 capability 行数据双语（area/capability/notes + areaKey 筛选）
- 矩阵单元测试全量双语；Playwright 覆盖 locale 切换后的矩阵行
- **仍不宣称可商用目标完成**：边缘证书验收、cron/systemd、跨主机备份；共享组件中文硬编码收口

## P41 状态（2026-07-20）
- 共享组件 i18n：ConfirmDialog / PermissionDenied / UserModals / UserGrantPanel
- 就绪中心自动覆盖文案 i18n
- **仍不宣称可商用目标完成**：边缘证书验收、cron/systemd、跨主机备份；清单数据层双语

## P42 状态（2026-07-20）
- 清单数据层双语：productionChecklist / backupDrill / edgeHttpsAcceptance / backupSchedule
- 共享 localizedText；Readiness/Storage 随 locale 展示
- **仍不宣称可商用目标完成**：边缘证书验收、cron/systemd、跨主机备份

## P43 状态（2026-07-20）
- 就绪归档/验收包按 locale 序列化 catalog（title/detail）与 Markdown 壳层双语
- 保留稳定 checklist id；导出入口透传 uiLocale
- **仍不宣称可商用目标完成**：边缘证书验收、cron/systemd、跨主机备份

## P44 状态（2026-07-20）
- 就绪中心部署材料包：证书验收命令、Nginx/HSTS、cron/systemd/env 样例 + 复制/下载
- 明确人工签核边界；不宣称部署验收完成
- **仍不宣称可商用目标完成**：边缘证书验收、cron/systemd 实装、跨主机备份告警

## P45 状态（2026-07-20）
- 验收包纳入 deploy_kit 索引摘要；Overview 增加部署材料包入口
- 就绪中心 #deploy-kit 锚点定位
- **仍不宣称可商用目标完成**：边缘证书验收、cron/systemd 实装、跨主机备份告警

## P46 状态（2026-07-20）
- 就绪归档纳入 deploy_kit 与本地提醒 hints
- 部署材料包本地查阅/下载/复制勾选（不计分）
- **仍不宣称可商用目标完成**：边缘证书验收、cron/systemd 实装、跨主机备份告警

## P47 状态（2026-07-20）
- 部署侧签核证据备注（边缘/异地备份/告警）持久化并进入归档
- 部署材料包补充 rsync-offsite 与 backup-alert-hook 样例
- **仍不宣称可商用目标完成**：边缘证书验收、cron/systemd 实装、跨主机备份告警

## P48 状态（2026-07-20）
- 签核备注完整性检测 + 导出 note 合成 + 缺失确认
- **仍不宣称可商用目标完成**：边缘证书验收、cron/systemd 实装、跨主机备份告警

## P49 状态（2026-07-20）
- Overview 签核完整性 + 验收包 signoff_completeness + #signoff-notes 锚点
- **仍不宣称可商用目标完成**：边缘证书验收、cron/systemd 实装、跨主机备份告警

## P50 状态（2026-07-20）
- 命令面板运维深链 + 统一 hash 锚点滚动
- **仍不宣称可商用目标完成**：边缘证书验收、cron/systemd 实装、跨主机备份告警

## P51 状态（2026-07-20）
- 导出前预检清单 + 同页 hash 监听滚动
- **仍不宣称可商用目标完成**：边缘证书验收、cron/systemd 实装、跨主机备份告警

## P52 状态（2026-07-20）
- 导出预检项一键跳转锚点（签核/部署材料/清单/Doctor）
- **仍不宣称可商用目标完成**：边缘证书验收、cron/systemd 实装、跨主机备份告警

## P53 状态（2026-07-20）
- 验收包 export_preflight + Overview 预检摘要入口
- **仍不宣称可商用目标完成**：边缘证书验收、cron/systemd 实装、跨主机备份告警

## P54 状态（2026-07-20）
- 归档 export_preflight + 预检复制 + Doctor i18n
- **仍不宣称可商用目标完成**：边缘证书验收、cron/systemd 实装、跨主机备份告警


## P55 状态（2026-07-20）
- Overview 健康态/Ready/Doctor 表头与空值 i18n；评分等级与 Doctor 分项标签本地化
- 预检/签核「建议下一步」面板（Overview + Readiness）+ 导出预检摘要 toast
- **仍不宣称可商用目标完成**：边缘证书验收、cron/systemd 实装、跨主机备份告警


## P56 状态（2026-07-20）
- Config/Storage/ApiSpec 表头与端点摘要 i18n；Overview HTTP TLS 标签本地化
- 关键管理页 emptyValue 统一；AccessGrants/ApiSpec 筛选 placeholder i18n
- **仍不宣称可商用目标完成**：边缘证书验收、cron/systemd 实装、跨主机备份告警


## P57 状态（2026-07-20）
- Query/Write/Downsample 表单标签与 placeholder i18n；查询结果列头本地化
- **仍不宣称可商用目标完成**：边缘证书验收、cron/systemd 实装、跨主机备份告警


## P58 状态（2026-07-20）
- Operations 动作卡/统计标签、图表 max series、横幅关闭、Config Token 标签 i18n
- **仍不宣称可商用目标完成**：边缘证书验收、cron/systemd 实装、跨主机备份告警


## P59 状态（2026-07-20）
- Overview 压缩/维护/内存统计标签 i18n；About 字段标签与降采样操作 title 本地化
- **仍不宣称可商用目标完成**：边缘证书验收、cron/systemd 实装、跨主机备份告警


## P60 状态（2026-07-20）
- Users/AccessGrants/Account 角色展示与筛选本地化
- **仍不宣称可商用目标完成**：边缘证书验收、cron/systemd 实装、跨主机备份告警


## P61 状态（2026-07-20）
- 权限矩阵角色列与库级权限标签 i18n；Storage 控制台内标记本地化
- **仍不宣称可商用目标完成**：边缘证书验收、cron/systemd 实装、跨主机备份告警


## P62 状态（2026-07-20）
- 权限矩阵角色分布/筛选文案与 Metrics 表头/样本计数 i18n
- **仍不宣称可商用目标完成**：边缘证书验收、cron/systemd 实装、跨主机备份告警


## P63 状态（2026-07-20）
- Databases 分区标题/字段类型与 Audit 导出按钮 i18n
- **仍不宣称可商用目标完成**：边缘证书验收、cron/systemd 实装、跨主机备份告警


## P64 状态（2026-07-20）
- Write 字段类型下拉 i18n
- **仍不宣称可商用目标完成**：边缘证书验收、cron/systemd 实装、跨主机备份告警


## P65 状态（2026-07-20）
- ActionResult 标签 / API 错误友好文案 i18n；HTTP status 推断主错误码；去掉 `[code]` 主文案前缀
- skip-link 聚焦主内容 + 全局 focus-visible 环（含深色）
- 部署材料验收边界说明、三步清单与 runbook 路径（不计分）
- 登录/改密/查询失败路径统一 formatCaughtError
- **仍不宣称可商用目标完成**：边缘证书验收、cron/systemd 实装、跨主机备份告警


## P66 状态（2026-07-20）
- ErrorBoundary / 会话过期标签 / 延迟水线 / 结果列标签 i18n
- API client 固定中文 message 收口为 code 友好路径；meta 列表错误 formatCaughtError
- 鉴权失败与跨标签登出 toast 使用 loginReasonMessage
- TopBar/Sidebar 图标按钮 aria-label；角色展示本地化
- **仍不宣称可商用目标完成**：边缘证书验收、cron/systemd 实装、跨主机备份告警


## P67 状态（2026-07-20）
- ConfirmDialog aria-describedby / data-testid；Shortcuts/Notify 关闭 aria-label
- 会话徽章 polite live；GlobalProgress progressbar 语义
- 命令面板 combobox/listbox 键盘语义
- 商业冒烟覆盖命令面板、快捷键帮助、skip-link 焦点
- **仍不宣称可商用目标完成**：边缘证书验收、cron/systemd 实装、跨主机备份告警


## P68 状态（2026-07-20）
- 登录/强制改密错误 live 区与 aria-invalid
- prefers-reduced-motion 全局减弱动画
- 浏览器离线顶栏提示（navigator.onLine）
- 商业冒烟覆盖登录校验与在线时无离线条
- **仍不宣称可商用目标完成**：边缘证书验收、cron/systemd 实装、跨主机备份告警


## P69 状态（2026-07-20）
- /readyz 可达性探测（与浏览器 offline 区分）；连续失败后展示不可达条 + 重试
- Account 改密 aria-invalid / alert 对齐登录页
- 商业冒烟：健康时无不可达条；账户改密校验错误可见
- **仍不宣称可商用目标完成**：边缘证书验收、cron/systemd 实装、跨主机备份告警


## P70 状态（2026-07-20）
- 服务可达性探测单例共享；Overview 连通性卡片与顶栏同源
- health check status i18n 映射
- 商业冒烟覆盖 Overview 连通性指示
- **仍不宣称可商用目标完成**：边缘证书验收、cron/systemd 实装、跨主机备份告警


## P71 状态（2026-07-20）
- Doctor level i18n（Overview/就绪中心）；TopBar 连通性迷你徽章
- Users/Config 表横向滚动；打印隐藏导航与瞬时条
- 商业冒烟覆盖 topbar-connectivity
- **仍不宣称可商用目标完成**：边缘证书验收、cron/systemd 实装、跨主机备份告警


## P72 状态（2026-07-20）
- 部署 Runbook 联调清单（边缘证书 / cron·systemd / 异地备份+告警）；复制与下载 Markdown
- 部署材料包默认附带联调附录；不计就绪评分
- 商业冒烟覆盖联调清单 testid
- **仍不宣称可商用目标完成**：边缘证书验收、cron/systemd 实装、跨主机备份告警


## P73 状态（2026-07-20）
- 登录可选会话 TTL（`ttl_seconds`）；非法 TTL 本地拦截
- 查询/表单写/Line Protocol 校验错误完整 i18n
- 商业冒烟覆盖 login-ttl
- **仍不宣称可商用目标完成**：边缘证书验收、cron/systemd 实装、跨主机备份告警


## P74 状态（2026-07-20）
- 流式查询预览 footer i18n；RP duration 纯函数与本地化错误
- 账户页会话过期/剩余展示
- 商业冒烟覆盖 account-session
- **仍不宣称可商用目标完成**：边缘证书验收、cron/systemd 实装、跨主机备份告警


## P75 状态（2026-07-20）
- 写入默认 TypedBatch + 本机偏好记忆；审计 limit/total/持久化合并提示；登录 TTL 记忆
- 商业冒烟覆盖 write-mode 与 audit-limit
- **仍不宣称可商用目标完成**：边缘证书验收、cron/systemd 实装、跨主机备份告警


## P76 状态（2026-07-20）
- 存储页 data-snapshot 演练源选择/复制路径；配置导出统一下载工具
- 商业冒烟覆盖 storage-drill-source
- **仍不宣称可商用目标完成**：边缘证书验收、cron/systemd 实装、跨主机备份告警


## P77 状态（2026-07-20）
- 审计/查询导出统一 download+stampFilename；审计 CSV 纯函数化
- **仍不宣称可商用目标完成**：边缘证书验收、cron/systemd 实装、跨主机备份告警


## P78 状态（2026-07-20）
- Metrics 自动刷新、raw/JSON 导出、展开折叠与 testid
- **仍不宣称可商用目标完成**：边缘证书验收、cron/systemd 实装、跨主机备份告警
