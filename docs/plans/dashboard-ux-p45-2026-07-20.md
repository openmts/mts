# Dashboard / 运维可商用 EARS 清单（2026-07-20 P45）

## 范围
- 验收包 JSON/Markdown 纳入 `deploy_kit` 索引摘要（样例 id/title/filename，不含全文）
- Overview 就绪卡片增加「打开部署材料包」入口（hash `#deploy-kit`）
- 就绪中心部署材料包锚点与自动滚动
- 文档同步

## 边界
- 不把复制/下载记为部署验收完成
- 不在 Dashboard 自动安装证书或启用 cron/systemd

## EARS
- [x] EARS-FE-P45-01 WHEN 导出验收包 THE SYSTEM SHALL 包含 deploy_kit 摘要且 manual_signoff_required=true
- [x] EARS-FE-P45-02 WHEN 验收包语言为 en THE SYSTEM SHALL 输出英文部署材料包索引标题
- [x] EARS-FE-P45-03 WHEN 管理员在 Overview 点击部署材料包入口 THE SYSTEM SHALL 跳转就绪中心并定位 deploy-kit
- [x] EARS-FE-P45-04 WHEN 商业冒烟 e2e 运行 THE SYSTEM SHALL 覆盖 Overview 部署材料包入口
- [x] EARS-DOC-P45-05 WHEN 更新基线 THE SYSTEM SHALL 记录 P45 与仍未完成项

## 验证
- `cd cmd/mts-dashboard && npm test && npm run build`
- `cd cmd/mts-dashboard && npm run test:e2e`
- 仓库根：`make test` / `make e2e` / `make lint`

## 可商用仍未完成（不宣称目标完成）
- 真实边缘证书/HSTS 人工验收执行
- 目标环境 cron/systemd 实装与演练归档
- 跨主机异地备份 + 告警通道真实跑通
