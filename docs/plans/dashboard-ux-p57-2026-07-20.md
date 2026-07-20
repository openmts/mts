# Dashboard / 运维可商用 EARS 清单（2026-07-20 P57）

## 范围
- Query 表单标签/placeholder/排序选项/结果列头与列式摘要表 i18n
- Write 表单标签/placeholder/行号/Sync/添加 tag·field i18n
- Downsample 创建对话框 Functions 与 placeholder i18n

## 边界
- 领域词 Measurement/Tags/RP/function 名可保留英文术语
- 不改变查询/写入/降采样 API 行为
- 不宣称生产验收完成

## EARS
- [x] EARS-FE-P57-01 WHEN 展示查询表单 THE SYSTEM SHALL 使用 i18n 标签与 placeholder
- [x] EARS-FE-P57-02 WHEN 展示查询结果列头 THE SYSTEM SHALL 使用 queryCol* 键
- [x] EARS-FE-P57-03 WHEN 展示列式摘要表 THE SYSTEM SHALL 使用本地化表头与序列计数
- [x] EARS-FE-P57-04 WHEN 展示写入表单 THE SYSTEM SHALL 使用 i18n 标签/placeholder/行号
- [x] EARS-FE-P57-05 WHEN 创建降采样策略 THE SYSTEM SHALL 使用本地化 Functions 与 placeholder
- [x] EARS-FE-P57-06 WHEN 商业冒烟 e2e 运行 THE SYSTEM SHALL 覆盖查询表单关键标签
- [x] EARS-DOC-P57-07 WHEN 更新基线 THE SYSTEM SHALL 记录 P57 与仍未完成项

## 验证
- `cd cmd/mts-dashboard && npm test && npm run build && npm run test:e2e`
- 仓库根：`make e2e` / `make lint` / `go test ./...`

## 可商用仍未完成（不宣称目标完成）
- 真实边缘证书/HSTS 人工验收执行
- 目标环境 cron/systemd 实装与演练归档
- 跨主机异地备份 + 告警通道真实跑通
