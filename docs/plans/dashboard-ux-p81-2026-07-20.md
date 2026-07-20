# Dashboard / 清单导出 EARS 清单（2026-07-20 P81）

## 范围
- 数据库页：筛选结果 JSON/CSV 导出
- 用户页：筛选结果 JSON/CSV 导出
- 降采样策略：筛选结果 JSON/CSV 导出
- 统一 download + stampFilename；商业冒烟覆盖 testid

## 边界
- 不改服务端元数据/用户/降采样契约
- 不宣称部署侧验收完成

## EARS
- [x] EARS-FE-P81-01 WHEN 数据库列表有筛选结果 THE SYSTEM SHALL 支持导出 JSON 与 CSV
- [x] EARS-FE-P81-02 WHEN 用户列表有筛选结果 THE SYSTEM SHALL 支持导出 JSON 与 CSV
- [x] EARS-FE-P81-03 WHEN 降采样策略有筛选结果 THE SYSTEM SHALL 支持导出 JSON 与 CSV
- [x] EARS-FE-P81-04 WHEN 商业冒烟运行 THE SYSTEM SHALL 覆盖 databases/users/downsample 导出 testid
- [x] EARS-DOC-P81-05 WHEN 更新基线 THE SYSTEM SHALL 记录 P81

## 实现备注
- 纯函数：`databasesExport` / `usersExport` / `downsampleExport`
- testid：`databases-export-json|csv`、`users-export-json|csv`、`downsample-export-json|csv`

## 验证
- `cd cmd/mts-dashboard && npm test && npm run build && npm run test:e2e`
- 仓库根：`make e2e` / `make lint` / `go test ./...`
