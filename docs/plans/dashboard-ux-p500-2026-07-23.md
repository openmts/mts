# Dashboard UX P500 — Access Grants/Matrix path 扫视

## 目标
Access Grants 与 Access Matrix 补齐 path/覆盖率扫视与深链，导出 meta 对齐。

## 范围
- `accessGrantsMetaAlign` / `accessMatrixMetaAlign` 纯函数 + 单测
- Grants：users path + permissions 样例 path、覆盖率、Users/Audit/Matrix 深链；export v2
- Matrix：能力行/路由覆盖统计、Grants/Users 深链
- 清单：`access-grants-meta-align`、`access-matrix-meta-align`
- e2e commercial-smoke 软断言

## 验收
- [x] 纯函数单测
- [x] testid 摘要卡 / 深链
- [x] grants export v2
- [x] 清单 + commandPalette
- [x] npm test / build / commercial-smoke
