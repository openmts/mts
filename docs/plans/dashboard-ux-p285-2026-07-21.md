# Dashboard UX P285（2026-07-21）

## 目标
- 标签页重新可见时：重载会话内存；布局侧同步网络与 /readyz

## 验收
- [x] `shouldSyncOnVisibility` 单测
- [x] main.ts visibilitychange -> syncFromStorage/ensureSession
- [x] DashboardLayout visibility -> network + readyz
- [x] e2e 不因 visibility 事件掉登录
- [x] npm test / build / e2e 通过后合入
