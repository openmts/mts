# Dashboard 对齐修复 EARS 清单（2026-07-19）

- **来源**: `docs/review/code-review-2026-07-19-1933-mts-dashboard-alignment.md`
- **说明**: 本轮完成前后端契约对齐与查询体验闭环（P0/P1 + P2-08）

## P0
- [x] EARS-FE-P0-01 删除契约对齐（DeleteRequest json tag 或前端字段）
- [x] EARS-FE-P0-02 降采样启停 action：enable/disable

## P1
- [x] EARS-FE-P1-01 非 admin 元数据可用（手填降级/data API）
- [x] EARS-FE-P1-02 查询 Builder：tags + order/offset
- [x] EARS-FE-P1-03 FieldValue 展示规范化
- [x] EARS-FE-P1-04 列式结果表格/图

## P2
- [ ] EARS-FE-P2-01 暗色主题全站
- [ ] EARS-FE-P2-02 i18n 关键路径
- [ ] EARS-FE-P2-03 嵌入子路径与 server 闭环
- [ ] EARS-FE-P2-04 共享 API 类型
- [ ] EARS-FE-P2-05 降采样 run/reset/dry-run
- [ ] EARS-FE-P2-06 maintenance errors + health checks
- [ ] EARS-FE-P2-07 前端契约测试覆盖 P0
- [x] EARS-FE-P2-08 list databases 返回 databases 字段

## P3
- [ ] EARS-FE-P3-01 聚合窗口查询 UI
- [ ] EARS-FE-P3-02 结果导出 CSV
- [ ] EARS-FE-P3-03 TypedBatch 多列编辑器
- [ ] EARS-FE-P3-04 图表多 series
- [ ] EARS-FE-P3-05 API Spec 浏览器
- [ ] EARS-FE-P3-06 审计查持久化

## 实现备注

### 2026-07-19 闭环
- `DeleteRequest` 补齐 snake_case JSON tag，前端删除请求可正确解析。
- 降采样 toggle 改为 `enable`/`disable`，文案同步为启用/禁用。
- `listDatabasesDetailed`：admin 403 时 manual 手填；Write/Query 均支持 datalist。
- Query：tags 过滤、order `{by:1,direction:1|2}`、offset；FieldValue 规范化展示；列结果摘要表。
- list databases 响应同时返回 `databases` + 兼容 `measurements`。
- 契约测试：`TestHTTPDeleteAcceptsSnakeCaseJSON`、`TestHTTPListDatabasesReturnsDatabasesField`。

### 验证
- `cmd/mts-dashboard`: `npm run test` + `npm run build` 通过
- `go test ./cmd/mts-server -run 'TestHTTPDeleteAcceptsSnakeCaseJSON|TestHTTPListDatabasesReturnsDatabasesField'` 通过
- `make test` 通过（首次偶发 compaction_integrity 失败后复测通过）
- `make e2e` 通过
- `make lint` 0 issues
