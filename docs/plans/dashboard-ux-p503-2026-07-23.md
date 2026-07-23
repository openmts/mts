# Dashboard UX / Server P503 — healthz/readyz path

## 目标
GET /healthz 与 /readyz 返回 path，并保留 healthy/ready 顶层字段。

## 验收
- [x] healthProbeResponse + 单测
- [x] operation_registry ResponseHint 更新
- [x] 清单 healthz-readyz-path
- [x] go test / dashboard 验证
