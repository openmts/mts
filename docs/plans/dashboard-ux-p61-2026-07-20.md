# Dashboard / 运维可商用 EARS 清单（2026-07-20 P61）

## 范围
- AccessMatrix 角色列头与当前角色展示 i18n
- 库级权限 read/write/admin 展示本地化（授权面板、实时授权）
- Storage 备份演练「控制台内」标记 i18n

## 边界
- API 权限字符串仍为 read/write/admin
- 不改变授权逻辑
- 不宣称生产验收完成

## EARS
- [x] EARS-FE-P61-01 WHEN 权限矩阵展示角色列 THE SYSTEM SHALL 使用 roleAdmin/roleUser
- [x] EARS-FE-P61-02 WHEN 展示库级权限标签 THE SYSTEM SHALL 使用 permRead/Write/Admin 语义
- [x] EARS-FE-P61-03 WHEN 授权面板勾选权限 THE SYSTEM SHALL 显示本地化权限名
- [x] EARS-FE-P61-04 WHEN Storage 演练步骤标记 inDashboard THE SYSTEM SHALL 使用 storageInDashboard
- [x] EARS-FE-P61-05 WHEN 商业冒烟 e2e 运行 THE SYSTEM SHALL 覆盖权限矩阵入口
- [x] EARS-DOC-P61-06 WHEN 更新基线 THE SYSTEM SHALL 记录 P61 与仍未完成项

## 验证
- `cd cmd/mts-dashboard && npm test && npm run build && npm run test:e2e`
- 仓库根：`make e2e` / `make lint` / `go test ./...`

## 可商用仍未完成（不宣称目标完成）
- 真实边缘证书/HSTS 人工验收执行
- 目标环境 cron/systemd 实装与演练归档
- 跨主机异地备份 + 告警通道真实跑通
