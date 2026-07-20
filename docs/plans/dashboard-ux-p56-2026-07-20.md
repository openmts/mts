# Dashboard / 运维可商用 EARS 清单（2026-07-20 P56）

## 范围
- Config / Storage / ApiSpec 表头与端点摘要 i18n
- Overview Doctor HTTP TLS 标签 i18n
- 关键管理页空值统一 emptyValue
- AccessGrants / ApiSpec 筛选 placeholder i18n
- fieldValue 空值常量与 emptyValue 对齐

## 边界
- 不改变后端契约与权限模型
- 不宣称生产验收完成

## EARS
- [x] EARS-FE-P56-01 WHEN 展示配置 Schema 表 THE SYSTEM SHALL 使用 configColName/Description
- [x] EARS-FE-P56-02 WHEN 展示错误码表 THE SYSTEM SHALL 使用 configColCode/HTTP/GRPC/Description
- [x] EARS-FE-P56-03 WHEN 展示 data_dir 快照表 THE SYSTEM SHALL 使用 storageColKind/Name/Size/Time
- [x] EARS-FE-P56-04 WHEN 展示 API Spec 表 THE SYSTEM SHALL 使用 apiSpecCol* 与 endpoints 摘要 i18n
- [x] EARS-FE-P56-05 WHEN Overview 展示 Doctor TLS THE SYSTEM SHALL 使用 doctorHttpTls
- [x] EARS-FE-P56-06 WHEN 关键页空值展示 THE SYSTEM SHALL 优先 emptyValue
- [x] EARS-FE-P56-07 WHEN 商业冒烟 e2e 运行 THE SYSTEM SHALL 覆盖 Config 表头入口
- [x] EARS-DOC-P56-08 WHEN 更新基线 THE SYSTEM SHALL 记录 P56 与仍未完成项

## 验证
- `cd cmd/mts-dashboard && npm test && npm run build && npm run test:e2e`
- 仓库根：`make e2e` / `make lint` / `go test ./...`

## 可商用仍未完成（不宣称目标完成）
- 真实边缘证书/HSTS 人工验收执行
- 目标环境 cron/systemd 实装与演练归档
- 跨主机异地备份 + 告警通道真实跑通
