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


## P79 状态（2026-07-20）
- Config 生效配置/Schema/错误码导出与复制；运维维护错误导出/复制
- 商业冒烟覆盖 config/ops 导出 testid
- **仍不宣称可商用目标完成**：边缘证书验收、cron/systemd 实装、跨主机备份告警


## P80 状态（2026-07-20）
- Access grants JSON/CSV 导出；权限矩阵 JSON 导出；API Spec JSON/Markdown 导出
- 商业冒烟覆盖 access-matrix / access-grants / api-spec 导出 testid
- **仍不宣称可商用目标完成**：边缘证书验收、cron/systemd 实装、跨主机备份告警


## P81 状态（2026-07-20）
- 数据库/用户/降采样筛选清单 JSON+CSV 导出
- 商业冒烟覆盖 databases/users/downsample 导出 testid
- **仍不宣称可商用目标完成**：边缘证书验收、cron/systemd 实装、跨主机备份告警


## P82 状态（2026-07-20）
- Storage 导出拉取/下载/复制；Query 导出 testid；About 构建信息导出复制
- 商业冒烟覆盖 storage-export / query-export / about-export
- **仍不宣称可商用目标完成**：边缘证书验收、cron/systemd 实装、跨主机备份告警


## P83 状态（2026-07-20）
- Overview 运维快照导出/复制；Write 结果与草稿导出
- 商业冒烟覆盖 overview-export / write-export testid
- **仍不宣称可商用目标完成**：边缘证书验收、cron/systemd 实装、跨主机备份告警


## P84 状态（2026-07-20）
- Operations 统计导出/复制；Account 会话快照导出/复制；404 导航 testid
- 商业冒烟覆盖 ops-export-stats / account-export / not-found
- **仍不宣称可商用目标完成**：边缘证书验收、cron/systemd 实装、跨主机备份告警


## P85 状态（2026-07-20）
- 侧栏折叠记忆、面包屑导航、改密策略分项实时提示
- 商业冒烟覆盖 sidebar-collapse / breadcrumb / password-hints
- **仍不宣称可商用目标完成**：边缘证书验收、cron/systemd 实装、跨主机备份告警


## P86 状态（2026-07-20）
- 登录密码显隐、记住用户名；面包屑复制路径
- 商业冒烟覆盖 login-toggle/remember 与 breadcrumb-copy
- **仍不宣称可商用目标完成**：边缘证书验收、cron/systemd 实装、跨主机备份告警


## P87 状态（2026-07-20）
- 最近访问清空；侧栏展开态导航过滤（折叠隐藏）
- 商业冒烟覆盖 sidebar-filter / recent-routes-clear
- **仍不宣称可商用目标完成**：边缘证书验收、cron/systemd 实装、跨主机备份告警


## P88 状态（2026-07-20）
- 全局 `/` 聚焦侧栏导航过滤；快捷键帮助登记
- 商业冒烟覆盖 sidebar-filter 焦点
- **仍不宣称可商用目标完成**：边缘证书验收、cron/systemd 实装、跨主机备份告警


## P89 状态（2026-07-20）
- 登录落地页本机偏好；命令面板空查询展示最近访问
- 商业冒烟覆盖 account-landing / command-palette-recent
- **仍不宣称可商用目标完成**：边缘证书验收、cron/systemd 实装、跨主机备份告警


## P90 状态（2026-07-20）
- 侧栏导航分组；界面密度舒适/紧凑本机记忆
- 商业冒烟覆盖 sidebar-section / account-density
- **仍不宣称可商用目标完成**：边缘证书验收、cron/systemd 实装、跨主机备份告警


## P91 状态（2026-07-20）
- 最近访问固定；通知历史面板（session）
- 商业冒烟覆盖 recent pin / notify-history
- **仍不宣称可商用目标完成**：边缘证书验收、cron/systemd 实装、跨主机备份告警


## P92 状态（2026-07-20）
- 通知历史 JSON/CSV 导出与复制；Ctrl/⌘+Shift+H 切换面板
- 商业冒烟覆盖 notify-history-export / 快捷键
- **仍不宣称可商用目标完成**：边缘证书验收、cron/systemd 实装、跨主机备份告警


## P93 状态（2026-07-20）
- 命令面板最近访问固定优先；账户快照含本机偏好
- 商业冒烟覆盖 command-recent data-pinned
- **仍不宣称可商用目标完成**：边缘证书验收、cron/systemd 实装、跨主机备份告警


## P94 状态（2026-07-20）
- 本机偏好重置/导入；通知历史类型过滤
- 商业冒烟覆盖 account-prefs / notify-history-filter
- **仍不宣称可商用目标完成**：边缘证书验收、cron/systemd 实装、跨主机备份告警


## P95 状态（2026-07-20）
- 本机偏好独立导出/复制；通知历史文本搜索
- 商业冒烟覆盖 account-prefs-export/copy 与 notify-history-search
- **仍不宣称可商用目标完成**：边缘证书验收、cron/systemd 实装、跨主机备份告警


## P96 状态（2026-07-20）
- 通知历史快捷/自定义时间范围过滤与清除筛选
- 商业冒烟覆盖 notify-history-time-*
- **仍不宣称可商用目标完成**：边缘证书验收、cron/systemd 实装、跨主机备份告警


## P97 状态（2026-07-20）
- 侧栏分组内导航自定义排序本机记忆；过滤/折叠时隐藏排序
- 商业冒烟覆盖 sidebar-order-*
- **仍不宣称可商用目标完成**：边缘证书验收、cron/systemd 实装、跨主机备份告警


## P98 状态（2026-07-20）
- 偏好导出/导入/重置纳入侧栏导航排序（nav_order）
- 商业冒烟覆盖排序持久化与 prefs-reset 清除
- **仍不宣称可商用目标完成**：边缘证书验收、cron/systemd 实装、跨主机备份告警


## P99 状态（2026-07-20）
- 命令面板运维深链（Flush/Compact/Retention/动作日志/维护错误）+ Operations hash 锚点
- 商业冒烟覆盖 operations 深链
- **仍不宣称可商用目标完成**：边缘证书验收、cron/systemd 实装、跨主机备份告警


## P100 状态（2026-07-20）
- Metrics/Config/Audit/Downsample 锚点 + 命令面板深链；useHashScroll 统一
- 商业冒烟覆盖 config-effective / audit-filters
- **仍不宣称可商用目标完成**：边缘证书验收、cron/systemd 实装、跨主机备份告警


## P101 状态（2026-07-20）
- Query/Write 工作台锚点与命令面板深链；hash 驱动历史/图表/写入模式
- 商业冒烟覆盖 query-history / write-mode-typed
- **仍不宣称可商用目标完成**：边缘证书验收、cron/systemd 实装、跨主机备份告警


## P102 状态（2026-07-20）
- 命令面板页内动作（主题/语言/密度/侧栏过滤/通知/快捷键/折叠）
- 商业冒烟覆盖 action-toggle-theme
- **仍不宣称可商用目标完成**：边缘证书验收、cron/systemd 实装、跨主机备份告警


## P103 状态（2026-07-20）
- 主内容返回顶部按钮 + 命令面板动作；scrollTop 纯函数单测
- 商业冒烟覆盖 back-to-top
- **仍不宣称可商用目标完成**：边缘证书验收、cron/systemd 实装、跨主机备份告警


## P104 状态（2026-07-20）
- 命令面板导航/动作分组展示；空组隐藏
- 商业冒烟覆盖 command-palette-group-nav/action
- **仍不宣称可商用目标完成**：边缘证书验收、cron/systemd 实装、跨主机备份告警


## P105 状态（2026-07-20）
- 命令面板 Home/End 键盘导航 + 索引 map 优化 + 选中项滚入视口
- 商业冒烟覆盖 Home/End 选中态
- **仍不宣称可商用目标完成**：边缘证书验收、cron/systemd 实装、跨主机备份告警


## P106 状态（2026-07-20）
- 路由 path 切换主内容自动回顶；同页 hash 深链不回顶
- 商业冒烟覆盖跨页 scrollTop 归零
- **仍不宣称可商用目标完成**：边缘证书验收、cron/systemd 实装、跨主机备份告警


## P107 状态（2026-07-20）
- 侧栏组内 HTML5 拖拽排序；保留 ↑↓ 无障碍回退；折叠/过滤隐藏拖拽手柄
- 商业冒烟覆盖 sidebar-drag-* 与重排 localStorage 持久化
- **仍不宣称可商用目标完成**：边缘证书验收、cron/systemd 实装、跨主机备份告警


## P108 状态（2026-07-20）
- 命令面板空查询默认折叠导航深链；可展开/收起；分组计数与色点增强
- 商业冒烟覆盖 command-palette-nav-expand 与深链显隐
- **仍不宣称可商用目标完成**：边缘证书验收、cron/systemd 实装、跨主机备份告警


## P109 状态（2026-07-20）
- 命令面板新增复制当前页 URL / 聚焦主内容 / 刷新当前页（安全页内动作）
- 商业冒烟覆盖 copy-page-url 与 focus/reload 入口
- **仍不宣称可商用目标完成**：边缘证书验收、cron/systemd 实装、跨主机备份告警


## P110 状态（2026-07-20）
- Users/Databases 多选、全选过滤结果、清空选择；导出优先已选行
- Users 批量启用/禁用（确认框，跳过当前用户与已达目标状态）
- 商业冒烟覆盖 selection toolbar
- **仍不宣称可商用目标完成**：边缘证书验收、cron/systemd 实装、跨主机备份告警


## P111 状态（2026-07-20）
- Users 表头列排序（name/role/status）+ sticky 表头；Databases 名称排序
- 排序本机记忆；商业冒烟覆盖 users-sort / databases-sort
- **仍不宣称可商用目标完成**：边缘证书验收、cron/systemd 实装、跨主机备份告警


## P112 状态（2026-07-20）
- Audit / Access Grants 多选导出、列排序本机记忆、sticky 表头
- 商业冒烟覆盖 selection toolbar 与 sort prefs
- **仍不宣称可商用目标完成**：边缘证书验收、cron/systemd 实装、跨主机备份告警


## P113 状态（2026-07-20）
- Retention 二次确认需输入 RETENTION；清空操作历史改为 ConfirmDialog
- 商业冒烟覆盖 require-text 与 clear-log 确认
- **仍不宣称可商用目标完成**：边缘证书验收、cron/systemd 实装、跨主机备份告警


## P114 状态（2026-07-20）
- Access Matrix：搜索、多选导出 JSON/CSV、列排序本机记忆、sticky 表头
- 商业冒烟覆盖 search/select/sort/csv
- **仍不宣称可商用目标完成**：边缘证书验收、cron/systemd 实装、跨主机备份告警


## P115 状态（2026-07-20）
- 抽取 ListSelectionToolbar；Users/Databases/Access Matrix/Access Grants 接入统一选择工具条
- 商业冒烟既有 selection testid 兼容
- **仍不宣称可商用目标完成**：边缘证书验收、cron/systemd 实装、跨主机备份告警


## P116 状态（2026-07-20）
- Audit / Downsample 接入 ListSelectionToolbar；Downsample 兼容 clear-select testid
- 商业冒烟覆盖 downsample-selection-toolbar
- **仍不宣称可商用目标完成**：边缘证书验收、cron/systemd 实装、跨主机备份告警


## P117 状态（2026-07-20）
- 签核备注实时保存、进度条、缺失字段跳转/复制；字段完成徽章
- 仍声明备注≠生产验收；不计入 readiness 总分
- 商业冒烟覆盖 signoff-progress 与实时保存
- **仍不宣称可商用目标完成**：边缘证书验收、cron/systemd 实装、跨主机备份告警

## P118 状态（2026-07-20）
- 部署材料样例与签核字段双向跳转；命令面板补充导出预检/联调清单深链
- 仍声明备注≠生产验收；不计入 readiness 总分
- 商业冒烟覆盖 deploy→signoff 串联
- **仍不宣称可商用目标完成**：边缘证书验收、cron/systemd 实装、跨主机备份告警

