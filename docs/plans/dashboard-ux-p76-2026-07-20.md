# Dashboard / 运维可商用 EARS 清单（2026-07-20 P76）

## 范围
- 存储页选择 data-snapshot 作为 restore-drill 源
- data-snapshot 列表支持「用作演练源 / 复制路径」
- 配置导出下载统一 `downloadJSON`/`stampFilename`

## 边界
- 不删除 restore-drill 目录 API（服务端无 delete data-snapshot 路由）
- 不宣称部署侧验收完成

## EARS
- [x] EARS-FE-P76-01 WHEN 存在 data-snapshot THE SYSTEM SHALL 在演练源下拉中列出并默认选中最新项
- [x] EARS-FE-P76-02 WHEN 执行旁路恢复演练 THE SYSTEM SHALL 使用所选 source_path
- [x] EARS-FE-P76-03 WHEN 用户点击复制路径 THE SYSTEM SHALL 写入剪贴板
- [x] EARS-FE-P76-04 WHEN 下载配置导出 THE SYSTEM SHALL 使用统一下载工具与时间戳文件名
- [x] EARS-FE-P76-05 WHEN 商业冒烟访问存储页 THE SYSTEM SHALL 覆盖 drill source testid
- [x] EARS-DOC-P76-06 WHEN 更新基线 THE SYSTEM SHALL 记录 P76

## 验证
- `cd cmd/mts-dashboard && npm test && npm run build && npm run test:e2e`
- 仓库根：`make e2e` / `make lint` / `go test ./...`
