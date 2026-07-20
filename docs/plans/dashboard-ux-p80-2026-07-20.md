# Dashboard / Access·API Spec 可商用 EARS 清单（2026-07-20 P80）

## 范围
- 实时授权（`/access/grants`）：JSON/CSV 导出
- 权限矩阵（`/access`）：矩阵 JSON 导出（当前筛选结果 + 界面语言）
- API Spec（`/api-spec`）：JSON + Markdown 导出；命名空间标签 i18n
- 统一 `download` + `stampFilename`；商业冒烟覆盖 testid

## 边界
- 不改服务端 api-spec / 授权契约
- 不宣称部署侧验收完成
- 不将导出能力计入 readiness 四维总分

## EARS
- [x] EARS-FE-P80-01 WHEN 实时授权页有筛选结果 THE SYSTEM SHALL 支持导出 JSON 与 CSV
- [x] EARS-FE-P80-02 WHEN 权限矩阵有可见行 THE SYSTEM SHALL 支持按当前语言导出矩阵 JSON
- [x] EARS-FE-P80-03 WHEN API Spec 已加载 THE SYSTEM SHALL 支持导出 JSON 与 Markdown
- [x] EARS-FE-P80-04 WHEN API Spec 展示命名空间筛选 THE SYSTEM SHALL 使用 i18n 标签而非硬编码 Namespace
- [x] EARS-FE-P80-05 WHEN 商业冒烟运行 THE SYSTEM SHALL 覆盖 access/api-spec 导出 testid
- [x] EARS-DOC-P80-06 WHEN 更新基线 THE SYSTEM SHALL 记录 P80

## 实现备注
- 纯函数：`grantsExport` / `accessMatrixExport` / `apiSpecExport`
- testid：`access-grants-export-json|csv`、`access-matrix-export`、`api-spec-export-json|md`

## 验证
- `cd cmd/mts-dashboard && npm test && npm run build && npm run test:e2e`
- 仓库根：`make e2e` / `make lint` / `go test ./...`
