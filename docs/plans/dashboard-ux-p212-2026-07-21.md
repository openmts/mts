# Dashboard UX P212（2026-07-21）

## P212 — Ops 卡片与 Downsample 批量离线禁用

- [x] EARS-FE-P212-01 Operations flush/compact/retention 离线禁用 + title 提示
- [x] EARS-FE-P212-02 Downsample 批量 enable/disable 离线禁用
- [x] EARS-FE-P212-03 openBatch 离线拦截
- [x] EARS-E2E-P212-04 商业冒烟：ops 卡片离线禁用

## 非目标
- 服务端 refresh token / 边缘证书 / cron
- 宣称可商用完成

## 验证
- [x] npm test / build / test:e2e
- [x] go test ./...
- [x] make e2e
