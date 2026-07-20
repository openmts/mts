# Dashboard / Account landing EmptyState（2026-07-21 P144）

## 范围
- Account 落地页选项列表化：筛选、分组、当前态、空态
- 保留原生 select 降级
- 商业冒烟覆盖 filter empty

## 边界
- 不改 landing 存储契约

## EARS
- [x] EARS-FE-P144-01 WHEN 有落地选项 THE SYSTEM SHALL 列表展示并支持筛选
- [x] EARS-FE-P144-02 WHEN 筛选无匹配 THE SYSTEM SHALL 展示 EmptyState
- [x] EARS-FE-P144-03 WHEN 选择列表项 THE SYSTEM SHALL 保存落地偏好
- [x] EARS-FE-P144-04 WHEN 商业冒烟运行 THE SYSTEM SHALL 覆盖 landing empty
- [x] EARS-DOC-P144-05 WHEN 更新基线 THE SYSTEM SHALL 记录 P144

## 验证
- [x] `npm test`（landingOptionsView）
- [x] `npm run build`
- [x] `npm run test:e2e`
- [x] `make e2e`
- [x] `go test -count=1 -timeout 120s ./...`
