# Dashboard / UserGrantPanel 体验（2026-07-21 P150）

## 范围
- 授权库列表筛选、空态、计数
- 当前授权排序与 EmptyState
- 选择提示与授权按钮 disabled 态

## EARS
- [x] EARS-FE-P150-01 WHEN 库列表较长 THE SYSTEM SHALL 支持筛选
- [x] EARS-FE-P150-02 WHEN 无授权/无匹配库 THE SYSTEM SHALL 展示 EmptyState
- [x] EARS-FE-P150-03 WHEN 未选择库或权限 THE SYSTEM SHALL 禁用授权按钮
- [x] EARS-DOC-P150-04 WHEN 更新基线 THE SYSTEM SHALL 记录 P150

## 验证
- [x] `npm test`
- [x] `npm run build`
- [x] `npm run test:e2e`
- [x] `make e2e`
- [x] `go test -count=1 -timeout 120s ./...`
