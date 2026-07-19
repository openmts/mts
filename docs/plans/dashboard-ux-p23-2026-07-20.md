# Dashboard / 运维可商用 EARS 清单（2026-07-20 P23）

## 范围
- 仓库级 `scripts/mts-backup.sh` 备份编排样例（data-snapshot / 异地 rsync / restore-drill / 保留清理）
- 脚本自检与 Make 入口
- 就绪中心快捷动作跳转 Storage 锚点
- 运维文档与基线同步

## EARS
- [x] EARS-OPS-P23-01 WHEN 运维配置 BASE_URL 与 Token THE SYSTEM SHALL 提供可执行的 data-snapshot 备份脚本
- [x] EARS-OPS-P23-02 WHEN 设置 REMOTE 目标 THE SYSTEM SHALL 支持 rsync 异地拷贝步骤
- [x] EARS-OPS-P23-03 WHEN 启用 restore-drill THE SYSTEM SHALL 调用旁路恢复 API 并在 fatal 时非 0 退出
- [x] EARS-FE-P23-04 WHEN 就绪中心点击快捷动作 THE SYSTEM SHALL 跳转 Storage 对应锚点
- [x] EARS-DOC-P23-05 WHEN 更新基线 THE SYSTEM SHALL 记录 P23 与仍未完成项

## 验证
- `bash -n scripts/mts-backup.sh && bash scripts/mts-backup-selfcheck.sh`
- `cd cmd/mts-dashboard && npm run test && npm run build`
- `make test` / `make e2e` / `make lint`

## 可商用仍未完成（不宣称目标完成）
- 真实边缘证书/HSTS 在生产代理的人工验收执行
- 目标环境 cron/systemd 的实际安装与演练记录归档
