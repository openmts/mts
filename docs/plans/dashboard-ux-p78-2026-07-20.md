# Dashboard / 运维可商用 EARS 清单（2026-07-20 P78）

## 范围
- Metrics 页：自动刷新、原始/JSON 导出、展开折叠、testid/a11y

## 边界
- 不改 `/metrics` 服务契约
- 不宣称部署侧验收完成

## EARS
- [x] EARS-FE-P78-01 WHEN 打开 Metrics THE SYSTEM SHALL 提供自动刷新间隔选项
- [x] EARS-FE-P78-02 WHEN 导出指标 THE SYSTEM SHALL 支持 raw 与过滤后 JSON
- [x] EARS-FE-P78-03 WHEN 过滤结果存在 THE SYSTEM SHALL 支持全部展开/折叠
- [x] EARS-FE-P78-04 WHEN 商业冒烟访问 metrics THE SYSTEM SHALL 覆盖关键 testid
- [x] EARS-DOC-P78-05 WHEN 更新基线 THE SYSTEM SHALL 记录 P78

## 验证
- `cd cmd/mts-dashboard && npm test && npm run build && npm run test:e2e`
- 仓库根：`make e2e` / `make lint` / `go test ./...`
