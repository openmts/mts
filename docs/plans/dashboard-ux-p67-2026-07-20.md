# Dashboard / 运维可商用 EARS 清单（2026-07-20 P67）

## 范围
- 确认对话框、快捷键帮助、通知关闭的无障碍标注
- 会话剩余徽章 live 区域；全局请求进度可感知
- 商业冒烟 e2e 覆盖键盘：命令面板 / 快捷键帮助 / skip-link

## 边界
- 不改变业务 API 与就绪评分规则
- 不计分部署验收项不变
- 不宣称生产验收完成

## EARS
- [x] EARS-FE-P67-01 WHEN 确认对话框打开 THE SYSTEM SHALL 提供 aria-describedby 与可测 data-testid，主/取消按钮可聚焦
- [x] EARS-FE-P67-02 WHEN 快捷键帮助打开 THE SYSTEM SHALL 关闭按钮具备 aria-label
- [x] EARS-FE-P67-03 WHEN 通知展示 THE SYSTEM SHALL 关闭按钮具备 aria-label
- [x] EARS-FE-P67-04 WHEN 会话剩余徽章可见 THE SYSTEM SHALL 使用 polite live 区域播报
- [x] EARS-FE-P67-05 WHEN 全局请求进行中 THE SYSTEM SHALL 暴露 progressbar 语义（busy 时）
- [x] EARS-FE-P67-06 WHEN 商业冒烟 e2e 运行 THE SYSTEM SHALL 覆盖命令面板、快捷键帮助与 skip-link 焦点
- [x] EARS-DOC-P67-07 WHEN 更新基线 THE SYSTEM SHALL 记录 P67 与仍未完成部署侧项

## 验证
- `cd cmd/mts-dashboard && npm test && npm run build && npm run test:e2e`
- 仓库根：`make e2e` / `make lint` / `go test ./...`

## 可商用仍未完成（不宣称目标完成）
- 真实边缘证书/HSTS 人工验收执行
- 目标环境 cron/systemd 实装与演练归档
- 跨主机异地备份 + 告警通道真实跑通
