# Dashboard UX P209（2026-07-21）

## P209 — 登录/强制改密离线门禁与强制改密脏守卫

- [x] EARS-FE-P209-01 登录在离线时拦截并提示 `offlineLoginBlocked`
- [x] EARS-FE-P209-02 登录提交按钮离线禁用
- [x] EARS-FE-P209-03 强制改密离线拦截 + 按钮禁用
- [x] EARS-FE-P209-04 强制改密表单 dirty badge + routeDirty + beforeunload

## 非目标
- 服务端 refresh token / 边缘证书
- 宣称可商用完成

## 验证
- [x] npm test / build / test:e2e
- [x] go test ./...
- [x] make e2e
