# Dashboard / 侧栏组内拖拽排序 EARS（2026-07-20 P107）

## 范围
- 侧栏分组内导航项支持 HTML5 拖拽重排，顺序写入既有 `nav_order` localStorage
- 保留 ↑↓ 按钮作为无障碍回退；折叠/过滤时不显示拖拽与排序控件
- 不跨分组拖动；仅同组 path 可 drop
- 商业冒烟覆盖拖拽手柄可见与拖拽后顺序持久化

## 边界
- 不引入第三方 DnD 库
- 不宣称部署侧验收完成
- 偏好导出/导入仍走既有 `nav_order`（P98），本轮不改序列化格式

## EARS
- [x] EARS-FE-P107-01 WHEN 用户在组内将导航项拖到另一项上 THE SYSTEM SHALL 在该组内重排并本机持久化
- [x] EARS-FE-P107-02 WHEN 拖拽源与目标不在同一分组 THE SYSTEM SHALL 忽略本次放置
- [x] EARS-FE-P107-03 WHEN 侧栏折叠或过滤激活 THE SYSTEM SHALL 隐藏拖拽手柄
- [x] EARS-FE-P107-04 WHEN 商业冒烟运行 THE SYSTEM SHALL 覆盖拖拽手柄与重排持久化
- [x] EARS-DOC-P107-05 WHEN 更新基线 THE SYSTEM SHALL 记录 P107

## 实现备注
- `moveNavPathTo` 纯函数：arrayMove 语义
- `useSidebarNavOrder.reorderTo` 写回 `setSectionOrder`
- `useSidebarNavDrag`：HTML5 DnD 状态与事件
- `SidebarNav`：Grip 手柄 + drop 行；↑↓ 保留
- testid：`sidebar-drag-*`、`sidebar-nav-row-*`（已有）

## 验证
- `cd cmd/mts-dashboard && npm test && npm run build && npm run test:e2e`
- 仓库根：`make e2e` / `make lint` / `go test ./...`（若无 Go 改动可跳过 lint/go 全量，至少确认前端门禁）