## P119 状态（2026-07-20）
- Overview 签核进度条与缺失字段跳转；仍不计入 readiness 总分
- 商业冒烟覆盖 overview-signoff-panel
- **仍不宣称可商用目标完成**：边缘证书验收、cron/systemd 实装、跨主机备份告警

## P120 状态（2026-07-20）
- 命令面板只读预填：查询/审计时间与筛选深链；不自动执行危险写
- 商业冒烟覆盖 query/audit prefill
- **仍不宣称可商用目标完成**：边缘证书验收、cron/systemd 实装、跨主机备份告警

## P121 状态（2026-07-20）
- Audit 事件表虚拟滚动；选择/导出仍覆盖筛选全集
- 商业冒烟覆盖 audit-table/header
- **仍不宣称可商用目标完成**：边缘证书验收、cron/systemd 实装、跨主机备份告警

## P122 状态（2026-07-20）
- Users 列表虚拟滚动；选择/导出仍覆盖筛选全集
- 商业冒烟覆盖 users-table/virtual-list
- **仍不宣称可商用目标完成**：边缘证书验收、cron/systemd 实装、跨主机备份告警

## P123 状态（2026-07-20）
- Access Grants 列表虚拟滚动；选择/导出仍覆盖筛选全集
- 商业冒烟在有数据时覆盖 virtual-list
- **仍不宣称可商用目标完成**：边缘证书验收、cron/systemd 实装、跨主机备份告警

## P124 状态（2026-07-20）
- Access Matrix 虚拟滚动；选择/导出仍覆盖筛选全集
- 商业冒烟覆盖 access-matrix-virtual-list
- **仍不宣称可商用目标完成**：边缘证书验收、cron/systemd 实装、跨主机备份告警

## P125 状态（2026-07-20）
- Databases 顶层库列表虚拟滚动；详情单展开面板
- 选择/导出仍覆盖筛选全集；商业冒烟覆盖 databases-virtual-list
- **仍不宣称可商用目标完成**：边缘证书验收、cron/systemd 实装、跨主机备份告警

## P126 状态（2026-07-20）
- Operations 只读状态条：连通性（/readyz）+ 统计刷新时间
- 商业冒烟覆盖 ops-status-strip
- **仍不宣称可商用目标完成**：边缘证书验收、cron/systemd 实装、跨主机备份告警

## P127 状态（2026-07-20）
- Downsample 策略表/状态表虚拟滚动；选择/导出仍覆盖筛选全集
- 商业冒烟覆盖 downsample-virtual-list
- **仍不宣称可商用目标完成**：边缘证书验收、cron/systemd 实装、跨主机备份告警

## P128 状态（2026-07-20）
- Storage 配置快照与 data_dir 快照列表虚拟滚动
- 修复 loading EmptyState description 绑定
- 商业冒烟覆盖 storage-*-virtual-list
- **仍不宣称可商用目标完成**：边缘证书验收、cron/systemd 实装、跨主机备份告警

## P129 状态（2026-07-20）
- Config schema / error-codes 表虚拟滚动；错误码筛选+导出覆盖筛选结果
- 商业冒烟覆盖 config-*-virtual-list
- **仍不宣称可商用目标完成**：边缘证书验收、cron/systemd 实装、跨主机备份告警

## P130 状态（2026-07-20）
- Metrics family 列表虚拟滚动；样本单展开详情面板
- 商业冒烟覆盖 metrics-virtual-list
- **仍不宣称可商用目标完成**：边缘证书验收、cron/systemd 实装、跨主机备份告警

## P131 状态（2026-07-20）
- Operations 动作日志虚拟滚动 + 筛选；历史上限 200；导出覆盖筛选结果
- 商业冒烟 sessionStorage 种子与 virtual-list 断言
- **仍不宣称可商用目标完成**：边缘证书验收、cron/systemd 实装、跨主机备份告警

## P132 状态（2026-07-20）
- 通知历史 VirtualTable + 上限 200；维护错误筛选/虚拟滚动
- 商业冒烟覆盖 notify-history-virtual-list / ops-maint-errors-virtual-list（有数据时）
- **仍不宣称可商用目标完成**：边缘证书验收、cron/systemd 实装、跨主机备份告警

## P133 状态（2026-07-20）
- Query 行结果 virtual-list testid/hint；列摘要 VirtualTable
- 商业冒烟覆盖 query-*-virtual-list（有结果时）
- **仍不宣称可商用目标完成**：边缘证书验收、cron/systemd 实装、跨主机备份告警

## P134 状态（2026-07-20）
- Query 历史全量 VirtualTable + 筛选；上限 200
- 商业冒烟覆盖 query-history-virtual-list（有历史时）
- **仍不宣称可商用目标完成**：边缘证书验收、cron/systemd 实装、跨主机备份告警

## P135 状态（2026-07-20）
- 命令面板结果密度：紧凑行高 + sticky 分组 + 结果计数
- 商业冒烟覆盖 command-palette-result-count / data-density
- **仍不宣称可商用目标完成**：边缘证书验收、cron/systemd 实装、跨主机备份告警

## P136 状态（2026-07-20）
- Readiness 签核字段填写引导 + 空白示例填充；不计入评分
- 商业冒烟覆盖 signoff-guide-* 与 backupOffsite 示例
- **仍不宣称可商用目标完成**：边缘证书验收、cron/systemd 实装、跨主机备份告警

## P137 状态（2026-07-21）
- ApiSpec / Overview health·doctor·maint / Readiness doctor 虚拟滚动
- 全量检视报告：docs/review/code-review-2026-07-21-0002-mts-dashboard-full.md
- **仍不宣称可商用目标完成**：边缘证书验收、cron/systemd 实装、跨主机备份告警

## P138 状态（2026-07-21）
- Write 表单写行上限 50 + RP 手填提示
- **仍不宣称可商用目标完成**：边缘证书验收、cron/systemd 实装、跨主机备份告警

## P139 状态（2026-07-21）
- Query predicates DSL UI 已实现（tag/field 比较；无完整 expr 树）
- 入口：`query-predicates`；解析：`parsePredicates` → `Query.predicates`
- 门禁：dashboard unit/build/e2e + `make e2e` + `go test ./...` 通过
- **仍不宣称可商用目标完成**：边缘证书验收、cron/systemd 实装、跨主机备份告警

## P140 状态（2026-07-21）
- Data 面只读 RP：`GET /api/v1/data/databases/{db}/retention-policies`
- Dashboard 优先 data 路径，回退 admin；闭环非 admin 元数据缺口
- **仍不宣称可商用目标完成**：边缘证书验收、cron/systemd 实装、跨主机备份告警

## P141 状态（2026-07-21）
- 子路径部署：`VITE_BASE` 与 `http.dashboard_base` 对齐说明（代码侧已支持）
- **仍不宣称可商用目标完成**：边缘证书验收、cron/systemd 实装、跨主机备份告警

## P142 状态（2026-07-21）
- Query fields datalist + series 选择器填充 tags（上限 200）
- meta：`listFields` / `listSeries`；失败可手填
- **仍不宣称可商用目标完成**：边缘证书验收、cron/systemd 实装、跨主机备份告警

## P143 状态（2026-07-21）
- 独立 `GET /api/v1/data/query/stats` 入口（Query 页 + 命令面板）
- stats 来源区分、详细指标折叠、空态引导
- **仍不宣称可商用目标完成**：边缘证书验收、cron/systemd 实装、跨主机备份告警

## P144 状态（2026-07-21）
- Account 落地页：筛选/分组/当前态/EmptyState；保留 select 降级
- **仍不宣称可商用目标完成**：边缘证书验收、cron/systemd 实装、跨主机备份告警

## P145 状态（2026-07-21）
- Data 面库列表：`GET /api/v1/data/databases`（按 read 权限过滤）
- Dashboard 优先 data 回退 admin，闭环非 admin 库下拉
- **仍不宣称可商用目标完成**：边缘证书验收、cron/systemd 实装、跨主机备份告警

## P146 状态（2026-07-21）
- Query series 选择器客户端筛选（标签/自由文本）
- 计数与空匹配文案；仍上限 200，无服务端分页
- **仍不宣称可商用目标完成**：边缘证书验收、cron/systemd 实装、跨主机备份告警

## P147 状态（2026-07-21）
- Write 页 measurement/field 元数据建议（datalist + 芯片填充）
- 失败可手填；与 Query meta 共用 data 面 API
- **仍不宣称可商用目标完成**：边缘证书验收、cron/systemd 实装、跨主机备份告警

## P148 状态（2026-07-21）
- Downsample 创建策略：source/target db·measurement·field 元数据 datalist
- 打开创建面板加载建议；失败可手填
- **仍不宣称可商用目标完成**：边缘证书验收、cron/systemd 实装、跨主机备份告警

## P149 状态（2026-07-21）
- 修复 Query 范围删除确认死锁（ConfirmDialog requireText 门禁）
- 删除确认展示范围摘要 + 无时间警告
- **仍不宣称可商用目标完成**：边缘证书验收、cron/systemd 实装、跨主机备份告警

## P150 状态（2026-07-21）
- UserGrantPanel：库筛选、空态、选择提示、授权按钮 disabled
- **仍不宣称可商用目标完成**：边缘证书验收、cron/systemd 实装、跨主机备份告警

## P151 状态（2026-07-21）
- `/databases` 非 admin 只读浏览 data 面库/measurement/fields/series
- 隐藏创建删除库与新建 RP；admin 能力不变
- **仍不宣称可商用目标完成**：边缘证书验收、cron/systemd 实装、跨主机备份告警

## P152 状态（2026-07-21）
- Query series「按 Tags 刷新」走服务端 tag 过滤
- **仍不宣称可商用目标完成**

## P153 状态（2026-07-21）
- Databases measurement series 上限 200 + total 提示
- **仍不宣称可商用目标完成**

## P154 状态（2026-07-21）
- Users 打开授权面板 e2e + `users-open-grant-*`
- **仍不宣称可商用目标完成**

## P155 状态（2026-07-21）
- series API `limit`/`total`/`truncated`；保留字 limit/offset/page/q 不当 tag
- 前端 `listSeriesDetailed` 消费服务端截断
- **仍不宣称可商用目标完成**

## P156 状态（2026-07-21）
- `/databases` 侧栏/命令面板对所有登录用户可见；路由去掉 admin meta
- **仍不宣称可商用目标完成**

## P157 状态（2026-07-21）
- 落地页允许非 admin 偏好 `/databases`；RBAC 拆分 browse/manage；冒烟路径 admin:false
- **仍不宣称可商用目标完成**

## P158 状态（2026-07-21）
- Databases 详情 measurement 筛选/计数/空态
- **仍不宣称可商用目标完成**

## P159 状态（2026-07-21）
- Databases → Query/Write 深链预填（不自动执行/提交）
- **仍不宣称可商用目标完成**

## P160 状态（2026-07-21）
- Playwright 非 admin：创建 reader、授权、只读库浏览、管理页拒绝
- **仍不宣称可商用目标完成**

## P161 状态（2026-07-21）
- 能力矩阵覆盖 databases-browse 文案
- **仍不宣称可商用目标完成**

## P162 状态（2026-07-21）
- 非 admin Overview 数据面工作区快捷入口
- **仍不宣称可商用目标完成**

## P163 状态（2026-07-21）
- Databases RP 空态 EmptyState（admin/只读文案）
- **仍不宣称可商用目标完成**

## P164 状态（2026-07-21）
- Write 预填/切换 measurement 自动加载 field 元数据
- **仍不宣称可商用目标完成**

## P165 状态（2026-07-21）
- Query 分享预填链接：多字段深链 + 复制分享按钮 + i18n + 单测/e2e

## P166 状态（2026-07-21）
- 非 admin 自身审计：`GET /users/{self}/audit` + Audit 页客户端过滤 + 导航开放

## P167 状态（2026-07-21）
- 库页 series 行一键跳转 Query（tags 预填）

## P168 状态（2026-07-21）
- Query 分享链接写入绝对 start_time/end_time（优先于 range）

