# Dashboard / 运维可商用 EARS 清单（2026-07-20 P40）

## 范围
- 权限矩阵 capability 行数据层双语（area / capability / notes）
- 区域筛选改用稳定 `areaKey`，展示文案随 locale 切换
- 单元测试覆盖全量行双语；Playwright 覆盖语言切换后的矩阵行
- 文档同步

## 边界
- 不改变后端鉴权语义；矩阵仍为产品语义图
- 仍不宣称可商用完成（边缘证书/HSTS 验收、cron/systemd、跨主机备份）

## EARS
- [x] EARS-FE-P40-01 WHEN 语言为 en THE SYSTEM SHALL 展示矩阵区域/能力/备注英文文案
- [x] EARS-FE-P40-02 WHEN 按区域筛选 THE SYSTEM SHALL 使用稳定 areaKey 且不依赖中文标签
- [x] EARS-FE-P40-03 WHEN 运行矩阵单元测试 THE SYSTEM SHALL 校验全部 capability 行含 zh/en
- [x] EARS-FE-P40-04 WHEN 商业冒烟 e2e 运行 THE SYSTEM SHALL 覆盖语言切换后的矩阵行
- [x] EARS-DOC-P40-05 WHEN 更新基线 THE SYSTEM SHALL 记录 P40 与仍未完成项

## 验证
- `cd cmd/mts-dashboard && npm test && npm run build`
- 仓库根：`make test` / `make e2e` / `make lint`

## 实现备注
- `CapabilityRow`：`areaKey` + `LocalizedText`（area/capability/notes）
- `matrixAreas()` 返回 `{ key, label }`；页面 `textForLocale(..., uiLocale)`
- e2e：`/access` 默认中文行 → 切换 locale → Data plane / Query rows / notes 英文 → 切回中文

## 可商用仍未完成（不宣称目标完成）
- 真实边缘证书/HSTS 人工验收执行
- 目标环境 cron/systemd 实装与演练归档
- 跨主机异地备份 + 告警通道真实跑通
- 共享组件/弹窗中文硬编码收口（UserModals / ConfirmDialog / UserGrantPanel 等）
