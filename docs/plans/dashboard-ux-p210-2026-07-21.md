# Dashboard UX P210（2026-07-21）

## P210 — Config 服务 Token 脏离开守卫 + 商业冒烟加深

- [x] EARS-FE-P210-01 Config admin/data token 相对 baseline 变更时 dirty badge
- [x] EARS-FE-P210-02 routeDirty + beforeunload 在 token 未保存时生效
- [x] EARS-FE-P210-03 保存/清除后 baseline 同步并清除 dirty
- [x] EARS-FE-P210-04 Token 输入区 testid：panel/admin/data/save/clear
- [x] EARS-E2E-P210-05 商业冒烟：config-token-dirty-badge
- [x] EARS-E2E-P210-06 商业冒烟加深：account dirty + overview/about/write 导出 banner

## 非目标
- 服务端 refresh token / 边缘证书 / cron
- 宣称可商用完成

## 验证
- [x] npm test / build / test:e2e
- [x] go test ./...（pprof/storage_engine 偶发 flaky，全量重跑通过）
- [x] make e2e
