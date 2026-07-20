# Dashboard / 侧栏导航自定义排序 EARS（2026-07-20 P97）

## 范围
- 侧栏分组内导航项可上移/下移，顺序写入 localStorage
- 支持重置本组 / 全部顺序
- 折叠态或过滤搜索时不显示排序控件
- 商业冒烟覆盖相关 testid

## 边界
- 不跨分组拖动；权限过滤后的不可见项不参与重排
- 不宣称部署侧验收完成

## EARS
- [x] EARS-FE-P97-01 WHEN 用户上移/下移导航项 THE SYSTEM SHALL 在组内重排并本机持久化
- [x] EARS-FE-P97-02 WHEN 用户重置顺序 THE SYSTEM SHALL 恢复默认分组内顺序
- [x] EARS-FE-P97-03 WHEN 侧栏折叠或过滤激活 THE SYSTEM SHALL 隐藏排序控件
- [x] EARS-FE-P97-04 WHEN 商业冒烟运行 THE SYSTEM SHALL 覆盖 order-up/down 与过滤隐藏
- [x] EARS-DOC-P97-05 WHEN 更新基线 THE SYSTEM SHALL 记录 P97

## 实现备注
- `navOrder` 纯函数 + 单测；key `mts.dashboard.nav-order.prefs.v1`
- `useSidebarNavOrder` 组合逻辑，控制 `SidebarNav.vue` 体量
- testid：`sidebar-order-up-*`、`sidebar-order-down-*`、`sidebar-order-reset-all`

## 验证
- `cd cmd/mts-dashboard && npm test && npm run build && npm run test:e2e`
- 仓库根：`make e2e` / `make lint` / `go test ./...`
