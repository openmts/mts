# Dashboard / 运维可商用 EARS 清单（2026-07-20 P77）

## 范围
- 审计 JSON/CSV 导出统一 `download` + `stampFilename`
- 查询结果 CSV / 历史 JSON 导出统一下载工具
- `csv.ts` 不再自实现 createObjectURL 下载

## 边界
- 不改导出字段语义；不宣称部署侧验收完成

## EARS
- [x] EARS-FE-P77-01 WHEN 导出审计 JSON/CSV THE SYSTEM SHALL 使用统一下载工具与时间戳文件名
- [x] EARS-FE-P77-02 WHEN 导出查询 CSV/历史 JSON THE SYSTEM SHALL 使用统一下载工具与时间戳文件名
- [x] EARS-FE-P77-03 WHEN 构建审计 CSV THE SYSTEM SHALL 正确转义逗号与引号
- [x] EARS-DOC-P77-04 WHEN 更新基线 THE SYSTEM SHALL 记录 P77

## 验证
- `cd cmd/mts-dashboard && npm test && npm run build && npm run test:e2e`
- 仓库根：`make e2e` / `make lint` / `go test ./...`
