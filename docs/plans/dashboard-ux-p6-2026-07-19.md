# Dashboard 体验增强 EARS 清单（2026-07-19 P6）

## 范围
- 查询页键盘快捷键
- 查询历史 JSON 导入 / 导出
- 查询结果面板偏好本地持久化

## EARS
- [x] EARS-FE-P6-01 WHEN 用户在查询页按下 Ctrl/⌘+Enter THE SYSTEM SHALL 触发查询（输入框内同样生效）
- [x] EARS-FE-P6-02 WHEN 用户按下 Escape THE SYSTEM SHALL 取消进行中的查询，或关闭历史面板
- [x] EARS-FE-P6-03 WHEN 用户按下 Ctrl/⌘+Shift+C THE SYSTEM SHALL 复制当前查询结果
- [x] EARS-FE-P6-04 WHEN 用户按下 Ctrl/⌘+H THE SYSTEM SHALL 切换查询历史面板
- [x] EARS-FE-P6-05 WHEN 用户导出查询历史 THE SYSTEM SHALL 下载 versioned JSON（含 name/pinned）
- [x] EARS-FE-P6-06 WHEN 用户导入历史 JSON THE SYSTEM SHALL 校验后按 id 合并并遵守容量上限
- [x] EARS-FE-P6-07 WHEN 用户切换图表/原始字段/历史面板 THE SYSTEM SHALL 将偏好写入 localStorage 并在下次打开恢复

## 实现备注
- `utils/keyboard.ts`：快捷键匹配与可编辑目标判断
- `utils/queryHistoryIO.ts`：导出载荷 / 导入解析 / 合并
- `utils/queryPrefs.ts`：`mts_query_prefs`
- `useQueryHistory.exportPayload/importPayload`
- `QueryPage` 绑定全局 keydown、导入文件 input、偏好 watch

## 验证
- `cd cmd/mts-dashboard && npm run test && npm run build`
- `make test` / `make e2e` / `make lint`