## P169 状态（2026-07-21）
- 自身审计 API 支持 action/since_unix/until_unix/limit 服务端过滤

## P170 状态（2026-07-21）
- 非 admin Overview 工作区增加审计入口

## P171 状态（2026-07-21）
- Write 页复制预填深链（database/measurement，无 payload）

## P172 状态（2026-07-21）
- Audit 页复制筛选深链（range/action/user/q）

## P173 状态（2026-07-21）
- Databases 深链预填（database/q）与复制库页链接

## P174 状态（2026-07-21）
- 命令面板库页筛选/详情锚点

## P175 状态（2026-07-21）
- Users 筛选深链（q/role/user）与复制筛选链接

## P176 状态（2026-07-21）
- Access 矩阵筛选深链（role/area/q）与复制筛选链接

## P177 状态（2026-07-21）
- Access Grants 筛选深链（user/database/permission/q）与复制筛选链接

## P178 状态（2026-07-21）
- Downsample 策略筛选深链（q/enabled）与复制筛选链接

## P179 状态（2026-07-21）
- Operations 筛选深链（maint_q/action_kind/status/q）与复制筛选链接

## P180 状态（2026-07-21）
- Storage 区块分享深链（backup-drill/edge-https/data-restore/snapshots）

## P181 状态（2026-07-21）
- Readiness 区块分享深链（export-preflight/deploy-kit/signoff-notes/deploy-runbook-drill/readiness-action）

## P182 状态（2026-07-21）
- Config 筛选/区块深链（schema_q/error_q）与 Metrics 筛选深链（q/family）

## P183 状态（2026-07-21）
- ApiSpec 筛选深链（ns/q）与复制分享

## P184 状态（2026-07-21）
- Account 落地页筛选深链（landing_q）与复制分享
## P185 状态（2026-07-21）
- Overview 区块分享深链

## P186 状态（2026-07-21）
- About 区块分享深链

## P187 状态（2026-07-21）
- 登录/强制改密 redirect 透传与展示

## P188 状态（2026-07-21）
- NotFound 路径展示与快捷恢复

## P189 状态（2026-07-21）
- 命令面板「复制当前筛选深链」+ 敏感 query 清洗

## P190 状态（2026-07-21）
- 非 admin 深链/分享/登录回跳 e2e 加深

## P191 状态（2026-07-21）
- 登出/会话过期保留 redirect

## P192 状态（2026-07-21）
- 会话徽章跳转与账户续期引导

## P193 状态（2026-07-21）
- 通知历史筛选深链、自动打开与复制分享

## P194 状态（2026-07-21）
- 命令面板错误通知深链 e2e

## P195 状态（2026-07-21）
- 离线写/运维/删除门禁

## P196 状态（2026-07-21）
- 快捷键帮助深链与命令面板入口

## P197 状态（2026-07-21）
- Users/Downsample 创建草稿脏离开守卫

## P198 状态（2026-07-21）
- Users/Downsample/Databases 管理写离线门禁

## P199 状态（2026-07-21）
- Config/Storage 管理写离线门禁

## P200 状态（2026-07-21）
- Databases 创建/RP 草稿脏离开守卫

## P201 状态（2026-07-21）
- 统一导出进度/取消（Query/Audit/Users）

## P202 状态（2026-07-21）
- Write 提交 Abort 取消

## P203 状态（2026-07-21）
- Databases/Downsample/AccessGrants/Operations 导出接入 ExportJob

## P204 状态（2026-07-21）
- AccessMatrix/Metrics/Config/ApiSpec 导出接入 ExportJob

## P205 状态（2026-07-21）
- Readiness 导出统一进度/取消（含双文件 bundle）

## P206 状态（2026-07-21）
- Account 密码+TTL 会话续期（无独立 refresh token）

## P207 状态（2026-07-21）
- 剩余小导出统一 ExportJob：Query history / Overview / About / Ops stats+errors / Storage / Account / Write / NotifyHistory
- 页面层不再直连 downloadJSON/downloadText

## P208 状态（2026-07-21）
- Account 改密/会话续期离线门禁
- 改密表单脏离开守卫（badge / routeDirty / beforeunload）

## P209 状态（2026-07-21）
- 登录离线门禁
- 强制改密离线门禁 + 脏离开守卫

## P210 状态（2026-07-21）
- Config 服务 Token 脏离开守卫（badge / routeDirty / beforeunload）
- 商业冒烟加深：Account dirty + Overview/About/Write 导出 banner

## P211 状态（2026-07-21）
- Storage 导出/校验等管理操作离线门禁
- 登录离线 e2e 覆盖

## P212 状态（2026-07-21）
- Operations 运维卡片离线禁用
- Downsample 批量操作离线禁用

## P213 状态（2026-07-21）
- Users 行级/批量/弹窗写操作离线禁用
- Databases 删除与 RP 添加离线禁用

## P214 状态（2026-07-21）
- Downsample 行级写操作与创建入口离线禁用
- Query 范围删除离线禁用

## P215 状态（2026-07-21）
- 会话 critical/expired 统一写操作只读门禁 + 布局 banner
- 主写路径接入 useMutationGuard（登录/续期除外）

## P216 状态（2026-07-21）
- Storage 快照删除入口离线/会话门禁

## P217（2026-07-21）
- Readiness 清单/签核：**相对进页基线**的会话脏离开守卫（`registerDirtyChecker` + `beforeunload` + dirty badge）
- 导入成功后重置基线；变更仍自动写入 localStorage（非服务端未保存语义）
- 部署侧边缘证书/cron/跨主机备份仍为人工签核项，dashboard 仅清单与证据备注

## P218（2026-07-21）
- Users 弹窗/授权面板：`writeBlocked` + `blockReason`，会话 critical 不再误提示离线

## P219（2026-07-21）
- 路由离开确认：form（未提交）与 local（本机已自动落盘）文案分流；Readiness 使用 local

## P220（2026-07-21）
- ConfirmDialog：离线/会话 critical 禁用确认 + 阻断提示条；危险写操作确认接入
- 本地清理（查询历史、运维本地日志）不接入写门禁

## P221（2026-07-21）
- 会话 critical 横幅：一键「去续期」「重新登录」
- Downsample 范围对话框 writeBlocked 阻断提示条

## P222（2026-07-21）
- 写门禁文案统一：`blockedMessageKey` / `mutationBlockedMessageKey`

## P223（2026-07-21）
- API 默认超时 30s（probe 5s）；超时友好错误 `timeout`
- NDJSON 流式查询不套默认超时

## P224（2026-07-21）
- API 超时可配置：`VITE_API_TIMEOUT_MS`；运维 runbook 补充超时策略

## P225（2026-07-21）
- 非 admin 会话 critical：写禁用 + 横幅续期 e2e

## P226（2026-07-21）
- 长请求全局进度：`longBusy` 文案条（≥1.2s）

## P227（2026-07-21）
- 导出错误路径 `formatCaughtError`
- AbortError→canceled（不再误标超时）；Query 取消清 loading + `queryCancelled` toast
- Write 取消统一 `isCanceledError`

## P228（2026-07-21）
- Readiness 导入错误 `formatCaughtError`
- Query 取消态 info 样式 + `query-cancel` 空闲 disabled e2e

## P229（2026-07-21）
- Write 取消态 info 样式 + `write-action-error`

## P230（2026-07-21）
- ExportJobBanner 状态色分流 + `data-export-status`

## P231（2026-07-21）
- 查询取消 e2e（route 延迟 mock + 取消文案/样式/loading）

## P232（2026-07-21）
- 取消类 toast 统一 `info`（导出/查询/写入）

## P233（2026-07-21）
- 查询超时 mock e2e + 写入取消 e2e

## P234（2026-07-21）
- 写入超时 mock e2e
- 导出取消 e2e（`__MTS_E2E_SLOW_EXPORT_MS`）
- `exportYieldMs` 工具

## P235（2026-07-21）
- Query/Write actionError live region（alert/status）
- clipboard 错误友好化

## P236（2026-07-21）
- 扩展结果条 live region（Query/Write/ApiSpec/Config/Audit）

## P237（2026-07-21）
- ActionResultBanner 可选重试
- 关键页 loadError 一键重试

## P238（2026-07-21）
- Config/Operations/Readiness doctor 加载失败重试

## P239（2026-07-21）
- Query 结果 fields 去掉 `as any`

## P240（2026-07-21）
- Overview 管理统计部分失败可重试
- ApiSpec 加载失败重试

## P241（2026-07-21）
- Storage 快照列表错误可见 + 重试

## P242（2026-07-21）
- 命令面板补 storage snapshots / account session·password·density 深链

## P243（2026-07-21）
- Overview 分项失败标签化
- Query series 失败可重试

## P244（2026-07-21）
- Query 引擎统计错误可重试
- Query meta 提示条可重新加载库列表

## P245（2026-07-21）
- Write 元数据提示/错误可重试

## P246（2026-07-21）
- Overview 管理分项（内存/compaction/维护/doctor/版本）独立重试按钮

## P247（2026-07-21）
- 错误/警告 toast 提供「通知历史」快捷入口（notifyHistoryBridge）

## P248（2026-07-21）
- Users 批量启停失败列出失败用户名

## P249（2026-07-21）
- Storage 操作失败 ActionResult 可重试（lastFailedAction）

## P250（2026-07-21）
- Config schema / Users 授权库列表失败可见可重试

## P251（2026-07-21）
- Operations flush/compact/retention 失败可重试；统计分项失败可见

## P252（2026-07-21）
- Downsample / Databases 写与详情加载失败 ActionResult 可重试

## P253（2026-07-21）
- AccessGrants 分项失败可重试；Users/Config 写操作失败可重试

## P254（2026-07-21）
- 抽取 createActionRetry / useActionRetry 公共失败重试状态机

## P255（2026-07-21）
- Ops/Storage/Databases/Downsample 迁移 createActionRetry

## P256（2026-07-21）
- Account 改密/会话续期失败可重试 banner

## P257（2026-07-21）
- Audit 用户列表 / Readiness 版本加载失败可重试

## P258（2026-07-21）
- 命令面板补齐 readiness doctor/action、account prefs、metrics filter 深链

## P259（2026-07-21）
- Metrics 自动刷新失败保留快照并可重试

## P260（2026-07-21）
- 抽取 PartialErrorBanner，统一分项失败条

## P261（2026-07-21）
- Overview 自动刷新 soft-fail：保留快照 + `overview-refresh-error`

## P262（2026-07-21）
- Overview 自动/手动刷新 a11y：`aria-pressed` / `aria-busy` + testid

## P263（2026-07-21）
- 列表排序 `aria-sort` + Ops/About 刷新 `aria-busy`

## P264（2026-07-21）
- Databases 详情加载失败 soft-fail + `databases-detail-error`

## P265（2026-07-21）
- Storage 双列表刷新 soft-fail 与分项错误条

## P266（2026-07-21）
- Databases measurement 详情 soft-fail + 分项可重试

## P267（2026-07-21）
- Config error-codes/schema 分项 soft-fail，不阻断 effective

## P268（2026-07-21）
- Downsample 策略/状态分项 soft-fail

## P269（2026-07-21）
- Query 失败/取消保留上次结果快照并可重试

## P270（2026-07-21）
- Write 失败/取消保留上次成功结果并可重试

## P271（2026-07-21）
- Query 快照判断抽取 + Operations 统计 soft-keep

## P272（2026-07-21）
- Users/Grants 刷新 soft-keep 与分项错误

## P273（2026-07-21）
- 会话临界横幅展示剩余时间

## P274（2026-07-21）
- Account 会话时钟刷新 + Audit soft-keep

## P275（2026-07-21）
- Readiness Doctor/Version 刷新 soft-keep

## P276（2026-07-21）
- About / ApiSpec 刷新 soft-keep

## P277（2026-07-21）
- Audit/Write/Downsample 元数据与库列表 soft-keep

## P278（2026-07-21）
- 会话 critical 剩余时间与 write-submit title e2e 强化

## P279（2026-07-21）
- 登录/强改密错误区 retry/dismiss；本地校验不暴露 retry

