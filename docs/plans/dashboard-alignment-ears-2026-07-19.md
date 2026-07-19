# Dashboard 对齐修复 EARS 清单（2026-07-19）

- **来源**: `docs/review/code-review-2026-07-19-1933-mts-dashboard-alignment.md`
- **说明**: P0/P1 与 P2-01~08 已闭环

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
- [ ] EARS-FE-P3-01 聚合窗口查询 UI
- [ ] EARS-FE-P3-02 结果导出 CSV
- [ ] EARS-FE-P3-03 TypedBatch 多列编辑器
- [ ] EARS-FE-P3-04 图表多 series
- [ ] EARS-FE-P3-05 API Spec 浏览器
- [ ] EARS-FE-P3-06 审计查持久化

## 实现备注

### 2026-07-19 第二轮（P2 闭环）
- 暗色：页面/组件补 dark 类 + `mts-*` 通用表面样式。
- i18n：扩展登录/概览/运维/查询关键文案；Login/Notify 接入字典。
- 子路径：`http.dashboard_base` + `dashboardHandler(base)`；前端 API 不再拼接 `VITE_BASE`（用 `VITE_API_BASE` 可选覆盖）。
- 共享类型：`src/api/types.ts`。
- 降采样：run / reset / dry-run 入口与结果展示。
- 运维/概览：maintenance errors + health checks。
- 前端契约测：`dashboardAlign.contract.test.ts`；服务端 enable/disable action 契约。

### 验证
- dashboard: `npm run test` + `npm run build` + `VITE_BASE=/mts/ npm run build:base`
- `make test` / `make e2e` / `make lint` 通过
