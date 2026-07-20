# Dashboard / Downsample create meta（2026-07-21 P148）

## 范围
- 创建降采样策略表单：db/measurement/field datalist 建议
- 打开创建面板时加载元数据；失败可手填

## EARS
- [x] EARS-FE-P148-01 WHEN 打开创建策略面板 THE SYSTEM SHALL 加载库列表建议
- [x] EARS-FE-P148-02 WHEN 填写 source db/measurement THE SYSTEM SHALL 加载 measurement/field 建议
- [x] EARS-FE-P148-03 WHEN 商业冒烟运行 THE SYSTEM SHALL 覆盖 create meta testid
- [x] EARS-DOC-P148-04 WHEN 更新基线 THE SYSTEM SHALL 记录 P148

## 验证
- [x] `npm test`
- [x] `npm run build`
- [x] `npm run test:e2e`（downsample create meta）
- [x] `make e2e`
- [x] `go test -count=1 -timeout 120s ./...`
