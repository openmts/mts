# Dashboard / 运维可商用 EARS 清单（2026-07-20 P25）

## 范围
- 就绪中心 localStorage 状态 JSON 导出/导入（交接与审计留痕）
- 演练归档模板下载（JSON/Markdown：时间、操作者、score、doctor 摘要、清单）
- About / 版本信息：服务端 version API + 前端 About 页
- Playwright 加深：就绪勾选持久化、Storage data-snapshot 入口可达
- 文档与基线同步（仍不宣称可商用完成）

## 边界
- 不覆盖 live data_dir；不实现跨主机备份调度
- 导出/导入仅浏览器本机状态，不改服务端配置
- 版本 API 只读，admin 鉴权

## EARS
- [x] EARS-FE-P25-01 WHEN 管理员导出就绪状态 THE SYSTEM SHALL 下载 versioned JSON（含 production/edgeHttps/backupSchedule/updatedAt）
- [x] EARS-FE-P25-02 WHEN 管理员导入合法就绪 JSON THE SYSTEM SHALL 合并或替换并写回 localStorage
- [x] EARS-FE-P25-03 WHEN 管理员下载演练归档 THE SYSTEM SHALL 生成含 score/doctor/清单摘要的 JSON 与 Markdown
- [x] EARS-API-P25-04 WHEN 管理员 GET /api/v1/admin/version THE SYSTEM SHALL 返回 version/commit/built_at
- [x] EARS-FE-P25-05 WHEN 用户打开 About 页 THE SYSTEM SHALL 展示服务版本与前端构建信息
- [x] EARS-E2E-P25-06 WHEN 商业浏览器冒烟 THE SYSTEM SHALL 覆盖就绪勾选持久化与 Storage data-snapshot 按钮可达
- [x] EARS-DOC-P25-07 WHEN 更新基线 THE SYSTEM SHALL 记录 P25 与仍未完成项

## 验证
- `cd cmd/mts-dashboard && npm run test && npm run build`
- `go test ./cmd/mts-server -count=1 -timeout 8m`
- `make test` / `make e2e` / `make lint`（按门禁）
- 可选：`cd cmd/mts-dashboard && npm run test:e2e`

## 可商用仍未完成（不宣称目标完成）
- 真实边缘证书/HSTS 在生产代理的人工验收执行
- 目标环境 cron/systemd 实际安装与演练归档
- 跨主机异地备份真实跑通与告警通道
