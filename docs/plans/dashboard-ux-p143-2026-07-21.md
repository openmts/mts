# Dashboard / Query engine stats（2026-07-21 P143）

## 范围
- 接入独立 `GET /api/v1/data/query/stats`
- Query 页常显 stats 面板：本次查询结果 / 引擎快照
- 扩展 QueryStats 字段展示（parts/index/pages/errors 等）
- 命令面板跳转 `#query-stats`

## 边界
- 不新增独立页面路由；stats 挂在 Query 页
- 不改后端契约

## EARS
- [x] EARS-FE-P143-01 WHEN 用户点击引擎 Stats THE SYSTEM SHALL 请求 query/stats 并展示
- [x] EARS-FE-P143-02 WHEN 展示 stats THE SYSTEM SHALL 区分来源（本次查询 / 引擎快照）
- [x] EARS-FE-P143-03 WHEN 无 stats THE SYSTEM SHALL 展示空态引导
- [x] EARS-FE-P143-04 WHEN 商业冒烟运行 THE SYSTEM SHALL 覆盖 engine stats 入口
- [x] EARS-DOC-P143-05 WHEN 更新基线 THE SYSTEM SHALL 记录 P143

## 验证
- [x] `npm test`（queryStatsView）
- [x] `npm run build`
- [x] `npm run test:e2e`
- [x] `make e2e`
- [x] `go test -count=1 -timeout 120s ./...`
