# Dashboard / 运维可商用 EARS 清单（2026-07-20 P75）

## 范围
- 写入页默认 TypedBatch，并记忆模式/选项到 localStorage
- 审计页 limit 可选 + total/合并持久化提示
- 登录 TTL 输入记忆

## 边界
- 不改服务端 audit 契约字段
- 不宣称可商用部署验收完成

## EARS
- [x] EARS-FE-P75-01 WHEN 首次打开写入页 THE SYSTEM SHALL 默认选中 TypedBatch 并标注推荐
- [x] EARS-FE-P75-02 WHEN 切换写入模式/选项 THE SYSTEM SHALL 持久化到本机偏好
- [x] EARS-FE-P75-03 WHEN 审计页加载 THE SYSTEM SHALL 展示 limit 选择、total 与合并持久化说明
- [x] EARS-FE-P75-04 WHEN 登录成功且填写 TTL THE SYSTEM SHALL 记忆 TTL 输入
- [x] EARS-FE-P75-05 WHEN 商业冒烟运行 THE SYSTEM SHALL 覆盖 write-mode 与 audit-limit
- [x] EARS-DOC-P75-06 WHEN 更新基线 THE SYSTEM SHALL 记录 P75

## 验证
- `cd cmd/mts-dashboard && npm test && npm run build && npm run test:e2e`
- 仓库根：`make e2e` / `make lint` / `go test ./...`
