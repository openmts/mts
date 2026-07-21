# Dashboard UX P284（2026-07-21）

## 目标
- 多标签会话：他页改 localStorage 后本页内存 token/用户/过期时间正确重载
- TopBar 会话徽章时钟 critical 下 1s 刷新
- 离线横幅提供「重新检测」入口

## 验收
- [x] `reloadAuthFromStorage` + `syncFromStorage` 先重载内存
- [x] `authStorageSync` 单测
- [x] main storage 监听仅处理会话 key
- [x] TopBar `sessionClockTickMs` 自适应
- [x] offline-banner-retry 可见
- [x] npm test / build / e2e 通过后合入