## P280（2026-07-21）
- 强制改密页密码可见性切换

## P281（2026-07-21）
- 会话临界倒计时 1s 自适应刷新

## P282（2026-07-21）
- PasswordInputWithToggle 统一密码可见性；Account/Login/Force 接入

## P283（2026-07-21）
- Users 弹窗与 Config token 密码可见性统一

## P284（2026-07-21）
- 多标签会话内存重载 + TopBar 自适应时钟 + 离线横幅重新检测

## P285（2026-07-21）
- 标签页可见性恢复时同步会话与连通性

## P286（2026-07-21）
- 会话 warn 非阻断横幅与续期入口

## P287（2026-07-22）
- 导出失败一键重试（ExportJobBanner + useExportJob lastRetry）

## P288（2026-07-22）
- 写入/查询进行中状态与取消按钮可访问性

## P289（2026-07-22）
- Server：`GET /api/v1/auth/session` + gRPC `GetSession`（role/expires/must_change/remaining）
- Principal 扩展 Role/ExpiresAt；series 列表落实 offset/q
- Dashboard：visibility/storage 同步调用 session；Account 服务端校验；Ops 接入 storage-memory

## P290（2026-07-22）
- 筛选空态统一「清除筛选」CTA：Users / Metrics / ApiSpec / AccessMatrix / Downsample / Query history / Audit

## P291（2026-07-22）
- 命令面板无匹配：说明文案 + 清空搜索 / 跳转 Query / Overview

## P292（2026-07-22）
- Series 服务端分页：Query 加载更多 + 服务端筛选 q；Databases 加载更多；对齐 offset/q 契约

## P293（2026-07-22）
- 剩余筛选空态清除 CTA：Databases / AccessGrants / UserGrant / Ops / NotifyHistory / Sidebar

## P294（2026-07-22）
- Databases series：本地筛选 + 服务端 q + 空态清除；抽取 `seriesFilter` 可测工具

## P295（2026-07-22）
- Query 无结果：放宽时间 / 清 tags / 提高 limit / 重试引导

## P296（2026-07-22）
- Storage 数据快照空态创建；Write 空态跳转 TypedBatch；Readiness doctor 空态重试

## P297（2026-07-22）
- Account 展示服务端会话 remaining_seconds 与最近校验时间（refreshSession 回填）

## P298（2026-07-22）
- Server：`GET /api/v1/auth/session` / gRPC GetSession 增加 `server_time_unix`
- TopBar 会话 badge：接入服务端 `remaining_seconds` / 校验时间，title 与副文案提示偏差
- Overview：memory / compaction / maintenance 空态 EmptyState + 分区块刷新
- Query：stats 空态一键拉取引擎 stats；chart 展开无 rows 时 EmptyState + 执行/收起
- Config：schema / error-codes 筛选空态「清除筛选」与无数据文案区分

## P299（2026-07-22）
- Account：展示服务端时间 `server_time_unix` 与客户端时钟偏差
- Operations：维护/compaction/memory 空态增加「刷新该区块」CTA（复用 loadStats）

## P300（2026-07-22）
- Operations：stats/errors/memory 分项独立加载与重试（sectionRetrying），空态 CTA 不再整页 loadStats
- About：服务端版本空态 EmptyState + 重试
- Overview：健康 reasons 空态文案（无额外不健康原因）

## P301（2026-07-22）
- Overview：Doctor 空态常显 + 分项重试（含失败文案）
- Operations：分项失败 chip 条（对齐 Overview partial retry）

## P302（2026-07-22）
- Query/Write：进行中横幅（耗时、长耗时/近超时提示、取消）
- 查询/写入超时文案区分；QueryWorkbench/Write 记录 startedAt

## P303（2026-07-22）
- Query 范围删除：Abort + 进行中横幅 + 取消/超时文案；ConfirmDialog 支持加载中取消
- Readiness：Doctor/Version 失败 chip 可单独重试；Doctor 空态失败文案

## P304（2026-07-22）
- Storage/Ops 长操作：Abort + InFlightBanner + 取消/超时文案
- ConfirmDialog 运维危险操作支持加载中取消；抽取 `actionAbort` 工具

## P305（2026-07-22）
- Users：删除/批量启用禁用 Abort + InFlightBanner(admin) + 取消/超时文案；ConfirmDialog 加载中可取消
- Downsample：删除/批量/创建/范围修复 Abort + 横幅 + 取消/超时；range 弹窗加载中可取消
- i18n：`adminInFlightTitle` / `adminActionCancelled` / `adminActionTimedOut`

## P306（2026-07-22）
- Server：`POST /api/v1/users/batch-disabled` 与 `POST /api/v1/admin/downsample/policies/batch`（单项结果汇总 + gRPC）
- Dashboard：Users/Downsample 批量操作改为单次 batch API，保留 Abort + InFlightBanner

## P307（2026-07-22）
- Databases：创建/删除/RP 写路径 Abort + InFlightBanner(admin) + 取消/超时；删除 Confirm 加载中可取消
- Config：validate/reload Abort + 进行中横幅 + 取消/超时
- Users 单用户启用禁用、Downsample 单策略启停补齐 Abort 与横幅联动

## P308（2026-07-22）
- Downsample：run/reset Abort + 进行中横幅 + 失败可重试（DsActionKey 扩展）
- Users：创建/改密/授权/撤销补齐 Abort 与横幅
- 命令面板：`retry-last-action` 触发页内 ActionResultBanner 重试

## P309（2026-07-22）
- Account / ForceChangePassword：改密与会话续期 Abort + InFlightBanner + 取消/超时文案
- apiLogin / apiChangePassword / useAuth 支持 AbortSignal
- UserModals：loading 态禁用提交；InFlightBanner 取消按钮 a11y

## P310（2026-07-22）
- Login：Abort + InFlightBanner(login) + 取消/超时文案 + 双提交防护
- Write/Query 删除/Account/Users 写路径双提交 early-return
- InFlightBanner 支持 kind=login

## P311（2026-07-22）
- Operations：危险操作确认双提交防护（confirmLoading early-return）；入口按钮 loading 中禁用
- Operations：clear-log Confirm 对齐 writeBlocked / blockReason / 加载中可取消
- Server：flush/compact/retention 进程内互斥（resource_exhausted + ErrEngineBusy），防并发运维叠加
- 会话 critical 横幅文案明确写/运维/管理变更已阻断

## P312（2026-07-22）
- Server：`GET /api/v1/admin/stats/maintenance`（gRPC 同）增加 `admin_op_busy`，反映 flush/compact/retention 进程内互斥
- Operations：拉取 busy 状态；入口禁用 + chip + 确认/重试门禁；本地执行时乐观置 busy，结束后刷新

## P313（2026-07-22）
- Server：users/downsample batch 支持 `?stream=1` / Accept NDJSON 进度流（item/summary）；statusRecorder 透传 Flusher
- Dashboard：Users/Downsample 批量改为 NDJSON 流并显示 InFlight 百分比进度；防双提交
- Query：清除历史 Confirm 对齐 writeBlocked 门禁

## P314（2026-07-22）
- Users/Downsample：批量取消时若已有进度，展示部分结果摘要并刷新列表
- Server：admin heavy 互斥覆盖 flush/compact/retention + data-snapshot/restore-drill/config-snapshot（避免 reentry）
- Storage：拉取 `admin_op_busy`，重操作门禁/禁用/chip；API 错误码透传

## P315（2026-07-22）
- Server：批量 NDJSON 取消时推送 partial `summary`（`cancelled=true`），立即停止后续写
- Overview：维护统计区展示 `admin_op_busy` chip
- 命令面板：运维状态条入口 + busy 关键词；契约文案对齐管理重操作互斥

## P316（2026-07-22）
- Dashboard：`useAdminOpBusy` 全局静默轮询 `admin_op_busy`；Layout 管理员横幅（刷新/跳转运维）
- Readiness：评分 reasons 标记 `admin_op_busy`（不单独扣分）；评分卡 chip；导出预检 info 项

## P317（2026-07-22）
- Server：`GET /api/v1/admin/ops-status`（gRPC `OpsStatus`）轻量返回 `admin_op_busy`，供 Dashboard 高频轮询
- Dashboard：`useAdminOpBusy` 增加 `setAdminOpBusy` / `markAdminOpBusyAndRefresh`；轮询改走 ops-status
- Ops/Storage/Overview：去掉本地 busy ref 与重复 maintenance 拉取；乐观置位写共享态
- e2e：运维页空闲时确认无 `admin-op-busy-banner`

## P318（2026-07-22）
- Server：`tryBeginAdminHeavy(op)` 记录 `op` + `started_at_unix`；`ops-status`/`stats/maintenance` 同步返回
- Dashboard：共享态携带 kind/开始时间；Layout 横幅详情、Ops/Overview chip 展示当前操作类型

## P319（2026-07-22）
- Dashboard：admin busy 期间 Layout 1s tick 实时刷新 elapsed；抽取 formatAdminOpElapsed/joinAdminOpChip
- Storage/Readiness/Ops/Overview chip 展示当前 op 类型；命令面板运维状态项 busy 时动态标签

## P320（2026-07-22）
- Dashboard：admin busy 时 ops-status 轮询加速至 ~3s（空闲 15s）
- Ops 状态条 chip / 命令面板展示实时 elapsed（复用 Layout tick 注入摘要）

## P321（2026-07-22）
- Dashboard：admin ops-status 轮询失败软提示（Layout 横幅/独立条）；Storage/Readiness chip 展示 elapsed
- 就绪导出预检：busy 时文案带当前 op 类型

## P322（2026-07-22）
- Overview 维护区 admin busy chip 对齐 elapsed/detail（注入 Layout 摘要）
- e2e：空闲时确认无 `admin-op-busy-poll-error-banner`

## P323（2026-07-22）
- Operations：状态条「刷新占用状态」快捷钮 + busy 提示文案（不拉全量 stats）
- Overview：管理统计失败且 admin busy 时展示关联提示 + 刷新占用状态

## P324（2026-07-22）
- 命令面板：管理员动作「刷新管理占用状态」
- Storage：busy chip 旁快捷刷新 ops-status

## P325（2026-07-22）
- Readiness：admin busy chip 旁「刷新状态」；空闲 e2e 断言无 chip
- 命令面板：可检索「刷新占用」管理员动作；非 admin 不可见（单测）

## P326（2026-07-22）
- Dashboard：`resource_exhausted` + admin heavy 文案映射为「管理重操作占用中」
- Ops/Storage：捕获互斥错误时乐观置 busy 并刷新 ops-status

## P327（2026-07-22）
- Server：admin heavy 互斥冲突 message 附带当前 `op`
- Dashboard：错误文案/乐观 busy 解析并展示占用 op

## P328（2026-07-22）
- Notify：toast 支持可选 action（label+path）；合并键含 action.path
- NotifyHost：展示 `notify-action`，点击 router.push 并 dismiss
- adminOpBusy：`ADMIN_OP_BUSY_OPS_PATH` / `adminOpBusyOpenAction`
- Ops/Storage：本地 busy 门禁与互斥错误 toast 附带「打开运维」跳转 `/operations#ops-status-strip`

## P329（2026-07-22）
- Server：互斥错误响应结构化 `admin_op_busy` + `op`（`newAdminHeavyBusyError`）
- Dashboard：`APIClientError` 透传字段；busy 识别/文案优先结构化 op；兼容 message 解析

## P330（2026-07-22）
- Server：互斥错误写 `X-MTS-Admin-Op-Busy` / `X-MTS-Admin-Op` 响应头
- Dashboard：`parseAdminBusyFromHeaders` + `readAPIError` 头/体双通道解析

## P331（2026-07-22）
- Server：gRPC `grpcError(ctx,err)` 对 busy 写 header/trailer metadata（`x-mts-admin-op-busy` / `x-mts-admin-op`）

## P332（2026-07-22）
- Overview：刷新失败若 admin busy，toast 附带打开运维 action，并乐观置 busy
- adminOpBusy：`buildAdminBusyNotifyOptions` 统一 action 构造

