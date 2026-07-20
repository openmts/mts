# Dashboard / 运维可商用 EARS 清单（2026-07-20 P79）

## 范围
- Config：导出生效配置 / Schema / 错误码；复制生效配置
- Operations：导出与复制 maintenance errors
- 统一 download + stampFilename / clipboard 工具

## 边界
- 不改服务端配置契约
- 不宣称部署侧验收完成

## EARS
- [x] EARS-FE-P79-01 WHEN 配置页有生效配置 THE SYSTEM SHALL 支持导出 JSON 与复制文本
- [x] EARS-FE-P79-02 WHEN 配置页有 schema/错误码 THE SYSTEM SHALL 支持分别导出 JSON
- [x] EARS-FE-P79-03 WHEN 运维页有维护错误 THE SYSTEM SHALL 支持导出 JSON 与复制纯文本
- [x] EARS-FE-P79-04 WHEN 商业冒烟运行 THE SYSTEM SHALL 覆盖 config/ops 导出 testid
- [x] EARS-DOC-P79-05 WHEN 更新基线 THE SYSTEM SHALL 记录 P79

## 验证
- `cd cmd/mts-dashboard && npm test && npm run build && npm run test:e2e`
- 仓库根：`make e2e` / `make lint` / `go test ./...`
