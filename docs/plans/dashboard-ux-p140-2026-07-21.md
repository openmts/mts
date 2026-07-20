# Dashboard / Data 面只读 RP EARS（2026-07-21 P140）

## 范围
- 服务端 `GET /api/v1/data/databases/{db}/retention-policies`（需 database read）
- Dashboard `listRetentionPolicies` 优先 data 路径，回退 admin
- 非 admin 查询/写入页可自动填充 RP 列表

## 边界
- 不开放 data 面创建/删除 RP（仍 admin）
- 不改变 admin 路径行为

## EARS
- [x] EARS-BE-P140-01 WHEN 用户具备 database read THE SYSTEM SHALL 允许 GET data 面 RP 列表
- [x] EARS-BE-P140-02 WHEN 用户无 admin THE SYSTEM SHALL 拒绝 admin RP 路径
- [x] EARS-FE-P140-03 WHEN 拉取 RP THE SYSTEM SHALL 优先 data 路径并回退 admin
- [x] EARS-DOC-P140-04 WHEN 更新基线 THE SYSTEM SHALL 记录 P140

## 验证
- go test ./cmd/mts-server -run DataListRetention ✅
- npm test && npm run build && npm run test:e2e ✅
- make e2e + go test ./... ✅
- golangci-lint ./cmd/mts-server ✅
