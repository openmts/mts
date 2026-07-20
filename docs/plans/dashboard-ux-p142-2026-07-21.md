# Dashboard / Query series·fields meta（2026-07-21 P142）

## 范围
- Query 表单接入 measurement 下 fields / series 元数据
- fields：datalist 建议
- series：下拉选择填充 tags（上限 200）
- 失败可降级手填

## 边界
- 不做完整 series 过滤 UI / 分页检索
- 不改后端 API

## EARS
- [x] EARS-FE-P142-01 WHEN 选定 database+measurement THE SYSTEM SHALL 加载 fields 与 series 元数据
- [x] EARS-FE-P142-02 WHEN series 数超过上限 THE SYSTEM SHALL 截断并提示
- [x] EARS-FE-P142-03 WHEN 用户选择 series THE SYSTEM SHALL 将 tags 写入表单
- [x] EARS-FE-P142-04 WHEN 商业冒烟运行 THE SYSTEM SHALL 覆盖 series meta testid
- [x] EARS-DOC-P142-05 WHEN 更新基线 THE SYSTEM SHALL 记录 P142

## 验证
- [x] `npm test`（含 seriesMeta）
- [x] `npm run build`
- [x] `npm run test:e2e`（query-series-meta / query-fields）
- [x] `make e2e`
- [x] `go test -count=1 -timeout 120s ./...`

## 实现备注
- `listFields` / `listSeries` 走 data 面
- series 上限 200；截断 amber 提示
- 选择 series → `tagsToExpr` 写入 queryForm.tags
- 不做 series 过滤分页 UI
