# Dashboard / 运维可商用 EARS 清单（2026-07-20 P20）

## 范围
- 暴露 `GET /api/v1/admin/doctor` 结构化部署检查
- Overview 管理员面板展示 doctor 结果
- 边缘 HTTPS / HSTS 人工验收清单（纯数据 + Runbook）
- 文档与 production checklist 同步

## EARS
- [x] EARS-BE-P20-01 WHEN 管理员调用 `/api/v1/admin/doctor` THE SYSTEM SHALL 返回 data/backup/TLS/鉴权检查行
- [x] EARS-BE-P20-02 WHEN doctor 检查仅有 warn 无致命错误 THE SYSTEM SHALL 返回 HTTP 200 且 `ok=true` 并保留 warn 行
- [x] EARS-FE-P20-03 WHEN Overview 管理员刷新 THE SYSTEM SHALL 展示 doctor 检查结果列表
- [x] EARS-FE-P20-04 WHEN 维护边缘 HTTPS 验收 THE SYSTEM SHALL 提供可勾选验收步骤纯数据清单
- [x] EARS-DOC-P20-05 WHEN 更新可商用基线 THE SYSTEM SHALL 记录 P20 能力与仍未完成项

## 验证
- `go test ./cmd/mts-server -run 'Doctor|SecurityHeaders|Commercial' -count=1`
- `cd cmd/mts-dashboard && npm run test && npm run build`
- `make test` / `make e2e` / `make lint`

## 可商用仍未完成（不宣称目标完成）
- 真实反向代理证书与 HSTS 在生产环境的人工验收执行
- 跨主机异地拷贝与定时备份编排（部署侧）
- Storage UI 一键旁路恢复编排
