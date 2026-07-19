# Dashboard / 运维可商用 EARS 清单（2026-07-20 P52）

## 范围
- 导出预检项支持一键跳转对应锚点（清单/Doctor/签核/部署材料等）
- 就绪中心补齐 production / edge / backup / doctor 锚点 id
- e2e 覆盖签核与部署材料跳转

## 边界
- 跳转仅为导航定位，不自动勾选、不计分、不宣称验收完成

## EARS
- [x] EARS-FE-P52-01 WHEN 预检项存在可定位目标 THE SYSTEM SHALL 展示跳转按钮
- [x] EARS-FE-P52-02 WHEN 用户点击签核预检跳转 THE SYSTEM SHALL 定位到 #signoff-notes
- [x] EARS-FE-P52-03 WHEN 用户点击部署材料预检跳转 THE SYSTEM SHALL 定位到 #deploy-kit
- [x] EARS-FE-P52-04 WHEN 商业冒烟 e2e 运行 THE SYSTEM SHALL 覆盖预检跳转
- [x] EARS-DOC-P52-05 WHEN 更新基线 THE SYSTEM SHALL 记录 P52 与仍未完成项

## 验证
- `cd cmd/mts-dashboard && npm test && npm run build && npm run test:e2e`
- 仓库根：`make test` / `make e2e` / `make lint`

## 可商用仍未完成（不宣称目标完成）
- 真实边缘证书/HSTS 人工验收执行
- 目标环境 cron/systemd 实装与演练归档
- 跨主机异地备份 + 告警通道真实跑通
