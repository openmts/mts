# Dashboard UX P208（2026-07-21）

## P208 — Account 改密/续期离线门禁 + 改密脏离开守卫

- [x] EARS-FE-P208-01 账户改密提交在离线时拦截并提示 `offlineAccountBlocked`
- [x] EARS-FE-P208-02 会话密码续期在离线时拦截并提示
- [x] EARS-FE-P208-03 改密表单非空时 dirty badge + routeDirty + beforeunload
- [x] EARS-FE-P208-04 离线时禁用改密/续期提交按钮

## 非目标
- 服务端独立 refresh token
- 宣称可商用完成

## 验证
- [x] npm test / build / test:e2e
- [x] go test ./...（pprof/storage_engine 首次 flaky 重跑通过）
- [x] make e2e
