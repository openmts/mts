# Dashboard UX EARS P151–P155（2026-07-21）

来源：`docs/review/code-review-2026-07-21-0216-mts-dashboard-full.md`

## 边界
- 不宣称可商用完成；部署侧三项仍 open
- 单机 POC；不引入 SQL parser / expr 树 UI
- series 分页仅 limit/total/truncated（无 cursor）

## P151 — 非 admin 只读库浏览器
- [x] EARS-FE-P151-01 WHEN 非 admin 访问 `/databases` THE SYSTEM SHALL 使用 data 面列出可读库并允许展开 measurement 元数据
- [x] EARS-FE-P151-02 WHEN 非 admin THE SYSTEM SHALL 隐藏创建/删除库与创建 RP，并展示只读提示
- [x] EARS-FE-P151-03 WHEN admin THE SYSTEM SHALL 保持完整管理能力
- [x] EARS-FE-P151-04 WHEN 商业冒烟运行 THE SYSTEM SHALL 覆盖 databases-page 在 admin 下的原有路径
- [x] EARS-DOC-P151-05 WHEN 更新基线 THE SYSTEM SHALL 记录 P151

## P152 — Query series 服务端 tag 过滤
- [x] EARS-FE-P152-01 WHEN 加载 measurement meta 且 tags 可解析 THE SYSTEM SHALL 将 tags 传给 listSeries
- [x] EARS-FE-P152-02 WHEN 用户点击「按 tags 刷新 series」THE SYSTEM SHALL 用当前 tags 重新拉取
- [x] EARS-FE-P152-03 WHEN tags 非法 THE SYSTEM SHALL 回退无过滤拉取并提示
- [x] EARS-DOC-P152-04 WHEN 更新基线 THE SYSTEM SHALL 记录 P152

## P153 — 库页 series 截断
- [x] EARS-FE-P153-01 WHEN measurement 展开 series 超过上限 THE SYSTEM SHALL 截断展示并提示 total
- [x] EARS-FE-P153-02 WHEN series 为空 THE SYSTEM SHALL 保持空态文案
- [x] EARS-DOC-P153-03 WHEN 更新基线 THE SYSTEM SHALL 记录 P153

## P154 — Users 授权面板 e2e
- [x] EARS-FE-P154-01 WHEN 用户行可打开授权 THE SYSTEM SHALL 暴露 `users-open-grant-*` testid
- [x] EARS-FE-P154-02 WHEN 商业冒烟 THE SYSTEM SHALL 打开 grant 面板并校验 filter/close
- [x] EARS-DOC-P154-03 WHEN 更新基线 THE SYSTEM SHALL 记录 P154

## P155 — series limit/total API
- [x] EARS-BE-P155-01 WHEN series 请求含 limit THE SYSTEM SHALL 返回截断列表与 total/truncated
- [x] EARS-BE-P155-02 WHEN query 含 limit/offset 等保留字 THE SYSTEM SHALL 不将其当作 tag
- [x] EARS-FE-P155-03 WHEN 前端 listSeries 传 limit THE SYSTEM SHALL 使用服务端 total
- [x] EARS-DOC-P155-04 WHEN 更新基线 THE SYSTEM SHALL 记录 P155

## 验证门禁
- `cd cmd/mts-dashboard && npm test && npm run build && npm run test:e2e`
- `make e2e`
- `timeout 180s env GOSUMDB=sum.golang.org go test -count=1 -timeout 120s ./...`
- Go 改动：`goimports-reviser` + `golangci-lint ./cmd/mts-server/...`
