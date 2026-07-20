# Dashboard / 运维可商用 EARS 清单（2026-07-20 P64）

## 范围
- Write 表单字段类型下拉（float/int/string/bool）展示走 i18n type*

## 边界
- 提交到服务端的 type 值仍为 float/int/string/bool
- 类型标签保持技术术语
- 不宣称生产验收完成

## EARS
- [x] EARS-FE-P64-01 WHEN 写入表单选择字段类型 THE SYSTEM SHALL 使用 typeFloat/Int/String/Bool 展示
- [x] EARS-FE-P64-02 WHEN 商业冒烟 e2e 运行 THE SYSTEM SHALL 覆盖写入入口
- [x] EARS-DOC-P64-03 WHEN 更新基线 THE SYSTEM SHALL 记录 P64 与仍未完成项

## 验证
- `cd cmd/mts-dashboard && npm test && npm run build && npm run test:e2e`
- 仓库根：`make e2e` / `make lint` / `go test ./...`

## 可商用仍未完成（不宣称目标完成）
- 真实边缘证书/HSTS 人工验收执行
- 目标环境 cron/systemd 实装与演练归档
- 跨主机异地备份 + 告警通道真实跑通
