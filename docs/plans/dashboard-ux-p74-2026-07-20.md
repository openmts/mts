# Dashboard / 运维可商用 EARS 清单（2026-07-20 P74）

## 范围
- 查询流式预览 footer i18n
- 数据库 RP duration 校验/格式化抽纯函数 + i18n
- 账户页展示会话过期时间与剩余

## 边界
- 不宣称可商用完成；不改部署侧验收计分

## EARS
- [x] EARS-FE-P74-01 WHEN 流式查询仅预览 THE SYSTEM SHALL 以当前语言追加预览说明 footer
- [x] EARS-FE-P74-02 WHEN RP duration 非法 THE SYSTEM SHALL 返回本地化错误
- [x] EARS-FE-P74-03 WHEN 打开账户页 THE SYSTEM SHALL 展示会话过期时间与剩余
- [x] EARS-FE-P74-04 WHEN 商业冒烟运行 THE SYSTEM SHALL 覆盖 account-session
- [x] EARS-DOC-P74-05 WHEN 更新基线 THE SYSTEM SHALL 记录 P74

## 验证
- `cd cmd/mts-dashboard && npm test && npm run build && npm run test:e2e`
- 仓库根：`make e2e` / `make lint` / `go test ./...`