## P333（2026-07-22）
- Databases/Users/Downsample/Config：写路径错误若 admin busy，toast 附带打开运维并乐观置 busy

## P334（2026-07-22）
- 通知历史保留 actionLabel/actionPath；面板可跳转；导出 JSON/CSV 含 action 字段

## P335（2026-07-22）
- ops-status 轮询失败指数退避（5s→30s）；Layout 横幅展示连续失败与下次重试间隔

## P336（2026-07-22）
- Write/Query/Audit/Metrics/AccessGrants：API 错误若 admin busy，toast 附带打开运维并乐观置 busy

## P337（2026-07-22）
- adminOpBusy：抽取 `adminOpPollIntervalMs` / `nextAdminOpFailStreak` 纯函数；单测覆盖 idle/busy/退避

## P338（2026-07-22）
- e2e：通知历史预置 action 条目，点击跳转 `/operations#ops-status-strip`

## P339（2026-07-22）
- 抽取 `resolveAdminBusyNotify` + `useNotifyAdminBusy`；全站管理页复用 busy toast/action

## P340（2026-07-22）
- 单测覆盖 resolveAdminBusyNotify（busy/local/plain）

## P341（2026-07-22）
- Readiness：version/doctor/导出失败走 `notifyMaybeAdminBusy`（soft-keep 也支持 treatLocalBusy）

## P342（2026-07-22）
- 命令面板：刷新占用状态 catch 统一 busy toast；重试失败在本地 busy 时带运维跳转

## P343（2026-07-22）
- `commandAdminOpRefreshFeedback` 纯函数 + 单测


## P344（2026-07-22）
- Server：`finishAdminHeavy` 记录最近一次管理重操作结果（op/ok/error/duration）

## P345（2026-07-22）
- Server：`ops-status` 与 `stats/maintenance` 响应增加 `last` 字段

## P346（2026-07-22）
- Dashboard：解析/展示最近一次运维结果（运维状态条）

## P347（2026-07-22）
- About/ApiSpec：加载失败走 `notifyMaybeAdminBusy` 与 soft-keep 联动

## P348（2026-07-22）
- Server：admin heavy `duration_ms` 使用毫秒时钟，短操作不再被量化为 0s
- Dashboard：最近一次耗时展示 ms/小数秒；Layout 注入 lastSummary

## P349（2026-07-22）
- Overview：空闲时展示最近一次管理重操作摘要芯片（复用 Layout lastSummary）

## P350（2026-07-22）
- Layout：空闲且有 last 时展示全局最近管理重操作条（可跳转运维状态条）

## P351（2026-07-22）
- e2e：flush 完成后校验 ops-status-last 与 admin-op-last-banner

## P352（2026-07-22）
- Layout：最近管理重操作条可关闭（按 finished_at 记忆，新操作再出现）

## P353（2026-07-22）
- Storage：空闲时展示 last 摘要条

## P354（2026-07-22）
- e2e：flush 后 dismiss 全局 last 条；ops-status-last 仍可见

## P355（2026-07-22）
- Operations：最近一次结果 ok/fail 色调区分（data-ok）

## P356（2026-07-22）
- 命令面板：运维状态支持「最近一次/last op」关键词检索

## P357（2026-07-22）
- Layout：最近管理重操作条按 ok/fail 换肤（红/绿），并 provide lastOk

## P358（2026-07-22）
- Overview/Storage：last 芯片/条带 ok/fail 色调对齐

## P359（2026-07-22）
- Config/Downsample/Readiness：标题区展示 last 摘要芯片

## P360（2026-07-22）
- Databases/Users：标题区展示 last 摘要芯片（admin）

## P361（2026-07-22）
- 命令面板：关闭最近管理重操作条动作 + 纯函数反馈

## P362（2026-07-22）
- e2e：flush 后 Databases/Users 可见 last 芯片

## P363（2026-07-22）
- Metrics/Audit：admin last 摘要芯片

## P364（2026-07-22）
- 失败 last：需打开运维确认（ack）后才可关闭；命令面板 require_ack

## P365（2026-07-22）
- 进入 Operations 自动 ack 失败 last；e2e 覆盖 Metrics/Audit last 芯片

## P366（2026-07-22）
- About/ApiSpec：标题 desc 行展示 admin last 摘要芯片（不入 h1，避免 a11y 名称污染）

## P367（2026-07-22）
- Layout：失败 last 横幅展开 `error` 详情（`admin-op-last-error`）；provide `lastError`
- 纯函数：`formatAdminHeavyLastDetail` / `formatAdminHeavyLastCopyText` + 单测

## P368（2026-07-22）
- Operations：运维状态条「复制最近一次」按钮（摘要 + 错误详情）
- e2e：About/ApiSpec last 芯片 + ops-status-last-copy
- Server：补齐 `finishAdminHeavy` 失败路径 `last.error` 单测（协议字段已有）

## P369（2026-07-22）
- Access 矩阵/授权页：admin last 摘要芯片（desc 行）

## P370（2026-07-22）
- 命令面板：复制最近管理重操作结果动作 + 纯函数反馈 + 单测

## P371（2026-07-22）
- e2e：Access last 芯片覆盖

## P372（2026-07-22）
- Layout：最近管理重操作条支持一键复制（摘要+错误）

## P373（2026-07-22）
- Account：admin last 摘要芯片（desc 行）

## P374（2026-07-22）
- Server HTTP：失败 heavy 后 `ops-status`/`stats/maintenance` 暴露 `last.error`

## P375（2026-07-22）
- e2e：横幅复制、Account last、命令面板复制最近一次

## P376（2026-07-22）
- Query/Write：admin last 摘要芯片（工作台可见最近运维结果）

## P377（2026-07-22）
- e2e：Query/Write last 芯片

## P378（2026-07-22）
- Server：`maintenance/errors` 响应附带 `last`（HTTP/gRPC 对齐），便于运维页单请求关联最近重操作结果

## P379（2026-07-22）
- Query/Write：admin busy 芯片（占用中优先于 last）

## P380（2026-07-22）
- Server：`maintenance/errors` 附带 `admin_op_busy/op/started_at_unix/last`（HTTP/gRPC）

## P381（2026-07-22）
- ActionResultBanner 支持 action 跳转；Write/Query 命中 admin busy 时结果条一键打开运维

## P382（2026-07-22）
- Overview：失败 last 展示 `lastError` 明细行

## P383（2026-07-22）
- e2e：mock 失败 last（横幅 error + dismiss/ack）；mock 写入 busy 结果条跳转运维

## P384（2026-07-22）
- PartialErrorBanner：message 命中 admin busy 时自动提供运维跳转

## P385（2026-07-22）
- Storage：失败 last 展示 error 明细；e2e 覆盖

## P386（2026-07-22）
- Readiness 评分纳入 `admin_op_last_failed`（总分 −5 + reasons）；`formatReadinessReason(s)` 本地化扣分原因；失败 last error 明细行

## P387（2026-07-22）
- Config 页 ActionResultBanner 绑定 busy 运维跳转（`configAdminBusyAction`）

## P388（2026-07-22）
- Server `GET /api/v1/admin/doctor` 附带 `admin_op_busy`/`op`/`started_at_unix`/`last`，与 maintenance/errors 对齐；Readiness/Overview 加载 doctor 时 `applyAdminOpStatus`；HTTP 单测 busy/last

## P389（2026-07-22）
- Storage / Operations / Downsample：ActionResultBanner 命中 admin busy 时提供运维跳转（与 Query/Write/Config 对齐）

## P390（2026-07-22）
- 共享 DTO：`MaintenanceErrorsResponse` / `DoctorResponse` 写入 `api/types.ts`；Overview `maintErrors` 加载时 `applyAdminOpStatus`

## P391（2026-07-22）
- Readiness 根节点 `data-testid="readiness-page"`；e2e 失败 last 场景 mock doctor 防覆盖

## P392（2026-07-22）
- Databases / Users：ActionResultBanner 命中 admin busy 时提供运维跳转（与运维类页对齐）

## P393（2026-07-22）
- 全站页头失败 last 错误明细：About/Access/Account/ApiSpec/Audit/Config/Databases/Downsample/Metrics/Query/Users/Write + Operations `ops-status-last-error`
- 共享组件 `AdminOpLastChip.vue`（可复用芯片+错误明细）

## P394（2026-07-22）
- Server `GET /api/v1/admin/health`（HTTP/gRPC）附带 `admin_op_busy/op/started_at_unix/last`；Overview 加载时 `applyAdminOpStatus` + 完整 applyHealth；HTTP 单测 busy/last

## P395（2026-07-22）
- 全站 last 芯片收敛到 `AdminOpLastChip`（Config/Query/Write/Users/Databases/Downsample/Metrics/Account/ApiSpec/Audit/Access/Readiness/Overview；Storage 保留卡片式 last 条）

## P396（2026-07-22）
- operation_registry / api-spec 描述对齐：ops_status、admin_doctor、admin_health、maintenance stats/errors 明示 busy/last 字段

## P397（2026-07-22）
- `AdminOpLastChip` 默认可点击跳转 `/operations#ops-status-strip`；Storage 失败/成功 last 卡片同步可跳转；e2e 覆盖 Overview last → 运维

## P398（2026-07-22）
- Storage last 卡片：复制最近一次结果按钮（`storage-admin-last-copy`，点击不触发跳转）

## P399（2026-07-22）
- Server `GET /api/v1/admin/version` 附带 `admin_op_busy/op/started_at_unix/last`；Overview/Readiness/About 加载时 `applyAdminOpStatus`；HTTP 单测 busy/last

## P400（2026-07-22）
- Server `stats/compaction` 与 `stats/storage-memory`（HTTP/gRPC）附带 `admin_op_busy/op/started_at_unix/last`；Overview/Operations 加载时 `applyAdminOpStatus`；HTTP 单测 busy/last；e2e fail-last 场景同步 mock

## P401（2026-07-22）
- `AdminOpLastChip` 支持 `showCopy`；Overview/Readiness/Config/Query/Write/About 开启页内复制最近一次；e2e 覆盖 overview 复制钮

## P402（2026-07-22）
- 剩余页 `AdminOpLastChip` 开启 `showCopy`（Users/Databases/Downsample/Metrics/Account/Audit/ApiSpec/Access/）
- e2e 覆盖各页 `*-admin-last-copy` 可见

## P403（2026-07-22）
- Server：`storage/snapshots`、`storage/data-snapshots`、`storage/export`（HTTP/gRPC）附带 `admin_op_busy/op/started_at_unix/last`
- Dashboard Storage 列表/导出加载时 `applyAdminOpStatus`；api-spec 描述对齐；HTTP 单测 busy/last；e2e fail-last mock 同步

## P404（2026-07-22）
- Server：config/effective|schema、api-spec、error-codes、audit 列表附带 `admin_op_busy/op/started_at_unix/last`（HTTP/gRPC）
- Dashboard Config/ApiSpec/Audit 加载时 `applyAdminOpStatus`；types/registry 描述对齐
- `AdminOpLastChip` a11y：芯片/复制钮/错误明细 `aria-label` + 错误 `role=status`
- e2e fail-last mock 同步；overview 复制钮与芯片 aria 断言

## P405（2026-07-22）
- Server：downsample policies/statuses 列表附带 `admin_op_busy/op/started_at_unix/last`（HTTP/gRPC）
- Dashboard Downsample 加载时 `applyAdminOpStatus`；Metrics 拉取 `/metrics` 后联动 `refreshAdminOpBusy`
- types/registry 描述对齐；HTTP 单测；e2e fail-last mock 同步

## P406（2026-07-22）
- Server：users 列表、admin/data databases 列表、retention-policies 列表附带 `admin_op_busy/op/started_at_unix/last`（HTTP/gRPC 对齐）
- Dashboard Users/Databases/AccessGrants/Audit 加载时 `applyAdminOpStatus`；`meta.listDatabasesDetailed`/`listRetentionPoliciesDetailed` 透传 adminOp
- types/registry 描述对齐；HTTP 单测；e2e fail-last mock 同步

