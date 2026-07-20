# Dashboard / 部署材料与签核串联 EARS（2026-07-20 P118）

## 范围
- 部署样例模板关联签核字段（relatedSignoff）
- 模板卡片「填写对应签核备注」跳转并聚焦字段
- 签核字段下方展示相关部署样例入口
- 命令面板补充导出预检 / 联调清单深链（只读导航）
- 不把部署本地勾选或签核备注计入 readiness 总分；不自动伪造成验收完成

## 边界
- 不执行真实证书/cron/异地备份
- 命令面板禁止危险写操作自动执行

## EARS
- [x] EARS-FE-P118-01 WHEN 用户查看部署样例 THE SYSTEM SHALL 展示关联签核字段的跳转入口
- [x] EARS-FE-P118-02 WHEN 用户点击关联签核 THE SYSTEM SHALL 滚动到签核区并聚焦对应文本框
- [x] EARS-FE-P118-03 WHEN 用户查看签核字段 THE SYSTEM SHALL 提供相关部署样例跳转
- [x] EARS-FE-P118-04 WHEN 管理员使用命令面板 THE SYSTEM SHALL 可导航到导出预检与联调清单锚点
- [x] EARS-FE-P118-05 WHEN 商业冒烟运行 THE SYSTEM SHALL 覆盖部署→签核串联
- [x] EARS-DOC-P118-06 WHEN 更新基线 THE SYSTEM SHALL 记录 P118

## 实现备注
- `relatedSignoffForTemplate` / `templatesForSignoffField`
- testid：`deploy-jump-signoff-*` / `signoff-related-*` / `signoff-open-tpl-*`
- 命令面板：`readiness-export-preflight` / `readiness-deploy-drill`

## 验证
- `cd cmd/mts-dashboard && npm test && npm run build && npm run test:e2e`
- 根目录 `make e2e` + `go test ./...`
