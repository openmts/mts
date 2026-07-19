# Dashboard / 运维可商用 EARS 清单（2026-07-20 P22）

## 范围
- 可商用就绪中心页（/ops/readiness）
- 上线清单 / 边缘 HTTPS 验收 / 备份编排指引状态持久化
- 备份跨主机与定时编排纯数据指引
- Playwright 冒烟覆盖就绪中心与存储页
- 文档与基线同步

## EARS
- [x] EARS-FE-P22-01 WHEN 管理员打开就绪中心 THE SYSTEM SHALL 聚合生产清单、HTTPS 验收、备份编排与 doctor 状态
- [x] EARS-FE-P22-02 WHEN 用户勾选清单项 THE SYSTEM SHALL 将完成状态持久化到 localStorage
- [x] EARS-FE-P22-03 WHEN 维护跨主机备份 THE SYSTEM SHALL 提供可勾选的定时/异地编排步骤指引
- [x] EARS-FE-P22-04 WHEN 商业浏览器冒烟 THE SYSTEM SHALL 覆盖 /ops/readiness 与 /storage
- [x] EARS-DOC-P22-05 WHEN 更新基线 THE SYSTEM SHALL 记录 P22 与仍未完成项

## 验证
- `cd cmd/mts-dashboard && npm run test && npm run build`
- `go test ./cmd/mts-server -count=1`
- `make test` / `make e2e` / `make lint`
- 可选：`npm run test:e2e`

## 可商用仍未完成（不宣称目标完成）
- 生产边缘证书在真实反向代理环境的人工验收执行
- 跨主机定时备份在目标环境的实际部署（脚本可参考，不强制内置调度器）
