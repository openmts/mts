# Dashboard / 本机偏好工具与通知过滤 EARS（2026-07-20 P94）

## 范围
- 账户页：本机偏好恢复默认、JSON/文件导入（落地页/密度/侧栏/语言/主题）
- 通知历史：按 kind 过滤；导出作用于当前过滤结果
- 侧栏折叠偏好导入后跨布局同步（CustomEvent）
- 商业冒烟覆盖相关 testid

## 边界
- 不导入密码/Token；非法 JSON 明确错误
- 不宣称部署侧验收完成

## EARS
- [x] EARS-FE-P94-01 WHEN 用户点击恢复默认偏好 THE SYSTEM SHALL 将落地页/密度/侧栏/语言/主题重置为默认
- [x] EARS-FE-P94-02 WHEN 用户导入合法偏好 JSON THE SYSTEM SHALL 应用并持久化本机偏好
- [x] EARS-FE-P94-03 WHEN 用户在通知历史选择类型过滤 THE SYSTEM SHALL 仅展示匹配项
- [x] EARS-FE-P94-04 WHEN 商业冒烟运行 THE SYSTEM SHALL 覆盖 prefs tools 与 notify-history-filter
- [x] EARS-DOC-P94-05 WHEN 更新基线 THE SYSTEM SHALL 记录 P94

## 实现备注
- `clientPrefs` 纯函数解析；`filterNotifyHistoryByKind`
- testid：`account-prefs-*`、`notify-history-filter`

## 验证
- `cd cmd/mts-dashboard && npm test && npm run build && npm run test:e2e`
- 仓库根：`make e2e` / `make lint` / `go test ./...`
