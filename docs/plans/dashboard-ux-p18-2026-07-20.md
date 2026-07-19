# Dashboard 体验增强 EARS 清单（2026-07-20 P18）

## 范围（可商用：备份演练引导 + CI/Make 入口）
- Storage 页备份演练清单与进度
- `backupDrill` / `formatBytes` 纯函数与单测
- Makefile：`dashboard-test` / `dashboard-test-e2e` / `dashboard-test-e2e-install`
- 更新 runbook / 基线

## EARS
- [x] EARS-FE-P18-01 WHEN 管理员打开存储页 THE SYSTEM SHALL 展示备份演练清单与进度
- [x] EARS-FE-P18-02 WHEN 执行 validate/snapshot/export THE SYSTEM SHALL 自动勾选对应 Dashboard 可完成步骤
- [x] EARS-FE-P18-03 WHEN 主机侧步骤完成 THE SYSTEM SHALL 允许运维手工勾选异地拷贝/旁路恢复等项
- [x] EARS-DEV-P18-04 WHEN 本地或 CI 需要浏览器冒烟 THE SYSTEM SHALL 提供 `make dashboard-test-e2e` 与 install 目标
- [x] EARS-DOC-P18-05 WHEN 查阅生产基线 THE SYSTEM SHALL 将备份演练标记为部分自动化（Dashboard 清单）

## 验证
- `cd cmd/mts-dashboard && npm run test && npm run build`
- `make dashboard-test`
- `make test` / `make e2e` / `make lint`
- 可选：`make dashboard-test-e2e`

## 可商用仍未完成（不宣称目标完成）
- 边缘 HTTPS/HSTS 真实部署落地
- 完整旁路恢复自动化（需额外 data_dir 拉起与数据比对 harness）
