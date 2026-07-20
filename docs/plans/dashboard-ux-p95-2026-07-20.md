# Dashboard / 偏好导出与通知搜索 EARS（2026-07-20 P95）

## 范围
- 账户页：独立导出/复制本机偏好（`mts.client.prefs`）
- 通知历史：文本搜索（与类型过滤组合）
- 商业冒烟覆盖相关 testid

## 边界
- 偏好导出不含密码/Token
- 搜索大小写不敏感，匹配 kind + message
- 不宣称部署侧验收完成

## EARS
- [x] EARS-FE-P95-01 WHEN 用户导出本机偏好 THE SYSTEM SHALL 下载仅含 prefs 的 JSON 包
- [x] EARS-FE-P95-02 WHEN 用户复制本机偏好 THE SYSTEM SHALL 将格式化 JSON 写入剪贴板
- [x] EARS-FE-P95-03 WHEN 用户在通知历史输入搜索词 THE SYSTEM SHALL 与类型过滤组合过滤结果
- [x] EARS-FE-P95-04 WHEN 商业冒烟运行 THE SYSTEM SHALL 覆盖 prefs export/copy 与 notify-history-search
- [x] EARS-DOC-P95-05 WHEN 更新基线 THE SYSTEM SHALL 记录 P95

## 实现备注
- `clientPrefsExport` / `filterNotifyHistory` 纯函数 + 单测
- testid：`account-prefs-export`、`account-prefs-copy`、`notify-history-search`

## 验证
- `cd cmd/mts-dashboard && npm test && npm run build && npm run test:e2e`
- 仓库根：`make e2e` / `make lint` / `go test ./...`
