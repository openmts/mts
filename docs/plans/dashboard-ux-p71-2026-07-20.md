# Dashboard / 运维可商用 EARS 清单（2026-07-20 P71）

## 范围
- Doctor 检查 level 展示 i18n（Overview + 就绪中心）
- TopBar 迷你连通性徽章（与可达性单例同源）
- 关键管理表增加 overflow-x 保护（Users/Config）
- 打印样式：隐藏导航/命令条，保留主内容

## 边界
- 连通徽章不计就绪评分
- 不宣称生产验收完成
- 打印样式仅浏览器 print，不改导出包格式

## EARS
- [x] EARS-FE-P71-01 WHEN 展示 Doctor level THE SYSTEM SHALL 使用 healthStatusLabel 本地化常见 level
- [x] EARS-FE-P71-02 WHEN 顶栏渲染且非登录页 THE SYSTEM SHALL 展示连通性迷你徽章（ok/warn/error 语义）
- [x] EARS-FE-P71-03 WHEN Users/Config 宽表展示 THE SYSTEM SHALL 使用 overflow-x-auto 包裹
- [x] EARS-FE-P71-04 WHEN 用户打印页面 THE SYSTEM SHALL 隐藏侧栏/顶栏/离线条/通知，保留 main
- [x] EARS-FE-P71-05 WHEN 商业冒烟 e2e 运行 THE SYSTEM SHALL 覆盖 topbar-connectivity
- [x] EARS-DOC-P71-06 WHEN 更新基线 THE SYSTEM SHALL 记录 P71 与仍未完成部署侧项

## 验证
- `cd cmd/mts-dashboard && npm test && npm run build && npm run test:e2e`
- 仓库根：`make e2e` / `make lint` / `go test ./...`

## 可商用仍未完成（不宣称目标完成）
- 真实边缘证书/HSTS 人工验收执行
- 目标环境 cron/systemd 实装与演练归档
- 跨主机异地备份 + 告警通道真实跑通
