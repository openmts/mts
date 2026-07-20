# Dashboard / Audit·AccessGrants 多选排序对齐 EARS（2026-07-20 P112）

## 范围
- Audit 事件表：多选、全选过滤结果、导出优先已选；列排序（time/user/action/database）本机记忆；sticky 表头
- Access Grants 表：同样多选导出与列排序（user/role/status/database/permission）；sticky 表头
- 不提供批量撤销授权 / 批量删除审计（只读清单能力）

## 边界
- 不改变服务端 API
- 不自动执行危险写操作
- 不宣称部署侧验收完成

## EARS
- [x] EARS-FE-P112-01 WHEN 用户在 Audit 勾选事件 THE SYSTEM SHALL 维护选择集并显示已选计数
- [x] EARS-FE-P112-02 WHEN Audit 有选择且导出 THE SYSTEM SHALL 仅导出已选事件
- [x] EARS-FE-P112-03 WHEN 用户点击 Audit 表头 THE SYSTEM SHALL 循环排序并本机记忆
- [x] EARS-FE-P112-04 WHEN 用户在 Access Grants 多选并导出 THE SYSTEM SHALL 仅导出已选授权行
- [x] EARS-FE-P112-05 WHEN 用户点击 Grants 表头 THE SYSTEM SHALL 循环排序并本机记忆
- [x] EARS-FE-P112-06 WHEN 商业冒烟运行 THE SYSTEM SHALL 覆盖两边选择与排序控件
- [x] EARS-DOC-P112-07 WHEN 更新基线 THE SYSTEM SHALL 记录 P112

## 实现备注
- 复用 `listSelection` / `listSort` / `useListSelection`
- Audit 行 id：`auditRowId(evt, index)` 纯函数
- Grants 行 id：`grantRowId(row)`
- testid：`audit-select-*` / `access-grants-select-*` / `*-sort-*`

## 验证
- `cd cmd/mts-dashboard && npm test && npm run build && npm run test:e2e`
- 仓库根：`make e2e` / `go test ./...`
