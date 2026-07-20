# Dashboard / 运维可商用 EARS 清单（2026-07-20 P73）

## 范围
- 登录页可选会话 TTL（`ttl_seconds`，对齐服务端 loginRequest）
- 查询 Builder 校验错误 i18n（聚合/tag/时间/offset/limit）
- 表单写 / Line Protocol 校验错误 i18n（注入 FormErrorT）
- 商业冒烟覆盖 login-ttl 控件

## 边界
- 不改服务端默认 TTL 策略；空 TTL = 不传字段
- 不把部署侧验收计入就绪评分
- 不宣称可商用目标完成

## EARS
- [x] EARS-FE-P73-01 WHEN 用户在登录页填写正整数 TTL THE SYSTEM SHALL 在登录请求中携带 `ttl_seconds`
- [x] EARS-FE-P73-02 WHEN TTL 非法 THE SYSTEM SHALL 本地提示且不发起登录
- [x] EARS-FE-P73-03 WHEN 查询表单校验失败 THE SYSTEM SHALL 使用当前语言错误文案
- [x] EARS-FE-P73-04 WHEN 表单/Line 写入校验失败 THE SYSTEM SHALL 使用当前语言错误文案
- [x] EARS-FE-P73-05 WHEN 商业冒烟运行 THE SYSTEM SHALL 覆盖 `login-ttl` testid
- [x] EARS-DOC-P73-06 WHEN 更新基线 THE SYSTEM SHALL 记录 P73 与仍未完成部署侧项

## 验证
- `cd cmd/mts-dashboard && npm test && npm run build && npm run test:e2e`
- 仓库根：`make e2e` / `make lint` / `go test ./...`

## 可商用仍未完成（不宣称目标完成）
- 真实边缘证书/HSTS 人工验收执行
- 目标环境 cron/systemd 实装与演练归档
- 跨主机异地备份 + 告警通道真实跑通
