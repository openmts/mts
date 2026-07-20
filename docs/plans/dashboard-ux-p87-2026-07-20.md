# Dashboard / 导航壳层体验 EARS（2026-07-20 P87）

## 范围
- 最近访问：一键清空（`clearRecentRoutes` + UI）
- 侧栏：展开态导航搜索/过滤；折叠态隐藏搜索框并清空过滤词
- 商业冒烟覆盖相关 testid

## 边界
- 不改路由表与权限模型
- 过滤仅作用于当前角色可见导航项
- 不宣称部署侧验收完成

## EARS
- [x] EARS-FE-P87-01 WHEN 用户存在多条最近访问 THE SYSTEM SHALL 提供清空入口
- [x] EARS-FE-P87-02 WHEN 用户清空最近访问 THE SYSTEM SHALL 清除 session 记录并仅保留当前页
- [x] EARS-FE-P87-03 WHEN 侧栏展开 THE SYSTEM SHALL 支持按 label/path 过滤导航
- [x] EARS-FE-P87-04 WHEN 侧栏折叠 THE SYSTEM SHALL 隐藏过滤框并重置过滤词
- [x] EARS-FE-P87-05 WHEN 商业冒烟运行 THE SYSTEM SHALL 覆盖 `sidebar-filter` 与 `recent-routes-clear`
- [x] EARS-DOC-P87-06 WHEN 更新基线 THE SYSTEM SHALL 记录 P87

## 实现备注
- `clearRecentRoutes` / `filterNavItems` 纯函数 + 单测
- testid：`recent-routes-clear`、`sidebar-filter`、`sidebar-filter-clear`、`sidebar-filter-empty`
- 清空后调用 `recordRecentRoute(当前路由)`，避免整条 recent 条消失

## 验证
- `cd cmd/mts-dashboard && npm test && npm run build && npm run test:e2e`
- 仓库根：`make e2e` / `make lint` / `go test ./...`
