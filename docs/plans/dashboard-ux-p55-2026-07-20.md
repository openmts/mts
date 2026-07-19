# Dashboard / 运维可商用 EARS 清单（2026-07-20 P55）

## 范围
- Overview 健康态 / Ready 文案与 Doctor 表头 i18n
- 就绪评分分项 Doctor 标签与评分等级文案 i18n
- 空值占位统一 `emptyValue`
- 预检/签核「建议下一步」面板（Overview + Readiness）
- 导出归档/验收包后附带预检摘要 toast（不计分）

## 边界
- 不改变就绪四维评分算法
- 下一步建议与 toast 不宣称生产验收完成
- 部署侧证书/cron/异地备份仍为人工项

## EARS
- [x] EARS-FE-P55-01 WHEN Overview 展示 healthy/ready THE SYSTEM SHALL 使用 i18n（含 unhealthy/notReady/emptyValue）
- [x] EARS-FE-P55-02 WHEN Overview 展示 Doctor 表 THE SYSTEM SHALL 使用 readinessDoctorCol* 表头
- [x] EARS-FE-P55-03 WHEN 展示就绪评分分项 THE SYSTEM SHALL 使用 readinessScoreDoctor 而非硬编码 Doctor
- [x] EARS-FE-P55-04 WHEN 展示评分等级 THE SYSTEM SHALL 本地化 good/warn/bad
- [x] EARS-FE-P55-05 WHEN 预检存在缺口 THE SYSTEM SHALL 在 Overview/Readiness 给出建议下一步并可跳转
- [x] EARS-FE-P55-06 WHEN 导出归档或验收包成功 THE SYSTEM SHALL toast 预检 warn/info/ok 摘要
- [x] EARS-FE-P55-07 WHEN 商业冒烟 e2e 运行 THE SYSTEM SHALL 覆盖下一步建议入口
- [x] EARS-DOC-P55-08 WHEN 更新基线 THE SYSTEM SHALL 记录 P55 与仍未完成项

## 验证
- `cd cmd/mts-dashboard && npm test && npm run build && npm run test:e2e`
- 仓库根：`make e2e` / `make lint` / `go test ./...`

## 可商用仍未完成（不宣称目标完成）
- 真实边缘证书/HSTS 人工验收执行
- 目标环境 cron/systemd 实装与演练归档
- 跨主机异地备份 + 告警通道真实跑通
