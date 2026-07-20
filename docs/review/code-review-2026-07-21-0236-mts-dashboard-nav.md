# MTS Dashboard 检视补丁：导航与只读库入口对齐（2026-07-21 02:36）

- **基线**: `cf3cbf5`（P151–P155）
- **问题**: P151 已开放 `/databases` 只读浏览，但侧栏/命令面板/落地页仍把路径标为 adminOnly，非 admin 无法发现入口。
- **性质**: 产品入口对齐；**不宣称可商用完成**

## 处理

| ID | 状态 |
|---|---|
| FE-NAV-P156 databases 非 admin 可见 | **已完成** |
| FE-LANDING-P157 落地页/路由/冒烟路径 | **已完成** |
| FE-DB-P158 measurement 筛选 | **已完成** |
| 部署侧三项 | open 不计分 |

## 验证门禁
- npm test / build / test:e2e
- go test ./...（无 Go 业务改动时仍跑）
- make e2e
