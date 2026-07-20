# Dashboard / 运维可商用 EARS 清单（2026-07-20 P63）

## 范围
- Databases 页 Measurements / Fields / Series 分区标题 i18n
- Databases 字段类型标签 i18n
- Audit 导出 JSON/CSV 按钮文案 i18n

## 边界
- Measurement/Series 等时序领域词可保留英文
- 字段类型名 float/int/string/bool 保持技术术语
- 不改变库管理/审计 API
- 不宣称生产验收完成

## EARS
- [x] EARS-FE-P63-01 WHEN 展开数据库详情 THE SYSTEM SHALL 使用 databasesMeasurements/Fields/Series 标题
- [x] EARS-FE-P63-02 WHEN 展示字段类型 THE SYSTEM SHALL 使用 typeFloat/Int/String/Bool
- [x] EARS-FE-P63-03 WHEN 导出审计日志 THE SYSTEM SHALL 使用 exportJSON/exportCSV 按钮文案
- [x] EARS-FE-P63-04 WHEN 商业冒烟 e2e 运行 THE SYSTEM SHALL 覆盖数据库与审计入口
- [x] EARS-DOC-P63-05 WHEN 更新基线 THE SYSTEM SHALL 记录 P63 与仍未完成项

## 验证
- `cd cmd/mts-dashboard && npm test && npm run build && npm run test:e2e`
- 仓库根：`make e2e` / `make lint` / `go test ./...`

## 可商用仍未完成（不宣称目标完成）
- 真实边缘证书/HSTS 人工验收执行
- 目标环境 cron/systemd 实装与演练归档
- 跨主机异地备份 + 告警通道真实跑通
