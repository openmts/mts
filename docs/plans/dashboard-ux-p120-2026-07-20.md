# Dashboard / 命令面板只读预填 EARS（2026-07-20 P120）

## 范围
- 查询页：`?range=1h|24h|7d|30d`（可选 database/measurement）预填时间，不自动执行
- 审计页：`?range=&action=&q=&user=` 预填筛选，不自动导出
- 命令面板新增预填深链导航项
- 禁止危险写操作自动执行

## 边界
- 不自动点「执行查询 / 导出 / flush / compact / retention」
- 预填失败静默忽略非法参数

## EARS
- [x] EARS-FE-P120-01 WHEN 用户打开 `/query?range=1h` THE SYSTEM SHALL 预填 start/end 且不自动执行
- [x] EARS-FE-P120-02 WHEN 用户打开 `/audit?range=1h` THE SYSTEM SHALL 预填 since/until
- [x] EARS-FE-P120-03 WHEN 命令面板搜索预填关键词 THE SYSTEM SHALL 展示只读预填导航项
- [x] EARS-FE-P120-04 WHEN 商业冒烟运行 THE SYSTEM SHALL 覆盖查询/审计预填深链
- [x] EARS-DOC-P120-05 WHEN 更新基线 THE SYSTEM SHALL 记录 P120

## 实现备注
- `utils/routePrefill.ts` 纯函数
- 命令项：`query-range-*` / `audit-range-*` / `audit-action-login`

## 验证
- `cd cmd/mts-dashboard && npm test && npm run build && npm run test:e2e`
- 根目录 `make e2e` + `go test ./...`