## P407（2026-07-22）
- Server：database-permissions、fields、series、measurements 列表附带 `admin_op_busy/op/started_at_unix/last`（HTTP/gRPC）
- Dashboard Users/AccessGrants/Databases 加载权限与 meta 时 `applyAdminOpStatus`；AccessMatrix 进入时 `refreshAdminOpBusy`
- meta 透传 adminOp（measurements/fields/series）；types/registry/单测同步

## P408（2026-07-22）
- Server：`GET /api/v1/auth/session`、`POST /api/v1/admin/storage/validate`、`GET /api/v1/data/query/stats` 附带 `admin_op_busy/op/started_at_unix/last`（HTTP/gRPC）
- Dashboard：session 刷新（`refreshSession`/`apiGetSession`）、Storage validate、Query engine stats 加载时 `applyAdminOpStatus`
- types/registry/单测同步；e2e fail-last mock 覆盖 session/validate/query-stats
- busy/last 契约覆盖矩阵：见本节「覆盖矩阵」

### busy/last 覆盖矩阵（P408）
| 接口族 | HTTP | gRPC | Dashboard apply | 备注 |
|--------|------|------|-----------------|------|
| ops-status / maintenance stats | 是 | 是 | 是 | 轮询主源 |
| doctor / health / version | 是 | 是 | 是 | Overview/Readiness/About |
| compaction / storage-memory / maintenance errors | 是 | 是 | 是 | Overview/Operations |
| storage snapshots/export | 是 | 是 | 是 | Storage |
| config / api-spec / error-codes / audit | 是 | 是 | 是 | Config/ApiSpec/Audit |
| downsample policies/statuses | 是 | 是 | 是 | Downsample；Metrics 联动 refresh |
| users / databases / RP | 是 | 是 | 是 | Users/Databases/Access |
| permissions / fields / series / measurements | 是 | 是 | 是 | Access/Databases meta |
| **session** | 是 | 是 | 是 | Account 校验 / 全局 refreshSession |
| **storage/validate** | 是 | 是 | 是 | Storage 校验按钮 |
| **query/stats** | 是 | 是 | 是 | Query 引擎 stats |
| 写路径/重操作启动本身 | 互斥错误体 | 互斥错误体 | 是 | busy 即时 chip |
| SQL/expr 树等 | — | — | — | 非目标 |

## P409（2026-07-22）
- Server：flush/compact、storage snapshot / data-snapshot / restore-drill 成功响应附带 `admin_op_busy/op/started_at_unix/last`（HTTP；snapshot gRPC 对齐）
- Dashboard：Operations 执行 flush/compact 后即时 `applyAdminOpStatus`；Storage 执行 snapshot/data-snapshot/restore-drill 后即时 apply
- types/registry 描述对齐；HTTP 单测；覆盖矩阵更新

### busy/last 覆盖矩阵（P409 增量）
| 接口族 | HTTP | gRPC | Dashboard apply | 备注 |
|--------|------|------|-----------------|------|
| flush / compact | 是 | 是 | 是 | 成功响应 last=本次结果 |
| storage snapshot | 是 | 是 | 是 | config_snapshot |
| storage data-snapshot | 是 | HTTP 主路径 | 是 | data_snapshot |
| storage restore-drill | 是 | HTTP 主路径 | 是 | restore_drill |
| retention/apply | ok 体（无 last） | 有 | loadStats 回读 | 仍走 finally 刷新 |

## P410（2026-07-22）
- Server：`retention/apply`、`config/reload`、downsample run/range/repair/dry-run 与 enable/disable/reset 的 `ok` 响应附带 `admin_op_busy/op/started_at_unix/last`（HTTP/gRPC 对齐）
- Dashboard：Operations retention、Config reload、Downsample run/range/dry-run 成功后即时 `applyAdminOpStatus`
- types/registry 描述对齐；HTTP 单测；覆盖矩阵增量

### busy/last 覆盖矩阵（P410 增量）
| 接口族 | HTTP | gRPC | Dashboard apply | 备注 |
|--------|------|------|-----------------|------|
| retention/apply | 是 | 是 | 是 | last=本次 retention |
| config/reload | 是 | 是 | 是 | 热加载字段列表 + busy/last |
| downsample run/range/repair | 是 | 是 | 是 | 运行结果 + busy/last |
| downsample dry-run | 是 | 是 | 是 | 预览不写库 |
| downsample enable/disable/reset | 是 | 部分 | loadData 回读 | ok 体带 last |

## P411（2026-07-22）
- Server：统一剩余 admin mutation 的 `ok` 响应附带 `admin_op_busy/op/started_at_unix/last`（users/databases/RP/downsample create-drop/permissions/delete 等 HTTP + gRPC）
- Dashboard：Users / Databases / Downsample 写操作成功后即时 `applyAdminOpStatus`
- HTTP 单测覆盖 create/drop database busy/last；全量 mts-server 测试通过

### busy/last 覆盖矩阵（P411 增量）
| 接口族 | HTTP | gRPC | Dashboard apply | 备注 |
|--------|------|------|-----------------|------|
| users create/update/delete/password | 是 | 是 | 是 | ok 体 |
| databases create/drop + RP create | 是 | 是 | 是 | ok 体 |
| downsample create/drop/enable/disable/reset | 是 | 是 | 是 | ok 体 |
| data delete | 是 | 是 | 列表/查询侧回读 | ok 体 |
| auth password/logout 等 | 是 | 是 | 会话刷新 | ok 体 |

## P412（2026-07-22）
- Server：`data/write`、`write/typed`、`write/points-typed` 成功响应附带 `admin_op_busy/op/started_at_unix/last`（HTTP/gRPC）
- Dashboard：Write 页写入成功、Query 页 delete 成功后即时 `applyGlobalAdminOpStatus`
- registry 描述对齐；HTTP 单测；全量 mts-server 测试通过

### busy/last 覆盖矩阵（P412 增量）
| 接口族 | HTTP | gRPC | Dashboard apply | 备注 |
|--------|------|------|-----------------|------|
| data write / typed / points-typed | 是 | 是 | 是 | 写路径成功响应 |
| data delete | 是 | 是 | 是 | Query 删除成功即时 apply |

## P413（2026-07-22）
- Server：`POST /api/v1/admin/config/validate` 成功/失败体附带 `admin_op_busy/op/started_at_unix/last`（HTTP/gRPC）
- Dashboard：Config validate、Write 加载 databases、Query 加载 databases/measurements/RP 时 `applyAdminOpStatus`
- Write/Query `AdminOpLastChip` 属性缩进整理；e2e fail-last 覆盖 write/query 页 last 芯片 + write/delete/config validate mock
- HTTP 单测；全量 mts-server / npm test / commercial-smoke 通过

### busy/last 覆盖矩阵（P413 增量）
| 接口族 | HTTP | gRPC | Dashboard apply | 备注 |
|--------|------|------|-----------------|------|
| config/validate | 是 | 是 | 是 | 校验结果 + busy/last |
| write/query meta 加载 | 是 | — | 是 | databases/measurements/RP |
| e2e fail-last write/query | mock | — | 是 | 防真实 last 覆盖失败场景 |

## P414（2026-07-22）
- Dashboard：Write 加载 RP/measurements/fields、Query 加载 fields/series 时透传 `adminOp` 并 `applyGlobalAdminOpStatus`
- 统一 measurement meta 读取为 detailed API，补齐写路径/查询路径 busy/last 可见性
- commercial-smoke 超时提升到 180s（覆盖 fail-last write/query 新增断言）
- npm test / build / commercial-smoke 通过

### busy/last 覆盖矩阵（P414 增量）
| 接口族 | HTTP | gRPC | Dashboard apply | 备注 |
|--------|------|------|-----------------|------|
| write RP/measurements/fields | 是 | — | 是 | Write 元数据加载 |
| query fields/series | 是 | — | 是 | Query 元数据/序列列表 |

## P415（2026-07-22）
- Server：users/downsample batch（非流式 JSON + 流式 summary + gRPC）成功响应附带 `admin_op_busy/op/started_at_unix/last`
- Dashboard：Users grant/revoke 成功后即时 `applyAdminOpStatus`；Users/Downsample batch summary 透传并 apply busy/last
- batchProgress 解析 summary 的 adminOp 字段；e2e fail-last 覆盖 users last 芯片与 permissions/batch mock
- HTTP batch 单测字段可反序列化；npm test / commercial-smoke 验证

### busy/last 覆盖矩阵（P415 增量）
| 接口族 | HTTP | gRPC | Dashboard apply | 备注 |
|--------|------|------|-----------------|------|
| users grant/revoke database-permissions | 是（P411） | 是（P411） | 是 | 批量授权 apply 末次成功响应 |
| users batch-disabled | 是 | 是 | 是 | 非流式体 + 流式 summary |
| downsample policies batch | 是 | 是 | 是 | 非流式体 + 流式 summary |

## P416（2026-07-22）
- Dashboard：Storage 删除 config 快照成功后即时 `applyAdminOpStatus`（服务端 ok 已 attach）
- e2e fail-last：DELETE `/admin/storage/snapshots` mock 防止真实 last 覆盖

### busy/last 覆盖矩阵（P416 增量）
| 接口族 | HTTP | gRPC | Dashboard apply | 备注 |
|--------|------|------|-----------------|------|
| storage snapshots DELETE | 是（既有） | 是（既有） | 是 | 删快照成功响应 |

## P417（2026-07-22）
- e2e fail-last：扩展 Config/Downsample/Databases/Audit/AccessMatrix/AccessGrants/Metrics/About/ApiSpec/Account 页 last 芯片断言（与列表/配置加载 apply 或 refreshAdminOpBusy 对齐）
- e2e global-setup 每次清理 data_dir，避免强制改密残留导致 bootstrap admin/admin 登录失败
- 抽取 `expectFailAdminLast` 辅助，降低 commercial-smoke 重复
- Users/Downsample batch 取消 summary 也透传 apply busy/last（若服务端 summary 附带）

### busy/last 覆盖矩阵（P417 增量）
| 接口族 | HTTP | gRPC | Dashboard apply | 备注 |
|--------|------|------|-----------------|------|
| fail-last 管理页芯片 | mock | — | 是 | 验收可见性 |
| batch cancelled summary | 是（P415） | — | 是 | 取消路径不丢 last |

## P418（2026-07-22）
- e2e fail-last：路由统一 `**` 匹配（含 query），补齐 `database-permissions` 列表 mock，降低真实 last 覆盖失败场景
- Dashboard：SPA 内 `/metrics` 兼容重定向到 `/observability/metrics`（浏览器全页 `/metrics` 仍为 Prometheus 抓取，不进 SPA）
- e2e：指标页 `#metrics-filter` 深链验收（UI 路由 `/observability/metrics`）
- 最近访问条展示上限与 `RECENT_ROUTES_MAX` 对齐，避免 fail-last 深路径挤掉 pin 目标

### busy/last 覆盖矩阵（P418 增量）
| 接口族 | HTTP | gRPC | Dashboard apply | 备注 |
|--------|------|------|-----------------|------|
| fail-last route ** | mock | — | 是 | 防 query 漏 mock |
| users database-permissions GET | mock | — | 是 | AccessGrants 并发拉取 |
| SPA `/metrics` → observability | — | — | — | 仅客户端路由；抓取路径不变 |

## P419（2026-07-22）
- 生产清单：新增 `admin-op-visibility`（busy/last 可观测，自动化覆盖）
- 生产 Runbook：日常运维/应急补充管理重操作 busy/last 排障口径
- e2e：写入模式深链 `/write#write-mode-{form,line,prometheus,typed}` 切换验收；snapshots DELETE mock 注释澄清
- 锚点审计：命令面板 52 条深链与页面 id/testid 对齐（write-mode-* 为动态 id）

### busy/last 覆盖矩阵（P419 增量）
| 接口族 | HTTP | gRPC | Dashboard apply | 备注 |
|--------|------|------|-----------------|------|
| 清单/Runbook 可观测说明 | — | — | — | 运维文档对齐 |
| write mode 深链 | — | — | — | 命令面板一致性 |

