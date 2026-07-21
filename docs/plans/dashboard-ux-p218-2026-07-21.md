# Dashboard UX P218（2026-07-21）

## P218 — Users 弹窗/授权面板会话 critical 文案对齐

### 背景
P215 将 `UsersPage` 的 `:offline` 传为 `writeBlocked`，但 `UserModals` / `UserGrantPanel` 的 title 仍写死 `offlineAdminBlocked`，会话 critical 时会误提示「离线」。

### EARS
- [x] EARS-FE-P218-01 `UserModals` 支持 `writeBlocked` + `blockReason`，title 区分 offline/session
- [x] EARS-FE-P218-02 `UserGrantPanel` 同上（revoke/grant）
- [x] EARS-FE-P218-03 `UsersPage` 传入 `writeBlocked`/`blockReason`（保留 offline 兼容 prop）
- [x] EARS-E2E-P218-04 商业冒烟：session critical 下 users-create-open title 非离线语义

### 非目标
- 宣称可商用完成
- 服务端 refresh token

### 验证
- [x] npm test / build / test:e2e
- [x] go test ./...
- [x] make e2e
