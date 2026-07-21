# Dashboard UX P214（2026-07-21）

## P214 — Downsample 行级写操作与 Query 范围删除离线禁用

- [x] EARS-FE-P214-01 Downsample 创建入口离线禁用
- [x] EARS-FE-P214-02 行级 run/range/reset/toggle/delete 离线禁用 + open 拦截
- [x] EARS-FE-P214-03 range confirm 离线禁用
- [x] EARS-FE-P214-04 Query 范围删除按钮离线禁用
- [x] EARS-E2E-P214-05 商业冒烟：downsample-open-create 离线禁用

## 非目标
- 服务端 refresh token / 边缘证书 / cron
- 宣称可商用完成

## 验证
- [x] npm test / build / test:e2e
- [x] go test ./...
- [x] make e2e
