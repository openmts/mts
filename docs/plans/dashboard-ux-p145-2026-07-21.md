# Dashboard / Data 面库列表（2026-07-21 P145）

## 范围
- 服务端：`GET /api/v1/data/databases` 返回当前用户可读库
- 前端：`listDatabasesDetailed` 优先 data，回退 admin，再 manual
- 非 admin 查询/写入页可自动填充库列表

## 边界
- 不替代 admin 建库/删库
- 不做分页（库数量通常可控）

## EARS
- [x] EARS-BE-P145-01 WHEN data 用户 GET /data/databases THE SYSTEM SHALL 仅返回其有 read 权限的库
- [x] EARS-BE-P145-02 WHEN 非 admin 访问 admin/databases THE SYSTEM SHALL 仍拒绝
- [x] EARS-FE-P145-03 WHEN 拉取库列表 THE SYSTEM SHALL 优先 data 路径
- [x] EARS-DOC-P145-04 WHEN 更新基线 THE SYSTEM SHALL 记录 P145

## 验证
- [x] `go test ./cmd/mts-server -run TestHTTPDataListDatabasesForReadUser...`
- [x] `go test -count=1 -timeout 120s ./...`
- [x] `golangci-lint run ./cmd/mts-server/...`
- [x] `npm test` / `npm run build` / `npm run test:e2e`
- [x] `make e2e`

## 实现备注
- 路由：`GET /api/v1/data/databases`
- admin 看全量；普通用户按 read 权限过滤
- FE：`listDatabasesDetailed` 优先 data → admin → manual
