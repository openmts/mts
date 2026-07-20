# Dashboard / 偏好包纳入侧栏导航排序 EARS（2026-07-20 P98）

## 范围
- 账户偏好导出/导入/重置覆盖 `nav_order`（侧栏组内导航顺序）
- 偏好变更事件触发侧栏排序热刷新
- 商业冒烟：排序写入后重置清空 localStorage

## 边界
- 仍不含密码/Token；不跨分组排序
- 不宣称部署侧验收完成

## EARS
- [x] EARS-FE-P98-01 WHEN 导出账户/偏好包 THE SYSTEM SHALL 包含 `prefs.nav_order`
- [x] EARS-FE-P98-02 WHEN 导入含 nav_order 的偏好 THE SYSTEM SHALL 写回本机并刷新侧栏
- [x] EARS-FE-P98-03 WHEN 重置本机偏好 THE SYSTEM SHALL 清除导航排序
- [x] EARS-FE-P98-04 WHEN 商业冒烟运行 THE SYSTEM SHALL 验证排序持久化与重置清除
- [x] EARS-DOC-P98-05 WHEN 更新基线 THE SYSTEM SHALL 记录 P98

## 实现备注
- `ClientPrefs.nav_order` + `parseNavOrderMap` 规范化
- `AccountPage` apply/reset/export 读写 `mts.dashboard.nav-order.prefs.v1`
- `useSidebarNavOrder` 监听 `mts-dashboard-prefs-changed`

## 验证
- `cd cmd/mts-dashboard && npm test && npm run build && npm run test:e2e`
- 仓库根：`make e2e` / `make lint` / `go test ./...`
