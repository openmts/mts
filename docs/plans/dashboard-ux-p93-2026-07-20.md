# Dashboard / 命令面板固定与偏好快照 EARS（2026-07-20 P93）

## 范围
- 命令面板最近访问：固定项优先并标注 `data-pinned`
- 账户导出：附带本机偏好（落地页/密度/侧栏/语言/主题，不含密钥）
- 商业冒烟覆盖 pinned 属性

## 边界
- 偏好快照仅本机可读配置，不导出 Token/密码
- 不宣称部署侧验收完成

## EARS
- [x] EARS-FE-P93-01 WHEN 命令面板展示最近访问 THE SYSTEM SHALL 将固定路径优先排序并标记
- [x] EARS-FE-P93-02 WHEN 用户导出账户快照 THE SYSTEM SHALL 包含非敏感本机偏好字段
- [x] EARS-FE-P93-03 WHEN 商业冒烟运行 THE SYSTEM SHALL 覆盖 `command-recent-*` 的 data-pinned
- [x] EARS-DOC-P93-04 WHEN 更新基线 THE SYSTEM SHALL 记录 P93

## 验证
- `cd cmd/mts-dashboard && npm test && npm run build && npm run test:e2e`
- 仓库根：`make e2e` / `make lint` / `go test ./...`
