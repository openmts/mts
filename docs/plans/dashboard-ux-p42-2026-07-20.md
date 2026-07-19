# Dashboard / 运维可商用 EARS 清单（2026-07-20 P42）

## 范围
- 抽取共享 `localizedText`（`LocalizedText` / `textForLocale`）
- 生产清单 / 备份演练 / 边缘 HTTPS / 备份编排 数据层双语
- Readiness / Storage 展示随 locale 切换
- 单元测试与 Playwright 覆盖清单标题双语
- 文档同步

## 边界
- 不改变清单 id、severity、自动化标记与就绪评分算法
- 仍不宣称可商用完成（边缘证书/HSTS 人工验收、cron/systemd、跨主机备份实装）

## EARS
- [x] EARS-FE-P42-01 WHEN 语言为 en THE SYSTEM SHALL 展示生产清单 title/detail 英文文案
- [x] EARS-FE-P42-02 WHEN 语言为 en THE SYSTEM SHALL 展示备份演练与边缘 HTTPS 步骤英文文案
- [x] EARS-FE-P42-03 WHEN 语言为 en THE SYSTEM SHALL 展示备份编排步骤英文文案
- [x] EARS-FE-P42-04 WHEN 运行相关单元测试 THE SYSTEM SHALL 校验全部清单行含 zh/en
- [x] EARS-FE-P42-05 WHEN 商业冒烟 e2e 运行 THE SYSTEM SHALL 覆盖就绪清单语言切换
- [x] EARS-DOC-P42-06 WHEN 更新基线 THE SYSTEM SHALL 记录 P42 与仍未完成项

## 验证
- `cd cmd/mts-dashboard && npm test && npm run build`
- `cd cmd/mts-dashboard && npm run test:e2e`
- 仓库根：`make test` / `make e2e` / `make lint`

## 可商用仍未完成（不宣称目标完成）
- 真实边缘证书/HSTS 人工验收执行
- 目标环境 cron/systemd 实装与演练归档
- 跨主机异地备份 + 告警通道真实跑通
