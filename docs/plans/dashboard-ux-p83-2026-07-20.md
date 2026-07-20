# Dashboard / Overview·Write 导出对齐 EARS（2026-07-20 P83）

## 范围
- Overview：运维快照 JSON 导出与复制；页面 testid
- Write：写入结果 JSON 导出、草稿 JSON 导出；提交/结果 testid
- 商业冒烟覆盖相关 testid

## 边界
- 不改服务端 health/write 契约
- 不宣称部署侧验收完成
- 导出不计入 readiness 四维总分

## EARS
- [x] EARS-FE-P83-01 WHEN 概览页已加载 THE SYSTEM SHALL 支持导出与复制运维快照 JSON
- [x] EARS-FE-P83-02 WHEN 写入页有结果 THE SYSTEM SHALL 支持导出写入结果 JSON
- [x] EARS-FE-P83-03 WHEN 写入页有表单内容 THE SYSTEM SHALL 支持导出草稿 JSON
- [x] EARS-FE-P83-04 WHEN 商业冒烟运行 THE SYSTEM SHALL 覆盖 overview/write 导出 testid
- [x] EARS-DOC-P83-05 WHEN 更新基线 THE SYSTEM SHALL 记录 P83

## 实现备注
- 纯函数：`overviewExport` / `writeExport`
- testid：`overview-page`、`overview-export-json`、`overview-copy-snapshot`、`write-page`、`write-export-result|draft`、`write-submit`、`write-result-ok`

## 验证
- `cd cmd/mts-dashboard && npm test && npm run build && npm run test:e2e`
- 仓库根：`make e2e` / `make lint` / `go test ./...`
