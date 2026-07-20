# Dashboard / Overview 签核进度串联 EARS（2026-07-20 P119）

## 范围
- Overview 展示签核填写进度条与完成态文案
- 缺失字段可从 Overview 跳转到就绪中心对应字段锚点
- 不计入 readiness 总分；备注≠生产验收

## 边界
- 不在 Overview 编辑签核文本
- 不把部署侧 open 项计入评分

## EARS
- [x] EARS-FE-P119-01 WHEN 管理员查看 Overview THE SYSTEM SHALL 展示签核填写进度条
- [x] EARS-FE-P119-02 WHEN 签核字段缺失 THE SYSTEM SHALL 提供跳转到对应字段的入口
- [x] EARS-FE-P119-03 WHEN 商业冒烟运行 THE SYSTEM SHALL 覆盖 Overview 签核进度面板
- [x] EARS-DOC-P119-04 WHEN 更新基线 THE SYSTEM SHALL 记录 P119

## 实现备注
- 复用 `signoffProgressPercent` / 字段锚点 `signoff-field-*`
- testid：`overview-signoff-panel` / `overview-signoff-progress-bar` / `overview-signoff-jump-*`

## 验证
- `cd cmd/mts-dashboard && npm test && npm run build && npm run test:e2e`
- 根目录 `make e2e` + `go test ./...`
