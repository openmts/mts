# Dashboard 体验增强 EARS 清单（2026-07-19 P5）

## 范围
- 查询历史命名 / 收藏（pinned 优先，容量裁剪保留收藏）
- ConfirmDialog / UserModals 焦点陷阱（Tab 循环 + 关闭还原焦点）
- 查询页 / TopBar 小屏响应式加固

## EARS
- [x] EARS-FE-P5-01 WHEN 用户成功执行查询 THE SYSTEM SHALL 将表单与模式写入本地历史，并支持自定义名称与收藏
- [x] EARS-FE-P5-02 WHEN 历史容量超过上限 THE SYSTEM SHALL 优先保留 pinned 条目，再按时间裁剪
- [x] EARS-FE-P5-03 WHEN 用户清空历史 THE SYSTEM SHALL 默认保留已收藏条目，并经确认对话框执行
- [x] EARS-FE-P5-04 WHEN 打开 ConfirmDialog 或 UserModals THE SYSTEM SHALL 将 Tab 焦点限制在对话框内，Escape 关闭，关闭后恢复打开前焦点
- [x] EARS-FE-P5-05 WHEN 小屏访问查询页 / 顶栏 THE SYSTEM SHALL 使用可换行工具栏、自适应表单栅格与标题截断，避免横向溢出

## 实现备注
- `utils/queryHistory.ts`：排序、标题、容量合并、localStorage 规范化 + 单测
- `useQueryHistory`：`rename` / `togglePin` / `clear({ keepPinned })` / `titleOf`
- `QueryPage`：历史面板星标、重命名、删除、确认清空
- `utils/focusTrap.ts`：`createFocusTrap` / `getFocusableElements`
- `ConfirmDialog` / `UserModals`：接入 trap + `aria-labelledby` + body scroll lock
- `TopBar` / 查询表单：小屏 padding / 栅格 / 标题 truncate

## 验证
- `cd cmd/mts-dashboard && npm run test && npm run build`
- `make test` / `make e2e` / `make lint`（以执行结果为准）
