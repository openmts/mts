# Dashboard / Write meta 建议（2026-07-21 P147）

## 范围
- Write 页加载 measurement 列表与 field 建议（data 面）
- datalist + 芯片快捷填充（form 首行 / typed）
- 失败可手填

## 边界
- 不自动改用户已填内容（除芯片点击）
- 不做 series 写入选择器

## EARS
- [x] EARS-FE-P147-01 WHEN 选择 database THE SYSTEM SHALL 加载 measurement 建议
- [x] EARS-FE-P147-02 WHEN 选定 measurement THE SYSTEM SHALL 加载 field 建议
- [x] EARS-FE-P147-03 WHEN 点击建议芯片 THE SYSTEM SHALL 填充表单/Typed
- [x] EARS-FE-P147-04 WHEN 商业冒烟运行 THE SYSTEM SHALL 覆盖 write-meta-panel
- [x] EARS-DOC-P147-05 WHEN 更新基线 THE SYSTEM SHALL 记录 P147

## 验证
- [x] `npm test`
- [x] `npm run build`
- [x] `npm run test:e2e`（write-meta-panel）
- [x] `make e2e`
- [x] `go test -count=1 -timeout 120s ./...`
