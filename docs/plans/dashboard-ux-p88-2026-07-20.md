# Dashboard / 侧栏过滤快捷键 EARS（2026-07-20 P88）

## 范围
- 全局 `/`（非输入态）聚焦侧栏导航过滤
- 折叠态自动展开后再聚焦；小屏先打开抽屉
- 快捷键帮助目录登记 `/`
- 商业冒烟覆盖 focus

## 边界
- 不在 INPUT/TEXTAREA/SELECT/contenteditable 中劫持 `/`
- Shift+/（`?`）仍只打开快捷键帮助
- 不宣称部署侧验收完成

## EARS
- [x] EARS-FE-P88-01 WHEN 用户在非输入态按下 `/` THE SYSTEM SHALL 聚焦侧栏导航过滤框
- [x] EARS-FE-P88-02 WHEN 侧栏处于折叠态且触发 `/` THE SYSTEM SHALL 展开侧栏后再聚焦
- [x] EARS-FE-P88-03 WHEN 快捷键帮助打开 THE SYSTEM SHALL 展示 `/` 聚焦过滤条目
- [x] EARS-FE-P88-04 WHEN 商业冒烟运行 THE SYSTEM SHALL 断言 `sidebar-filter` 获得焦点
- [x] EARS-DOC-P88-05 WHEN 更新基线 THE SYSTEM SHALL 记录 P88

## 实现备注
- `matchSidebarFilterFocus` 纯函数 + 单测
- `SidebarNav.focusFilter` via defineExpose
- testid 复用 `sidebar-filter`

## 验证
- `cd cmd/mts-dashboard && npm test && npm run build && npm run test:e2e`
- 仓库根：`make e2e` / `make lint` / `go test ./...`
