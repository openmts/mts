# Dashboard / 运维可商用 EARS 清单（2026-07-20 P54）

## 范围
- 就绪归档 JSON/Markdown 纳入 export_preflight 摘要（与验收包对齐）
- 导出预检支持复制纯文本摘要
- Doctor 面板表头/摘要 i18n

## 边界
- 预检与复制不宣称生产验收完成
- 不改变就绪评分算法

## EARS
- [x] EARS-FE-P54-01 WHEN 导出就绪归档 THE SYSTEM SHALL 包含 export_preflight
- [x] EARS-FE-P54-02 WHEN 用户复制预检摘要 THE SYSTEM SHALL 写入本地化纯文本
- [x] EARS-FE-P54-03 WHEN 展示 Doctor 面板 THE SYSTEM SHALL 使用 i18n 表头与摘要
- [x] EARS-FE-P54-04 WHEN 商业冒烟 e2e 运行 THE SYSTEM SHALL 覆盖复制预检与 Doctor 入口
- [x] EARS-DOC-P54-05 WHEN 更新基线 THE SYSTEM SHALL 记录 P54 与仍未完成项

## 验证
- `cd cmd/mts-dashboard && npm test && npm run build && npm run test:e2e`
- 仓库根：`make test` / `make e2e` / `make lint`

## 可商用仍未完成（不宣称目标完成）
- 真实边缘证书/HSTS 人工验收执行
- 目标环境 cron/systemd 实装与演练归档
- 跨主机异地备份 + 告警通道真实跑通
