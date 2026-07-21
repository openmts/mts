# Dashboard UX P233（2026-07-21）

## 目标
补齐超时错误与写入取消的 e2e 覆盖。

## EARS
- [x] EARS-E2E-P233-01 查询 408/timeout mock 展示超时文案（error 样式）
- [x] EARS-E2E-P233-02 写入取消展示 `writeCancelled`（info 样式）
- [x] EARS-E2E-P233-03 route mock 用后 unroute

## 非目标
- 真实客户端计时超时（依赖 Vite 构建常量）
- 宣称可商用完成
