# Dashboard / 查询与写入工作台深链 EARS（2026-07-20 P101）

## 范围
- Query/Write 页面关键区块 hash 锚点
- hash 驱动：打开查询历史/图表；切换写入模式
- 命令面板深链条目（非 adminOnly，工作区可用）
- 商业冒烟覆盖 query-history 与 write-mode-typed

## 边界
- 深链不自动执行查询/写入
- 不宣称部署侧验收完成

## EARS
- [x] EARS-FE-P101-01 WHEN 用户选择查询历史深链 THE SYSTEM SHALL 打开 `/query#query-history` 并展示历史面板
- [x] EARS-FE-P101-02 WHEN 用户选择 TypedBatch 深链 THE SYSTEM SHALL 打开写入页并切换到 typed 模式
- [x] EARS-FE-P101-03 WHEN 命令面板搜索工作台关键词 THE SYSTEM SHALL 展示对应 Query/Write 深链
- [x] EARS-FE-P101-04 WHEN 商业冒烟运行 THE SYSTEM SHALL 覆盖 query-history 与 write-mode-typed
- [x] EARS-DOC-P101-05 WHEN 更新基线 THE SYSTEM SHALL 记录 P101

## 验证
- `cd cmd/mts-dashboard && npm test && npm run build && npm run test:e2e`
- 仓库根：`make e2e` / `make lint` / `go test ./...`
