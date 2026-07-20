# Dashboard / 剩余列表虚拟化 EARS（2026-07-21 P137）

## 范围
- ApiSpec 端点表 VirtualTable + 空态
- Overview health/doctor/maint VirtualTable
- Readiness doctor checks VirtualTable + 空态

## 边界
- 不改 API 契约与评分
- 有数据时 e2e 软断言

## EARS
- [x] EARS-FE-P137-01 ApiSpec 端点虚拟滚动
- [x] EARS-FE-P137-02 Overview health/doctor/maint 虚拟滚动
- [x] EARS-FE-P137-03 Readiness doctor 虚拟滚动
- [x] EARS-FE-P137-04 商业冒烟覆盖（有数据时）
- [x] EARS-DOC-P137-05 基线记录

## 验证
- npm test && npm run build && npm run test:e2e ✅
- make e2e + go test ./... ✅
