# Dashboard / Storage·Query·About 导出对齐 EARS（2026-07-20 P82）

## 范围
- Storage：导出拉取 / 下载包装 JSON / 复制 JSON + testid
- Query：页面 testid；历史导出与结果 CSV 导出 testid
- About：构建信息 JSON 导出与复制
- 商业冒烟覆盖相关 testid

## 边界
- 不改服务端 storage/export、version 契约
- 不宣称部署侧验收完成

## EARS
- [x] EARS-FE-P82-01 WHEN 存储页拉取导出配置 THE SYSTEM SHALL 支持包装 JSON 下载与复制
- [x] EARS-FE-P82-02 WHEN 查询页可见 THE SYSTEM SHALL 暴露结果 CSV 与历史导出 testid
- [x] EARS-FE-P82-03 WHEN 关于页可见 THE SYSTEM SHALL 支持导出/复制客户端与服务端构建信息
- [x] EARS-FE-P82-04 WHEN 商业冒烟运行 THE SYSTEM SHALL 覆盖 storage/query/about 导出 testid
- [x] EARS-DOC-P82-05 WHEN 更新基线 THE SYSTEM SHALL 记录 P82

## 实现备注
- 纯函数：`storageExport` / `aboutExport`
- testid：`storage-export-fetch|download|copy`、`query-page`、`query-export-csv|history`、`about-export-json|copy`

## 验证
- `cd cmd/mts-dashboard && npm test && npm run build && npm run test:e2e`
- 仓库根：`make e2e` / `make lint` / `go test ./...`
