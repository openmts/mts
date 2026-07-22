# Dashboard UX P488 — Storage/Overview/Downsample path 对齐

## 目标
在 path 契约收口后，把 Storage 演练、Overview 运维扫视、Downsample run/dry-run 的 path 可观测性补齐，并修正 gRPC run 路径。

## 范围
- Server：gRPC downsample run/run-range/repair path
- Dashboard：Storage/Overview/Downsample path 徽章与成功文案
- 清单/e2e：`storage-overview-path`

## 验收
- [x] gRPC run 响应含 path
- [x] Storage/Overview/Downsample 徽章
- [x] npm test / build / commercial-smoke
