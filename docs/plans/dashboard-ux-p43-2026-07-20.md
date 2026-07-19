# Dashboard / 运维可商用 EARS 清单（2026-07-20 P43）

## 范围
- 就绪归档 / 验收包按当前 UI locale 序列化清单 title·detail
- Markdown 壳层文案双语；JSON 增加 `locale` 与 `catalog`
- 导出入口透传 `uiLocale`
- 文档同步

## 边界
- 不改变清单 id / 评分算法；`checklist.*` 仍输出稳定 id 列表
- 仍不宣称可商用完成（边缘证书/HSTS 人工验收、cron/systemd、跨主机备份）

## EARS
- [x] EARS-FE-P43-01 WHEN 导出就绪归档且语言为 en THE SYSTEM SHALL 在 JSON catalog 中写入英文 title/detail
- [x] EARS-FE-P43-02 WHEN 导出验收包且语言为 en THE SYSTEM SHALL 生成英文 Markdown 壳层与 catalog 摘要
- [x] EARS-FE-P43-03 WHEN 导出归档 THE SYSTEM SHALL 同时保留稳定 checklist id 列表
- [x] EARS-FE-P43-04 WHEN 运行相关单元测试 THE SYSTEM SHALL 覆盖 zh/en 归档与验收包
- [x] EARS-DOC-P43-05 WHEN 更新基线 THE SYSTEM SHALL 记录 P43 与仍未完成项

## 验证
- `cd cmd/mts-dashboard && npm test && npm run build`
- `cd cmd/mts-dashboard && npm run test:e2e`
- 仓库根：`make test` / `make e2e` / `make lint`

## 可商用仍未完成（不宣称目标完成）
- 真实边缘证书/HSTS 人工验收执行
- 目标环境 cron/systemd 实装与演练归档
- 跨主机异地备份 + 告警通道真实跑通
