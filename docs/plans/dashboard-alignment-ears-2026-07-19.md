# Dashboard 对齐修复 EARS 清单（2026-07-19）

- **来源**: `docs/review/code-review-2026-07-19-1933-mts-dashboard-alignment.md`
- **说明**: P0/P1/P2/P3 已闭环

## P0
- [x] EARS-FE-P0-01 删除契约对齐（DeleteRequest json tag 或前端字段）
- [x] EARS-FE-P0-02 降采样启停 action：enable/disable

## P1
- [x] EARS-FE-P1-01 非 admin 元数据可用（手填降级/data API）
- [x] EARS-FE-P1-02 查询 Builder：tags + order/offset
- [x] EARS-FE-P1-03 FieldValue 展示规范化
- [x] EARS-FE-P1-04 列式结果表格/图

## P2
- [x] EARS-FE-P2-01 暗色主题全站
- [x] EARS-FE-P2-02 i18n 关键路径
- [x] EARS-FE-P2-03 嵌入子路径与 server 闭环
- [x] EARS-FE-P2-04 共享 API 类型
- [x] EARS-FE-P2-05 降采样 run/reset/dry-run
- [x] EARS-FE-P2-06 maintenance errors + health checks
- [x] EARS-FE-P2-07 前端契约测试覆盖 P0
- [x] EARS-FE-P2-08 list databases 返回 databases 字段

## P3
- [x] EARS-FE-P3-01 聚合窗口查询 UI
- [x] EARS-FE-P3-02 结果导出 CSV
- [x] EARS-FE-P3-03 TypedBatch 多列编辑器
- [x] EARS-FE-P3-04 图表多 series
- [x] EARS-FE-P3-05 API Spec 浏览器
- [x] EARS-FE-P3-06 审计查持久化

## 实现备注

### 2026-07-19 第三轮（P3 闭环）
- 查询：aggregates / window / group_tags 接入 `buildQuery`；行结果 CSV 导出。
- TypedBatch：多 tag 列 + 多 field 列（float/int/string/bool）。
- 图表：按 tag 组合拆分多 series，可配置 max series。
- API Spec：admin 页面浏览 `/api/v1/admin/api-spec`。
- 审计：内存环与 `_internal.audit_log` 合并读回；页面提示 + CSV 导出。

### 验证
- dashboard: `npm run test` + `npm run build`
- `make test` / `make e2e` / `make lint` 通过
