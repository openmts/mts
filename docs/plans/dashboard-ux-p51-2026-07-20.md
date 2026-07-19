# Dashboard / 运维可商用 EARS 清单（2026-07-20 P51）

## 范围
- 就绪中心导出前预检清单（清单/HTTPS/备份/Doctor/签核/部署材料查阅）
- Readiness/Storage 同页 hash 变更监听与滚动
- e2e 覆盖预检入口

## 边界
- 预检不阻止导出，不计入就绪评分，不宣称验收完成
- hash 滚动仅 UI 定位

## EARS
- [x] EARS-FE-P51-01 WHEN 打开就绪中心 THE SYSTEM SHALL 展示导出前预检清单
- [x] EARS-FE-P51-02 WHEN 签核备注缺失 THE SYSTEM SHALL 在预检中以 warn 展示
- [x] EARS-FE-P51-03 WHEN 同页 hash 变化 THE SYSTEM SHALL 滚动到对应锚点
- [x] EARS-FE-P51-04 WHEN 商业冒烟 e2e 运行 THE SYSTEM SHALL 覆盖预检入口
- [x] EARS-DOC-P51-05 WHEN 更新基线 THE SYSTEM SHALL 记录 P51 与仍未完成项

## 验证
- `cd cmd/mts-dashboard && npm test && npm run build && npm run test:e2e`
- 仓库根：`make test` / `make e2e` / `make lint`

## 可商用仍未完成（不宣称目标完成）
- 真实边缘证书/HSTS 人工验收执行
- 目标环境 cron/systemd 实装与演练归档
- 跨主机异地备份 + 告警通道真实跑通
