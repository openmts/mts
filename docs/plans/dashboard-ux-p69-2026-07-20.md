# Dashboard / 运维可商用 EARS 清单（2026-07-20 P69）

## 范围
- 服务端 `/readyz` 可达性探测（与浏览器 offline 区分）
- 顶栏不可达提示条（不计就绪评分）
- Account 改密表单 aria-invalid / alert 对齐登录页
- 商业冒烟：健康时不可达条不出现；账户改密入口可测

## 边界
- 探测失败不等于业务 API 鉴权失败；不触发强制登出
- 浏览器 offline 时优先展示离线条，不叠加误导性「服务不可达」主因
- 不计分；不宣称生产验收完成

## EARS
- [x] EARS-FE-P69-01 WHEN 分类探测结果 THE SYSTEM SHALL 区分 online-ok / online-unreachable / offline / unknown
- [x] EARS-FE-P69-02 WHEN 浏览器在线且 /readyz 连续失败 THE SYSTEM SHALL 展示服务不可达提示条
- [x] EARS-FE-P69-03 WHEN /readyz 恢复成功 THE SYSTEM SHALL 隐藏不可达提示条
- [x] EARS-FE-P69-04 WHEN 浏览器 offline THE SYSTEM SHALL 优先离线提示，不把主因标为服务不可达
- [x] EARS-FE-P69-05 WHEN Account 改密校验失败 THE SYSTEM SHALL 以 alert 展示并以 aria-invalid 标记字段
- [x] EARS-FE-P69-06 WHEN 商业冒烟 e2e 运行 THE SYSTEM SHALL 覆盖健康时无不可达条与账户页表单
- [x] EARS-DOC-P69-07 WHEN 更新基线 THE SYSTEM SHALL 记录 P69 与仍未完成部署侧项

## 验证
- `cd cmd/mts-dashboard && npm test && npm run build && npm run test:e2e`
- 仓库根：`make e2e` / `make lint` / `go test ./...`

## 可商用仍未完成（不宣称目标完成）
- 真实边缘证书/HSTS 人工验收执行
- 目标环境 cron/systemd 实装与演练归档
- 跨主机异地备份 + 告警通道真实跑通
