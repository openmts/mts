# Dashboard / 通知历史导出与快捷键 EARS（2026-07-20 P92）

## 范围
- 通知历史：JSON / CSV 导出与复制
- 全局快捷键：Ctrl/⌘+Shift+H 打开/关闭通知历史
- 快捷键帮助目录登记；商业冒烟覆盖

## 边界
- 空历史时导出/复制提示且不下载
- 不宣称部署侧验收完成

## EARS
- [x] EARS-FE-P92-01 WHEN 用户打开通知历史且有记录 THE SYSTEM SHALL 支持导出 JSON/CSV
- [x] EARS-FE-P92-02 WHEN 用户复制通知历史 THE SYSTEM SHALL 将格式化 JSON 写入剪贴板
- [x] EARS-FE-P92-03 WHEN 用户按下 Ctrl/⌘+Shift+H THE SYSTEM SHALL 切换通知历史面板
- [x] EARS-FE-P92-04 WHEN 商业冒烟运行 THE SYSTEM SHALL 覆盖 export 与快捷键 testid
- [x] EARS-DOC-P92-05 WHEN 更新基线 THE SYSTEM SHALL 记录 P92

## 实现备注
- `notifyHistoryExport` 纯函数 + 单测
- `matchNotifyHistoryOpen`；testid：`notify-history-export-json/csv`、`notify-history-copy`

## 验证
- `cd cmd/mts-dashboard && npm test && npm run build && npm run test:e2e`
- 仓库根：`make e2e` / `make lint` / `go test ./...`
