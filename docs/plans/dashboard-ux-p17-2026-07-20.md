# Dashboard 体验增强 EARS 清单（2026-07-20 P17）

## 范围（可商用：Playwright 浏览器冒烟）
- Playwright 配置与 mts-server 自动启停 harness
- 商业路径：强制改密 → 登录 → 写入 → 查询 → Flush → 权限矩阵/实时授权/指标
- 指标页路由改为 `/observability/metrics`，避免与 Prometheus `/metrics` 冲突

## EARS
- [x] EARS-FE-P17-01 WHEN 执行 `npm run test:e2e` THE SYSTEM SHALL 自动构建并启动临时 mts-server
- [x] EARS-FE-P17-02 WHEN bootstrap admin 使用默认密码登录 THE SYSTEM SHALL 进入强制改密并在改密后重新登录成功
- [x] EARS-FE-P17-03 WHEN 管理员执行 Line Protocol 写入 THE SYSTEM SHALL 在 UI 显示写入成功
- [x] EARS-FE-P17-04 WHEN 管理员执行运维 Flush THE SYSTEM SHALL 在 UI 显示 Flush 已完成
- [x] EARS-FE-P17-05 WHEN 访问权限矩阵/实时授权/指标页 THE SYSTEM SHALL 渲染对应标题
- [x] EARS-FE-P17-06 WHEN 指标页路由设计 THE SYSTEM SHALL 使用 `/observability/metrics` 避免与后端 `/metrics` 文本接口冲突

## 验证
- `cd cmd/mts-dashboard && npm run test && npm run build && npm run test:e2e`
- `make test` / `make e2e` / `make lint`

## 可商用仍未完成（不宣称目标完成）
- 边缘 HTTPS/HSTS 真实部署落地
- CI 中默认安装 Playwright 浏览器（当前需 `npm run test:e2e:install`）
