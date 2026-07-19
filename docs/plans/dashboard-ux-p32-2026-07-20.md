# Dashboard / 运维可商用 EARS 清单（2026-07-20 P32）

## 范围
- Overview：本地就绪评分摘要 + 跳转就绪中心
- Playwright：验收包按钮、降采样筛选/批量入口、Overview 评分
- 文档同步

## 边界
- 评分仍基于浏览器本地清单 + doctor 摘要，不替代人工验收
- 不宣称可商用完成

## EARS
- [x] EARS-FE-P32-01 WHEN 管理员打开 Overview THE SYSTEM SHALL 展示本地就绪评分与分项摘要
- [x] EARS-FE-P32-02 WHEN 点击 Overview 就绪入口 THE SYSTEM SHALL 导航到 /ops/readiness
- [x] EARS-FE-P32-03 WHEN 商业冒烟 e2e 运行 THE SYSTEM SHALL 覆盖验收包按钮、降采样筛选栏与 Overview 评分
- [x] EARS-DOC-P32-04 WHEN 更新基线 THE SYSTEM SHALL 记录 P32 与仍未完成项

## 验证
- `cd cmd/mts-dashboard && npm run test && npm run build`
- `make test` / `make e2e` / `make lint`

## 可商用仍未完成（不宣称目标完成）
- 真实边缘证书/HSTS 人工验收执行
- 目标环境 cron/systemd 实装与演练归档
- 跨主机异地备份 + 告警通道真实跑通
