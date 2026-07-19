# Dashboard / 运维可商用 EARS 清单（2026-07-20 P24）

## 范围
- 就绪评分纳入 doctor warn/ok 与 TLS 状态（纯函数可单测）
- Overview 入口跳转就绪中心
- CI 主门禁纳入 `backup-script-check`
- 备份脚本补充登录取 Token 示例
- 文档与基线同步

## EARS
- [x] EARS-FE-P24-01 WHEN 计算就绪评分 THE SYSTEM SHALL 融合必做清单、HTTPS、备份编排与 doctor 状态
- [x] EARS-FE-P24-02 WHEN doctor 存在 warn 或加载失败 THE SYSTEM SHALL 降低就绪评分并展示原因
- [x] EARS-FE-P24-03 WHEN Overview 管理员查看 THE SYSTEM SHALL 提供进入就绪中心的入口
- [x] EARS-OPS-P24-04 WHEN 运行 CI 门禁 THE SYSTEM SHALL 执行备份脚本自检
- [x] EARS-DOC-P24-05 WHEN 更新基线 THE SYSTEM SHALL 记录 P24 与仍未完成项

## 验证
- `cd cmd/mts-dashboard && npm run test && npm run build`
- `make backup-script-check`
- `make test` / `make e2e` / `make lint`

## 可商用仍未完成（不宣称目标完成）
- 真实边缘证书/HSTS 在生产代理的人工验收执行
- 目标环境 cron/systemd 实际安装与演练归档
