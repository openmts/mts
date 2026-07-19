# Dashboard / 运维可商用 EARS 清单（2026-07-20 P53）

## 范围
- 验收包 JSON/Markdown 纳入 export_preflight 摘要
- Overview 展示导出预检 warn/ok 摘要并跳转 #export-preflight
- 就绪中心预检区锚点 id=export-preflight

## 边界
- 预检不计分、不阻止导出、不宣称验收完成

## EARS
- [x] EARS-FE-P53-01 WHEN 导出验收包 THE SYSTEM SHALL 包含 export_preflight 摘要
- [x] EARS-FE-P53-02 WHEN 管理员打开 Overview THE SYSTEM SHALL 展示导出预检摘要
- [x] EARS-FE-P53-03 WHEN 用户点击打开导出预检 THE SYSTEM SHALL 进入就绪中心预检区
- [x] EARS-FE-P53-04 WHEN 商业冒烟 e2e 运行 THE SYSTEM SHALL 覆盖 Overview 预检入口
- [x] EARS-DOC-P53-05 WHEN 更新基线 THE SYSTEM SHALL 记录 P53 与仍未完成项

## 验证
- `cd cmd/mts-dashboard && npm test && npm run build && npm run test:e2e`
- 仓库根：`make test` / `make e2e` / `make lint`

## 可商用仍未完成（不宣称目标完成）
- 真实边缘证书/HSTS 人工验收执行
- 目标环境 cron/systemd 实装与演练归档
- 跨主机异地备份 + 告警通道真实跑通
