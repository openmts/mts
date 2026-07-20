# Dashboard / 运维可商用 EARS 清单（2026-07-20 P60）

## 范围
- Users 角色筛选与列表角色列 i18n（roleAdmin/roleUser）
- AccessGrants / Account 角色展示本地化

## 边界
- 角色值仍为 admin/user（API 不变），仅展示层本地化
- 不宣称生产验收完成

## EARS
- [x] EARS-FE-P60-01 WHEN 用户列表筛选角色 THE SYSTEM SHALL 展示本地化角色名
- [x] EARS-FE-P60-02 WHEN 用户列表展示角色列 THE SYSTEM SHALL 使用 roleAdmin/roleUser
- [x] EARS-FE-P60-03 WHEN 授权/账户页展示角色 THE SYSTEM SHALL 使用本地化角色名
- [x] EARS-FE-P60-04 WHEN 商业冒烟 e2e 运行 THE SYSTEM SHALL 覆盖用户筛选入口
- [x] EARS-DOC-P60-05 WHEN 更新基线 THE SYSTEM SHALL 记录 P60 与仍未完成项

## 验证
- `cd cmd/mts-dashboard && npm test && npm run build && npm run test:e2e`
- 仓库根：`make e2e` / `make lint` / `go test ./...`

## 可商用仍未完成（不宣称目标完成）
- 真实边缘证书/HSTS 人工验收执行
- 目标环境 cron/systemd 实装与演练归档
- 跨主机异地备份 + 告警通道真实跑通
