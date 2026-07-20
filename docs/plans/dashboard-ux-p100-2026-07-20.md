# Dashboard / 多页命令面板深链与 hash 锚点 EARS（2026-07-20 P100）

## 范围
- Metrics / Config / Audit / Downsample 页面关键区块锚点
- 命令面板补充对应深链条目
- 统一 `useHashScroll` composable
- 商业冒烟覆盖 config-effective 与 audit-filters

## 边界
- 深链仅导航定位，不自动触发写操作
- 不宣称部署侧验收完成

## EARS
- [x] EARS-FE-P100-01 WHEN 管理员搜索配置/审计/指标/降采样关键词 THE SYSTEM SHALL 展示对应深链
- [x] EARS-FE-P100-02 WHEN 选择深链 THE SYSTEM SHALL 打开目标页并滚动到锚点区块
- [x] EARS-FE-P100-03 WHEN 页面 hash 变化 THE SYSTEM SHALL 通过 useHashScroll 定位
- [x] EARS-FE-P100-04 WHEN 商业冒烟运行 THE SYSTEM SHALL 覆盖 config-effective 与 audit-filters
- [x] EARS-DOC-P100-05 WHEN 更新基线 THE SYSTEM SHALL 记录 P100

## 验证
- `cd cmd/mts-dashboard && npm test && npm run build && npm run test:e2e`
- 仓库根：`make e2e` / `make lint` / `go test ./...`
