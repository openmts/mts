# Dashboard / Operations 只读状态条 EARS（2026-07-20 P126）

## 范围
- Operations 页顶部展示只读状态条：服务连通性 + 统计最近刷新时间 + 刷新中
- 复用 `useServerReachability`（与顶栏同源 /readyz）
- 动作仍保持确认门禁；状态条只读

## 边界
- 不改 flush/compact/retention 写路径
- 不将连通性状态计入 readiness 评分

## EARS
- [x] EARS-FE-P126-01 WHEN 进入运维页 THE SYSTEM SHALL 展示连通性与最近统计刷新时间
- [x] EARS-FE-P126-02 WHEN 用户刷新统计 THE SYSTEM SHALL 更新最近刷新时间与加载态
- [x] EARS-FE-P126-03 WHEN /readyz 不可达 THE SYSTEM SHALL 在状态条显示不可达文案（与顶栏同源）
- [x] EARS-FE-P126-04 WHEN 商业冒烟运行 THE SYSTEM SHALL 覆盖 ops-status-strip
- [x] EARS-DOC-P126-05 WHEN 更新基线 THE SYSTEM SHALL 记录 P126

## 实现备注
- testid：`ops-status-strip` / `ops-status-connectivity` / `ops-status-stats-at` / `ops-status-loading`
- i18n：`opsStatusStripTitle` / `opsStatsLastLoaded` / `opsStatsNeverLoaded` / `opsStatsLoading`

## 验证
- `cd cmd/mts-dashboard && npm test && npm run build && npm run test:e2e`
- 根目录 `make e2e` + `go test ./...`