## P420（2026-07-22）
- Server：改密旧密码错误映射 `bad_request`（不再 `unauthenticated`），避免客户端误清仍有效会话；成功响应 `changePasswordResponse` 含 `must_change_password=false` 与 admin busy/last
- Dashboard：`/api/v1/auth/password` 失败不触发会话清理；改密成功清脏表单再跳转登录；`invalid credentials` 友好文案
- 就绪中心：生产清单项补充 `jump` 与「打开相关页面」；`admin-op-visibility` 跳转运维 `#ops-status-strip`
- 命令面板：生产清单 / admin-op 可见性深链
- e2e：账户改密错误保会话、就绪清单跳转、写入 admin busy 429 文案

### busy/last 与会话边界（P420 增量）
| 接口/场景 | HTTP | gRPC | Dashboard | 备注 |
|-----------|------|------|-----------|------|
| auth/password 旧密码错误 | bad_request | bad_request | 不 clearAuth | 会话仍有效 |
| auth/password 成功 | ok+must_change=false+adminOp | 同左 | clearAuth 后登录 | 表单清脏 |
| readiness prod jump admin-op | — | — | 是 | → /operations#ops-status-strip |
| write admin busy 429 | 既有 | 既有 | e2e mock | 友好文案 |

## P421（2026-07-22）
- Dashboard Users：自改密改走 `useAuth.changePassword`，成功后 clearAuth 并跳转登录（修复成功改密仍停留已撤销会话的缺陷）
- Dashboard Users：管理员 `set_password` 若目标为当前用户，同步 logout 并要求重登
- 登录页：支持 `?user=` 预填；`password_changed` 自动聚焦密码框
- Account / ForceChange / Users 改密成功跳转携带 `user` query
- 命令面板：`reset-nav-order` 重置侧栏排序（派发 prefs 变更事件）
- Server：`set_user_password` 契约描述对齐（撤销 token / 清 must_change / adminOp）
- e2e：强制改密旧密码错误保会话、登录预填、命令面板重置排序入口

### 会话边界（P421 增量）
| 场景 | 服务端 | Dashboard | 备注 |
|------|--------|-----------|------|
| Users 自改密成功 | 撤销 token | clearAuth → Login | 原先未清会话 |
| Users 设密当前用户 | 撤销 token | logout → Login | 与 SetPassword 语义一致 |
| 强制改密旧密码错误 | bad_request | 留在 force-change | 与 P420 对齐 |
| 登录 password_changed | — | 预填 user + 焦点密码 | 减少重登摩擦 |

## P422（2026-07-22）
- Server：`validateUserPassword`（≥8 且禁止 `admin`）接入 SetPassword / ChangePassword / CreateUser 初始密码（HTTP/gRPC）
- Dashboard：`validateAssignedPassword` + Users 设密确认框 / 自改密确认框 + PasswordHints（assigned/change）
- Users 创建用户可选密码走同一强度策略；弱密码前端拦截
- e2e：账户会话续期成功、Users 设密弱密码拦截
- 单测：passwordPolicy / passwordHints / server validateUserPassword

### 密码策略对齐（P422）
| 路径 | 最小长度 | 禁止默认 admin | 确认框 | 备注 |
|------|----------|----------------|--------|------|
| auth/password | 8 | 是 | Account/Force 有；Users 现有 | bad_request |
| users/:name/password | 8 | 是 | 是 | 撤销 token |
| users create password | 8（若填写） | 是 | — | 空密码仍允许 |

## P423（2026-07-22）
- Dashboard：登录/续期 `invalid credentials` 友好文案（密码不正确；续期时会话仍有效）
- Account：续期失败清空密码输入，便于重试
- e2e：会话续期错误密码保会话 → 再成功续期；Users 自改密/创建用户弱密码拦截

### 会话续期边界（P423 增量）
| 场景 | 服务端 | Dashboard | 备注 |
|------|--------|-----------|------|
| renew 密码错误 | unauthenticated | 留在账户页 | 不清会话 |
| renew 成功 | 新 token | 更新会话 | 密码输入清空 |
| Users 自改密弱密码 | —（前端） | action-result 错误 | 不提交 |
| create 弱密码 | —（前端） | action-result 错误 | 不提交 |

## P424（2026-07-22）
- e2e：登录错误密码文案断言（密码不正确 / Incorrect password / 用户名或密码）
- e2e：Users 对非当前用户 `reader-e2e` 设密成功，保留 admin 会话于 `/users`
- Dashboard：命令面板补齐 section 级重置（workspace/access/admin/system）与侧栏 `sidebar-order-reset-*` 对齐
- Server：`set_user_password` 成功响应 `setPasswordResponse{user_name, admin_op_*}`（HTTP/gRPC）
- Dashboard Users：设密成功优先使用服务端 `user_name` 判定是否踢当前会话

### 会话/导航边界（P424 增量）
| 场景 | 服务端 | Dashboard | 备注 |
|------|--------|-----------|------|
| login 密码错误文案 | unauthenticated+invalid credentials | 密码不正确（友好文案） | e2e 断言 |
| 设密非当前用户 | 撤销目标 token；返回 user_name | action-result 成功；留 /users | reader-e2e |
| 设密当前用户 | 撤销 token | logout → Login | 既有 P421 |
| 命令面板 section 重置 | — | reset-nav-section-order:* | 与侧栏按钮一致 |

## P425（2026-07-22）
- Dashboard：`friendlyApiError` 映射服务端密码策略（admin 禁用 / ≥8 / required）与 `user not found` 友好文案（不中英混杂）
- Server：`set_user_password` 用户不存在统一 `not_found` + `user not found`（HTTP/gRPC）
- e2e：Users 设密确认不一致拦截；命令面板英文 `reset workspace` section 重置
- 单测：apiError 密码策略与用户不存在映射

### 错误文案与设密边界（P425 增量）
| 场景 | 服务端 | Dashboard | 备注 |
|------|--------|-----------|------|
| 密码策略 bad_request | admin / ≥8 / required 英文 message | 中文/英文友好映射 | 绕过前端时可用 |
| 设密目标不存在 | not_found + user not found | 用户不存在 | HTTP/gRPC 一致 |
| 设密确认不一致 | —（前端） | action-result 错误 | e2e |
| 命令面板 section 重置英文检索 | — | keywords + locale label | e2e |

## P426（2026-07-22）
- 内核：`UpdateUser` 在用户从启用变为禁用时主动撤销全部 token（对齐 SetPassword 会话边界）
- Dashboard Users：状态筛选 `active|disabled` + 深链 `?status=` + 分享链接/清空筛选/命令面板入口
- API Spec：`apiEndpoint.response` 可选字段；`set_user_password` 契约标注 setPasswordResponse；页面展示 response 行
- e2e：状态深链、禁用用户登录失败友好文案、再启用后继续 reader 授权路径；命令面板跳转 disabled 筛选

### 会话/筛选边界（P426 增量）
| 场景 | 服务端 | Dashboard | 备注 |
|------|--------|-----------|------|
| 禁用用户 | 撤销 token；登录仍 invalid credentials | 密码不正确（不泄露账户是否禁用） | 安全口径 |
| 用户状态筛选 | — | status 筛选+深链 | 与 role/q 组合 |
| 设密契约 response | ResponseHint | API Spec 展示 | 对齐 P424 响应 |

## P427（2026-07-22）
- Server：全部 HTTP 登记操作补齐 `ResponseHint`，API Spec `endpoints[].response` 可浏览（flush/compact/write/batch-disabled 等）
- Dashboard：批量启用/禁用确认文案标明 token 撤销 / 需重登
- 就绪清单：新增 `user-disable-revokes-tokens`（跳转 Users `?status=disabled`）
- Runbook：补充「禁用用户与会话边界」排障口径
- e2e：API Spec response 检索；就绪清单跳转禁用用户筛选

### 契约/会话边界（P427 增量）
| 场景 | 服务端 | Dashboard | 备注 |
|------|--------|-----------|------|
| API Spec response | ResponseHint on HTTP ops | 行内 response + 搜索 | 对齐运维写接口 |
| 批量禁用 | 撤 token（内核） | 确认文案提示 | i18n |
| 就绪清单禁用会话 | — | jump → /users?status=disabled | automated |

## P428（2026-07-22）
- 命令面板：`readiness-user-disable-revokes-tokens` 深链 Users 禁用筛选；清单关键词对齐
- API Spec 导出：Markdown 含 `response=` / `响应=` 单测
- e2e：命令面板禁用会话清单入口；Users 对 `reader-e2e` 真实批量禁用/启用（确认文案含 token 撤销）

### 清单/批量边界（P428 增量）
| 场景 | 服务端 | Dashboard | 备注 |
|------|--------|-----------|------|
| 命令面板禁用会话 | — | → /users?status=disabled | 与就绪清单一致 |
| API Spec 导出 response | ResponseHint | md 导出可见 | 单测 |
| 批量禁用 reader | 撤 token | 确认框文案+状态筛选可见 | 真实 e2e |

## P429（2026-07-22）
- 服务端：批量用户禁用/启用写入轻量 `last`（`batch_user_disable` / `batch_user_enable`，不占 heavy 互斥）
- Dashboard：批量禁用后 Users `users-admin-last` 芯片可见/可复制 e2e
- 命令面板：`users-status-active` 关键词扩展 + `readiness-user-enable-active` 对称入口；e2e 点选 active 筛选
- API Spec：抽出 `apiSpecFilter` 跨 ns 搜索纯函数并单测；ApiSpecPage 复用

### 前后端对齐（P429）
| 场景 | 服务端 | Dashboard | 备注 |
|------|--------|-----------|------|
| 批量禁用 last | `recordAdminOpLast` | applyAdminOpStatus + 芯片 | stream summary 附 last |
| 启用用户命令 | — | status=active 深链 | 与 disabled 对称 |
| API Spec 搜索 | ResponseHint | 跨 ns fallback 单测 | 默认首 ns 不漏契约 |

## P430（2026-07-22）
- 服务端：降采样批量启停写入轻量 last（`batch_downsample_enable` / `batch_downsample_disable`）；gRPC 批量用户/降采样同步 record
- Dashboard：批量 op 文案 i18n；就绪清单 `batch-admin-last`；命令面板就绪入口
- e2e：批量启用后 last 芯片；命令面板「批量 last」深链 Users

### 前后端对齐（P430）
| 场景 | 服务端 | Dashboard | 备注 |
|------|--------|-----------|------|
| 降采样批量 last | recordAdminOpLast | Downsample 芯片 | 与用户批量对称 |
| gRPC 批量 | 同步 record | — | 与 HTTP 一致 |
| 就绪清单 | — | batch-admin-last | 可勾选自动化项 |

## P431（2026-07-22）
- 服务端：单条用户禁用/启用（HTTP PUT + gRPC UpdateUser）写轻量 last（`user_disable` / `user_enable`）
- Dashboard：op 文案；Operations 复制 last 回退与 Storage 对齐
- e2e：reader 单条禁用后 `users-admin-last` 可见

### 前后端对齐（P431）
| 场景 | 服务端 | Dashboard | 备注 |
|------|--------|-----------|------|
| 单条禁用 last | recordUserDisabledTransitionLast | applyAdminOpStatus | 仅 disabled 状态变化时写入 |
| Ops 复制 | — | label 回退 | 与 Storage/chip 一致 |

## P432（2026-07-22）
- Runbook：单条/批量禁用 last 与会话边界说明
- API Spec：users_resource / update_user ResponseHint 含 user_disable|user_enable last
- 命令面板关键词 `user_disable`/`user_enable`；e2e 单条启用后 last 芯片
- 就绪清单 `batch-admin-last` 文案覆盖单条启停

### 前后端对齐（P432）
| 场景 | 服务端 | Dashboard | 备注 |
|------|--------|-----------|------|
| UpdateUser 契约 | ResponseHint last | API Spec 可见 | 与 PUT 行为一致 |
| 单条启用 last | user_enable | e2e 芯片 | 与禁用对称 |
| 运维文档 | — | Runbook 5.x | 可商用操作说明 |

