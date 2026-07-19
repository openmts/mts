# Dashboard / 运维可商用 EARS 清单（2026-07-20 P39）

## 范围
- Metrics / AccessMatrix / AccessGrants / ApiSpec / NotFound 用户可见文案 i18n
- 权限矩阵等级标签随语言切换
- Playwright 冒烟加深（矩阵/授权/指标/404）
- 文档同步

## 边界
- 矩阵 capability 行数据中的中文产品语义暂保留（数据层双语可后续专项）
- 仍不宣称可商用完成

## EARS
- [x] EARS-FE-P39-01 WHEN 语言为 en THE SYSTEM SHALL 展示指标浏览页英文文案
- [x] EARS-FE-P39-02 WHEN 语言为 en THE SYSTEM SHALL 展示权限矩阵/实时授权页英文壳层文案
- [x] EARS-FE-P39-03 WHEN 访问未知路由 THE SYSTEM SHALL 展示 i18n 化 404 空态
- [x] EARS-FE-P39-04 WHEN 商业冒烟 e2e 运行 THE SYSTEM SHALL 覆盖矩阵、授权、指标与 404
- [x] EARS-DOC-P39-05 WHEN 更新基线 THE SYSTEM SHALL 记录 P39 与仍未完成项

## 验证
- `cd cmd/mts-dashboard && npm run test && npm run build`
- `make test` / `make e2e` / `make lint`

## 可商用仍未完成（不宣称目标完成）
- 真实边缘证书/HSTS 人工验收执行
- 目标环境 cron/systemd 实装与演练归档
- 跨主机异地备份 + 告警通道真实跑通
- 权限矩阵 capability 行全文双语（可选后续）
