# Dashboard UX P211（2026-07-21）

## P211 — Storage 导出/校验离线门禁 + 登录离线 e2e

- [x] EARS-FE-P211-01 Storage `doExport` 离线拦截
- [x] EARS-FE-P211-02 Storage validate/snapshot/export/data-snapshot/restore 按钮离线禁用
- [x] EARS-E2E-P211-03 商业冒烟：storage offline 禁用 export/validate
- [x] EARS-E2E-P211-04 商业冒烟：login offline 禁用 submit

## 非目标
- 服务端 refresh token / 边缘证书 / cron
- 宣称可商用完成

## 验证
- [x] npm test / build / test:e2e
- [x] go test ./...
- [x] make e2e