## P433（2026-07-22）
- Users：单条启用/禁用确认框（禁用文案含 token 撤销）
- Downsample：真实创建策略 + 批量禁用后 `downsample-admin-last` e2e
- Storage：last 展示统一为 `AdminOpLastChip`（testid 保持 `storage-admin-last`）

### 前后端对齐（P433）
| 场景 | 服务端 | Dashboard | 备注 |
|------|--------|-----------|------|
| 单条禁用确认 | user_disable last | ConfirmDialog + 撤销文案 | 与批量口径一致 |
| 降采样批量 last | batch_downsample_* | e2e 芯片 | 真实写路径 |
| Storage last | ops-status last | AdminOpLastChip | 复制/跳转运维一致 |

## P434（2026-07-22）
- Downsample 创建表单暴露 source/target retention、refresh、lookback（默认 autogen/1m）
- Operations last 统一为 `AdminOpLastChip`（保留 ops-status-last* testid）
- e2e：创建对话框可见高级字段；批量创建路径填写 retention/refresh

### 前后端对齐（P434）
| 场景 | 服务端 | Dashboard | 备注 |
|------|--------|-----------|------|
| 创建默认值 | applyDownsamplePolicyRequestDefaults | 表单可见可改 | POC 可商用 |
| Ops last | ops-status last | AdminOpLastChip | 与 Storage 一致 |

## P435（2026-07-22）
- Downsample：创建表单 batch_size；列表 path/interval 可核对 retention/refresh；导出 v2 含高级字段
- 就绪清单 `downsample-advanced-form` + 命令面板入口
- e2e：batch_size 字段与列表 path/interval testid

### 前后端对齐（P435）
| 场景 | 服务端 | Dashboard | 备注 |
|------|--------|-----------|------|
| 创建 batch_size | 默认 100 | 表单可改 | 与 defaults 一致 |
| 导出契约 | — | export version 2 | retention/refresh/lookback/batch_size |
| 就绪清单 | — | downsample-advanced-form | 可勾选 |

## P436（2026-07-22）
- Downsample：策略详情面板（retention/refresh/lookback/batch_size/functions + 水位摘要）
- 纯函数 `downsamplePolicyDetail` 单测；就绪清单 `downsample-policy-detail`
- e2e：打开/关闭详情并核对高级字段

### 前后端对齐（P436）
| 场景 | 服务端 | Dashboard | 备注 |
|------|--------|-----------|------|
| 策略详情 | 列表/状态 API | detail panel | 只读核对 |
| 就绪清单 | — | downsample-policy-detail | 可勾选 |

## P437（2026-07-22）
- Downsample：策略详情复制 JSON / 详情链接；`?policy=` 深链打开详情
- routePrefill 支持 policy；纯函数 detail JSON v1
- 深链缺失策略：详情壳 + 提示，不自动写
- e2e：复制按钮、URL policy、深链重开详情
- 就绪清单：`downsample-policy-deep-link`
- Server：`GET /api/v1/admin/downsample/policies/{name}` 单策略读取（含 admin_op last）

### 前后端对齐（P437）
| 场景 | 服务端 | Dashboard | 备注 |
|------|--------|-----------|------|
| 详情深链 | — | `?policy=` + hash | 只读，不自动写 |
| 详情导出 | — | JSON kind policy_detail | 运维粘贴核对 |
| 单策略 GET | `GET .../policies/{name}` | 列表+深链；契约可直读 | 对齐 registry GET |
| 就绪清单 | — | downsample-policy-deep-link | 可勾选 |

## P438（2026-07-22）
- Downsample 列表独立 refresh 列（不再仅悬停 interval）
- 详情 functions 分行清单 + 复制 Markdown
- 深链列表缺失时 `GET /api/v1/admin/downsample/policies/{name}` 直读补齐
- e2e：refresh 列、functions 列表、copy-md

### 前后端对齐（P438）
| 场景 | 服务端 | Dashboard | 备注 |
|------|--------|-----------|------|
| 单策略直读 | GET policies/{name} | 深链补齐 | 失败走 missing 提示 |
| 列表 refresh | 策略字段已有 | 独立列 | 运维扫视 |
| 详情 functions | policy.functions | 分行 + MD | 粘贴核对 |

## P439（2026-07-22）
- Server：`GET /api/v1/admin/downsample/policies/{name}/status` 单策略状态
- Dashboard 状态表增加 next_run 列；详情 status 展示 lag/duration/windows/points/active
- 深链补齐时直读单策略 status

### 前后端对齐（P439）
| 场景 | 服务端 | Dashboard | 备注 |
|------|--------|-----------|------|
| 单策略 status | GET .../status | 详情/深链补齐 | 列表仍用 statuses 批量 |
| next_run/lag | status 字段 | 状态表+详情 | 运维扫视 |

## P440（2026-07-22）
- 导出 v3：JSON/CSV 含 functions、group_by_tags
- 状态健康筛选：error/active/lagging（min_lag_seconds）
- Server：`GET /admin/downsample/statuses?health=&q=&min_lag_seconds=`
- 状态行点击打开策略详情
- e2e：status filter、状态行点开详情

### 前后端对齐（P440）
| 场景 | 服务端 | Dashboard | 备注 |
|------|--------|-----------|------|
| 状态筛选 | statuses query | health/min_lag + 本地二次过滤 | 可商用扫视 |
| 导出 functions | policy.functions | export v3 | 交接核对 |
| 状态→详情 | status/policy GET | 行点击 open detail | 只读 |

## P441（2026-07-22）
- Server：`GET /admin/downsample/statuses` 附带 `summary`（total/enabled/active/error/lagging/max_lag）
- Overview：降采样健康卡片 + 跳转 Downsample
- 纯函数 `downsampleStatusSummary` 单测

### 前后端对齐（P441）
| 场景 | 服务端 | Dashboard | 备注 |
|------|--------|-----------|------|
| statuses summary | summary 字段 | Overview 卡片 | 过滤后汇总 |
| 跳转 | — | /downsample#status | 运维下钻 |

## P442（2026-07-22）
- 就绪清单 `downsample-status-health` + 命令面板入口（含 Overview 健康卡片）
- Downsample 状态摘要条 + 导出 JSON/CSV + 复制摘要
- 客户端本地汇总回退；优先服务端 `summary`
- e2e：status summary 可见与导出按钮

### 前后端对齐（P442）
| 场景 | 服务端 | Dashboard | 备注 |
|------|--------|-----------|------|
| statuses summary | summary 字段 | 页内摘要条/导出 | 过滤后汇总 |
| 就绪清单 | — | downsample-status-health | 可勾选 |
| 命令面板 | — | readiness + overview 入口 | 运维跳转 |

## P443（2026-07-22）
- routePrefill：`health` / `min_lag` 与 server `statuses?health=&min_lag_seconds=` 对齐
- 状态健康筛选写入 URL；复制分享链接带健康筛选
- 就绪清单/命令面板深链 `?health=error#downsample-status`
- e2e：深链预填 health=error

### 前后端对齐（P443）
| 场景 | 服务端 | Dashboard | 备注 |
|------|--------|-----------|------|
| 状态健康深链 | statuses query | `?health=&min_lag=` | 只读筛选 |
| 分享链接 | — | share 含 health | 运维协作 |

## P444（2026-07-22）
- Overview 降采样健康卡片：一键深链 error / lagging / active（只读筛选）
- `downsampleStatusSummaryJump` 默认带 `?health=`
- Overview / Readiness 导出与归档纳入 `downsample_status_summary`
- Server：`GET /admin/downsample/statuses?summary_only=1` 仅返回 summary（省略 statuses 行）
- Overview/Readiness 轻量拉取 `summary_only=1`
- e2e：Overview 错误深链跳转验证

### 前后端对齐（P444）
| 场景 | 服务端 | Dashboard | 备注 |
|------|--------|-----------|------|
| summary_only | statuses?summary_only=1 | Overview/Readiness 轻量摘要 | 减少载荷 |
| 一键健康深链 | statuses?health= | Overview 按钮 | 只读筛选 |
| 导出归档摘要 | summary 字段 | overview/readiness/acceptance | 交接核对 |

## P445（2026-07-22）
- Readiness：降采样健康卡片 + error/lagging 一键深链（`#downsample-health-panel`）
- Metrics：降采样健康摘要条 + 深链（`summary_only=1` 轻量拉取）
- Overview 导出契约 `mts.overview.snapshot` **version: 2**（含 `downsample_status_summary`）
- 就绪清单 `readiness-downsample-health-card` + 命令面板入口（Readiness/Metrics）
- e2e：Readiness 错误深链、Metrics 摘要可见

### 前后端对齐（P445）
| 场景 | 服务端 | Dashboard | 备注 |
|------|--------|-----------|------|
| summary_only | statuses?summary_only=1 | Readiness/Metrics/Overview | 轻量摘要 |
| 就绪健康卡片 | summary | Readiness 卡片+归档 | 只读 |
| Metrics 摘要条 | summary | Metrics 页 | 运维扫视 |
| Overview 导出 v2 | — | version=2 + summary | 交接契约 |

## P446（2026-07-22）
- Operations：降采样健康条 + error/lagging 深链（`summary_only=1`）
- Metrics 导出 JSON **version: 2** 含 `downsample_status_summary`
- 全局 Admin 横幅：error/lagging>0 时提示 + 深链/刷新（60s 轮询，只读）
- 就绪清单 `ops-downsample-health-card` + 命令面板运维入口
- e2e：Operations 健康条；横幅软断言

### 前后端对齐（P446）
| 场景 | 服务端 | Dashboard | 备注 |
|------|--------|-----------|------|
| Ops 健康条 | summary_only | Operations 页 | 只读 |
| Metrics 导出 v2 | summary | JSON 附带 summary | 交接 |
| 全局告警横幅 | summary_only 轮询 | Layout banner | error/lag 触发 |

## P447（2026-07-22）
- 全局降采样告警横幅：session 级 dismiss（按 summary 指纹；摘要变化重新提示）
- opsExport：`mts.ops.stats` **version: 2** 含 `downsample_status_summary`
- 快捷键 `g` 然后 `d`：admin 跳转 `/downsample?health=error#downsample-status`
- 命令面板关键词 `g d` / `gd` 对齐错误深链
- e2e：横幅 dismiss 按钮软断言

### 前后端对齐（P447）
| 场景 | 服务端 | Dashboard | 备注 |
|------|--------|-----------|------|
| 横幅 dismiss | summary 指纹 | sessionStorage | 会话级 |
| ops 导出 v2 | summary | ops stats JSON | 交接 |
| 快捷深链 | statuses?health=error | g then d | 只读筛选 |

## P448（2026-07-22）
- ForceChangePassword：密码策略提示门禁（未满足不可提交）
- Overview：一键「健康报告」聚合导出（overview v2 + downsample summary + ops 片段）
- 全局降采样告警横幅：复制摘要
- e2e：Overview 健康报告按钮；横幅 copy 软断言

### 前后端对齐（P448）
| 场景 | 服务端 | Dashboard | 备注 |
|------|--------|-----------|------|
| 强制改密门禁 | password_policy | 前端 hints + 提交门禁 | 与 server 对齐 |
| 健康报告 | summary/stats | mts.health.report | 交接扫视 |
| 横幅复制 | summary | 剪贴板文本 | 只读 |

## P449（2026-07-22）
- Overview：健康报告 Markdown 导出 + 复制（`formatHealthReportMarkdown` / `healthReportFilenames`）
- Login：默认密码风险提示强化（`login-default-risk`）
- ForceChange / PasswordHints：策略分项进度条（`passwordHintsProgress`，submit 保持可点）
- e2e：login 风险条、改密进度、Overview MD/复制按钮软断言

### 前后端对齐（P449）
| 场景 | 服务端 | Dashboard | 备注 |
|------|--------|-----------|------|
| 健康报告 MD | 无变更 | md 文本导出/复制 | 交接扫视 |
| 默认密码风险 | seed admin | Login 风险条 | 运维提示 |
| 改密进度 | password_policy | 进度条 + 分项 | 不禁用 submit |

