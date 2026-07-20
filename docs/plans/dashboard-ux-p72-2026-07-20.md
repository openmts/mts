# Dashboard / 运维可商用 EARS 清单（2026-07-20 P72）

## 范围
- 部署侧 runbook 联调清单（边缘证书、cron/systemd、异地备份+告警）
- 就绪中心展示联调步骤、复制/下载 Markdown
- 材料包下载时附带联调清单章节
- **不计就绪四维评分**；本地勾选 ≠ 验收完成

## 边界
- 不执行远程部署；不伪造验收完成
- 不把联调清单完成度计入 readiness score
- 不宣称生产验收完成

## EARS
- [x] EARS-FE-P72-01 WHEN 构建联调清单 THE SYSTEM SHALL 覆盖边缘 HTTPS/HSTS、cron/systemd、异地备份与告警三类部署侧项
- [x] EARS-FE-P72-02 WHEN 导出联调 Markdown THE SYSTEM SHALL 含 runbook 路径、验收命令提示与证据字段占位
- [x] EARS-FE-P72-03 WHEN 就绪中心打开部署材料区 THE SYSTEM SHALL 展示联调清单并可复制/下载
- [x] EARS-FE-P72-04 WHEN 下载部署材料包 THE SYSTEM SHALL 在文末附带联调清单章节
- [x] EARS-FE-P72-05 WHEN 商业冒烟 e2e 运行 THE SYSTEM SHALL 覆盖联调清单 testid
- [x] EARS-DOC-P72-06 WHEN 更新基线 THE SYSTEM SHALL 记录 P72 与仍未完成部署侧项

## 验证
- `cd cmd/mts-dashboard && npm test && npm run build && npm run test:e2e`
- 仓库根：`make e2e` / `make lint` / `go test ./...`

## 可商用仍未完成（不宣称目标完成）
- 真实边缘证书/HSTS 人工验收执行
- 目标环境 cron/systemd 实装与演练归档
- 跨主机异地备份 + 告警通道真实跑通
