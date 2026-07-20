# Dashboard / 运维可商用 EARS 清单（2026-07-20 P59）

## 范围
- Overview 压缩/维护统计标签 i18n（复用 opsStat*）
- Overview 内存字段常见 key 本地化
- About 客户端/服务端字段标签 i18n
- Downsample 运行/重置按钮 title 与空值 i18n

## 边界
- 未知内存字段 key 回退原始字段名
- 不改变统计 API 与数值语义
- 不宣称生产验收完成

## EARS
- [x] EARS-FE-P59-01 WHEN Overview 展示压缩统计 THE SYSTEM SHALL 使用 opsStat* 标签
- [x] EARS-FE-P59-02 WHEN Overview 展示维护统计 THE SYSTEM SHALL 使用 opsStat* 标签
- [x] EARS-FE-P59-03 WHEN Overview 展示内存统计 THE SYSTEM SHALL 映射常见 key 为本地化文案
- [x] EARS-FE-P59-04 WHEN About 展示版本信息 THE SYSTEM SHALL 使用 about* 字段标签
- [x] EARS-FE-P59-05 WHEN 降采样策略操作按钮展示 title THE SYSTEM SHALL 使用 downsampleRunTitle/ResetTitle
- [x] EARS-FE-P59-06 WHEN 商业冒烟 e2e 运行 THE SYSTEM SHALL 覆盖 Overview 就绪入口
- [x] EARS-DOC-P59-07 WHEN 更新基线 THE SYSTEM SHALL 记录 P59 与仍未完成项

## 验证
- `cd cmd/mts-dashboard && npm test && npm run build && npm run test:e2e`
- 仓库根：`make e2e` / `make lint` / `go test ./...`

## 可商用仍未完成（不宣称目标完成）
- 真实边缘证书/HSTS 人工验收执行
- 目标环境 cron/systemd 实装与演练归档
- 跨主机异地备份 + 告警通道真实跑通
