# Dashboard / 运维可商用 EARS 清单（2026-07-20 P46）

## 范围
- 就绪归档 JSON/Markdown 纳入 `deploy_kit` 索引与 `deploy_kit_local_hints`
- 就绪状态增加 `deployKit` 本地提醒勾选（reviewed/downloaded/copied）
- 下载/复制自动勾选本地提醒；**不计入就绪评分**
- 文档同步

## 边界
- 本地勾选 ≠ 目标环境证书/cron/备份验收完成
- 不改变 readiness 四维评分算法

## EARS
- [x] EARS-FE-P46-01 WHEN 导出就绪归档 THE SYSTEM SHALL 包含 deploy_kit 摘要
- [x] EARS-FE-P46-02 WHEN 导出就绪归档 THE SYSTEM SHALL 记录 deploy_kit_local_hints
- [x] EARS-FE-P46-03 WHEN 用户下载或复制部署材料 THE SYSTEM SHALL 更新本地提醒勾选
- [x] EARS-FE-P46-04 WHEN 计算就绪评分 THE SYSTEM SHALL 不将 deployKit 本地勾选计入总分
- [x] EARS-FE-P46-05 WHEN 商业冒烟 e2e 运行 THE SYSTEM SHALL 覆盖本地提醒入口可见性
- [x] EARS-DOC-P46-06 WHEN 更新基线 THE SYSTEM SHALL 记录 P46 与仍未完成项

## 验证
- `cd cmd/mts-dashboard && npm test && npm run build`
- `cd cmd/mts-dashboard && npm run test:e2e`
- 仓库根：`make test` / `make e2e` / `make lint`

## 可商用仍未完成（不宣称目标完成）
- 真实边缘证书/HSTS 人工验收执行
- 目标环境 cron/systemd 实装与演练归档
- 跨主机异地备份 + 告警通道真实跑通
