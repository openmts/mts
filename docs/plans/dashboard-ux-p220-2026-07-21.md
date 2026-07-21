# Dashboard UX P220（2026-07-21）

## P220 — ConfirmDialog 写门禁（离线/会话 critical）

### 背景
危险确认框打开后，若网络断开或会话进入 critical，确认按钮此前仍可能可点（仅靠 confirm 回调二次拦截，无 UI 反馈）。

### EARS
- [x] EARS-FE-P220-01 `ConfirmDialog` 支持 `writeBlocked` / `blockReason` / `offlineMessageKey`
- [x] EARS-FE-P220-02 阻断时禁用确认、展示 `confirm-dialog-blocked` 提示
- [x] EARS-FE-P220-03 接入：Query 范围删除、Users 批量/删除、DB 删除、Storage 删除、Downsample 删除/批量、Ops 危险确认
- [x] EARS-FE-P220-04 本地清理类对话框不接入写门禁（Query 清空历史、Ops 清空本地日志）
- [x] EARS-E2E-P220-05 商业冒烟：Query 删除确认打开后切离线 → 确认禁用 + blocked 横幅

### 非目标
- 宣称可商用完成
- 服务端 refresh token

### 验证
- [x] npm test / build / test:e2e
- [x] go test ./...
- [x] make e2e
