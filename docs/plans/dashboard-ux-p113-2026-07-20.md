# Dashboard / 运维危险操作确认强化 EARS（2026-07-20 P113）

## 范围
- Retention 应用：二次确认 + 输入固定口令 `RETENTION` 才能执行
- 清空操作历史：改为 ConfirmDialog，禁止静默清空
- Flush/Compact 保持危险确认（已有），补充测试覆盖口令框

## 边界
- 不自动执行运维写操作
- 不宣称部署侧验收完成

## EARS
- [x] EARS-FE-P113-01 WHEN 用户打开 Retention 确认框 THE SYSTEM SHALL 要求输入 RETENTION 方可确认
- [x] EARS-FE-P113-02 WHEN 用户清空操作历史 THE SYSTEM SHALL 弹出确认对话框且确认后才清空
- [x] EARS-FE-P113-03 WHEN 用户取消清空 THE SYSTEM SHALL 保留现有日志
- [x] EARS-FE-P113-04 WHEN 商业冒烟运行 THE SYSTEM SHALL 覆盖 retention require-text 与 clear-log 确认
- [x] EARS-DOC-P113-05 WHEN 更新基线 THE SYSTEM SHALL 记录 P113

## 实现备注
- `confirmRequireText` 按 kind 返回口令
- `clearLogOpen` + ConfirmDialog
- testid：`ops-clear-log` / `confirm-dialog-input`

## 验证
- `cd cmd/mts-dashboard && npm test && npm run build && npm run test:e2e`
