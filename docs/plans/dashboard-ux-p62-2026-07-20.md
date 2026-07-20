# Dashboard / 运维可商用 EARS 清单（2026-07-20 P62）

## 范围
- AccessMatrix 角色分布/筛选文案使用 roleAdmin/roleUser 占位
- Metrics 样本计数与 labels/value 表头 i18n

## 边界
- 不改变指标解析与权限矩阵数据
- 不宣称生产验收完成

## EARS
- [x] EARS-FE-P62-01 WHEN 权限矩阵展示角色分布 THE SYSTEM SHALL 嵌入本地化角色名
- [x] EARS-FE-P62-02 WHEN 权限矩阵筛选角色 THE SYSTEM SHALL 使用本地化角色名
- [x] EARS-FE-P62-03 WHEN Metrics 展开指标族 THE SYSTEM SHALL 使用本地化样本计数与表头
- [x] EARS-FE-P62-04 WHEN 商业冒烟 e2e 运行 THE SYSTEM SHALL 覆盖 metrics 入口
- [x] EARS-DOC-P62-05 WHEN 更新基线 THE SYSTEM SHALL 记录 P62 与仍未完成项

## 验证
- `cd cmd/mts-dashboard && npm test && npm run build && npm run test:e2e`
- 仓库根：`make e2e` / `make lint` / `go test ./...`

## 可商用仍未完成（不宣称目标完成）
- 真实边缘证书/HSTS 人工验收执行
- 目标环境 cron/systemd 实装与演练归档
- 跨主机异地备份 + 告警通道真实跑通
