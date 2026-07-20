# Dashboard / 运维可商用 EARS 清单（2026-07-20 P70）

## 范围
- 服务可达性探测改为模块单例，布局与 Overview 共享状态
- Overview 展示连通性卡片（ok / unreachable / offline / unknown），与顶栏条一致
- 健康检查 status 展示本地化（ok/passed/fail 等）
- 商业冒烟覆盖 Overview 连通性指示

## 边界
- 不计入就绪四维评分
- 不宣称生产验收完成
- 探测仍不强制登出

## EARS
- [x] EARS-FE-P70-01 WHEN 多处订阅服务可达性 THE SYSTEM SHALL 共享同一探测状态与定时器（单例）
- [x] EARS-FE-P70-02 WHEN Overview 加载 THE SYSTEM SHALL 展示连通性状态卡片并与 classifyReachability 一致
- [x] EARS-FE-P70-03 WHEN 用户在 Overview 点击重试连通 THE SYSTEM SHALL 触发同一 checkOnce
- [x] EARS-FE-P70-04 WHEN 展示 health check status THE SYSTEM SHALL 对常见 status 做 i18n 映射
- [x] EARS-FE-P70-05 WHEN 商业冒烟 e2e 运行 THE SYSTEM SHALL 覆盖 Overview 连通性 testid 为可达
- [x] EARS-DOC-P70-06 WHEN 更新基线 THE SYSTEM SHALL 记录 P70 与仍未完成部署侧项

## 验证
- `cd cmd/mts-dashboard && npm test && npm run build && npm run test:e2e`
- 仓库根：`make e2e` / `make lint` / `go test ./...`

## 可商用仍未完成（不宣称目标完成）
- 真实边缘证书/HSTS 人工验收执行
- 目标环境 cron/systemd 实装与演练归档
- 跨主机异地备份 + 告警通道真实跑通
