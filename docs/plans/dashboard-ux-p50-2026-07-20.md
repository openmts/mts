# Dashboard / 运维可商用 EARS 清单（2026-07-20 P50）

## 范围
- 命令面板增加运维深链：部署材料包、签核备注、备份演练、边缘 HTTPS、data_dir 旁路恢复
- 统一 hash 锚点滚动工具；Readiness/Storage 使用 scheduleScrollToHash
- 商业冒烟覆盖命令面板签核/部署材料深链

## 边界
- 深链仅导航定位，不自动完成部署侧验收
- 不改变就绪评分算法

## EARS
- [x] EARS-FE-P50-01 WHEN 管理员打开命令面板 THE SYSTEM SHALL 提供运维深链项
- [x] EARS-FE-P50-02 WHEN 过滤关键字 signoff/deploy kit THE SYSTEM SHALL 匹配对应深链
- [x] EARS-FE-P50-03 WHEN 进入带 hash 的就绪/存储页 THE SYSTEM SHALL 滚动到锚点
- [x] EARS-FE-P50-04 WHEN 商业冒烟 e2e 运行 THE SYSTEM SHALL 覆盖命令面板签核与部署材料深链
- [x] EARS-DOC-P50-05 WHEN 更新基线 THE SYSTEM SHALL 记录 P50 与仍未完成项

## 验证
- `cd cmd/mts-dashboard && npm test && npm run build && npm run test:e2e`
- 仓库根：`make test` / `make e2e` / `make lint`

## 可商用仍未完成（不宣称目标完成）
- 真实边缘证书/HSTS 人工验收执行
- 目标环境 cron/systemd 实装与演练归档
- 跨主机异地备份 + 告警通道真实跑通
