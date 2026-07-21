# 代码检视：mts-dashboard P284（2026-07-21）

## 问题
- `storage` 事件后仅 `syncFromStorage` 读 getter，但 client 内存 bearer/expires 未从 localStorage 重载，他页登出/续期后本页仍可能用旧 token 发请求。

## 修复
- 新增 `reloadAuthFromStorage` / `readAuthStorageSnapshot`
- `syncFromStorage` 先重载内存
- TopBar 临界时钟与 mutation guard 对齐
- 离线横幅可手动 recheck 网络状态与 readyz

## 状态
| 项 | 状态 |
|----|------|
| P284 | 待验证合入 |
