# Dashboard / 运维可商用 EARS 清单（2026-07-20 P36）

## 范围
- 键盘快捷键帮助（? / 顶栏按钮）
- 最近访问路由条（sessionStorage）
- TopBar 标题复用 pageTitle 映射
- 文档同步

## 边界
- 最近访问不记录登录/强制改密页；最多 8 条
- 快捷键帮助在输入框内不拦截 `?`
- 仍不宣称可商用完成

## EARS
- [x] EARS-FE-P36-01 WHEN 用户按 ?（非输入态）或点击顶栏快捷键按钮 THE SYSTEM SHALL 打开快捷键帮助并陷阱焦点
- [x] EARS-FE-P36-02 WHEN 用户在控制台内导航 THE SYSTEM SHALL 记录最近访问路径并展示可点击条
- [x] EARS-FE-P36-03 WHEN TopBar 展示页标题 THE SYSTEM SHALL 与 document.title 使用同一路由映射
- [x] EARS-DOC-P36-04 WHEN 更新基线 THE SYSTEM SHALL 记录 P36 与仍未完成项

## 验证
- `cd cmd/mts-dashboard && npm run test && npm run build`
- `make test` / `make e2e` / `make lint`

## 可商用仍未完成（不宣称目标完成）
- 真实边缘证书/HSTS 人工验收执行
- 目标环境 cron/systemd 实装与演练归档
- 跨主机异地备份 + 告警通道真实跑通
