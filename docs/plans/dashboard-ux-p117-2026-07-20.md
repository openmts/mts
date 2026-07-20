# Dashboard / 就绪中心签核体验增强 EARS（2026-07-20 P117）

## 范围
- 签核备注：输入实时保存（input）、进度条、按字段跳转缺失项
- 复制缺失字段摘要；字段级完成态徽章
- 不把签核计入 readiness 总分；文案继续声明「备注≠验收完成」

## 边界
- 不宣称部署侧验收完成
- 不自动填充虚假证据

## EARS
- [x] EARS-FE-P117-01 WHEN 用户编辑签核文本 THE SYSTEM SHALL 在输入过程中持久化本机备注
- [x] EARS-FE-P117-02 WHEN 三项备注状态变化 THE SYSTEM SHALL 更新进度条与完成计数
- [x] EARS-FE-P117-03 WHEN 用户点击缺失字段跳转 THE SYSTEM SHALL 聚焦对应文本框
- [x] EARS-FE-P117-04 WHEN 用户复制缺失摘要 THE SYSTEM SHALL 将未填字段列表写入剪贴板
- [x] EARS-FE-P117-05 WHEN 商业冒烟运行 THE SYSTEM SHALL 覆盖进度条与实时保存
- [x] EARS-DOC-P117-06 WHEN 更新基线 THE SYSTEM SHALL 记录 P117

## 实现备注
- `signoffProgressPercent` 纯函数
- testid：`signoff-progress` / `signoff-jump-*` / `signoff-copy-missing` / `signoff-field-status-*`

## 验证
- `cd cmd/mts-dashboard && npm test && npm run build && npm run test:e2e`
