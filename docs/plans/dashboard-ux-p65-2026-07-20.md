# Dashboard / 运维可商用 EARS 清单（2026-07-20 P65）

## 范围
- 动作结果标签 / API 错误展示 i18n 与语义统一
- 焦点环、skip-link 键盘可达性与深色对比
- 部署材料「本地勾选 ≠ 验收完成」引导与 runbook 链接强化
- 登录/查询路径错误文案走统一友好映射

## 边界
- 不计分项（deployKit / 签核 / 预检）不进入就绪四维总分
- 不宣称生产边缘证书、cron/systemd、异地备份已验收
- 领域术语（Measurement/RP/float 等）可保留英文

## EARS
- [x] EARS-FE-P65-01 WHEN 展示 ActionResult 种类标签 THE SYSTEM SHALL 按当前 locale 输出成功/失败/警告/信息（中英）
- [x] EARS-FE-P65-02 WHEN 捕获 API/网络错误 THE SYSTEM SHALL 使用友好错误映射，并跟随 locale；裸 code 不作为主文案
- [x] EARS-FE-P65-03 WHEN HTTP status 可推断错误码且 code 缺失 THE SYSTEM SHALL 映射到主错误码
- [x] EARS-FE-P65-04 WHEN 用户 Tab 到 skip-link 或交互控件 THE SYSTEM SHALL 显示可见 focus-visible 环（含深色模式）
- [x] EARS-FE-P65-05 WHEN 激活 skip-link THE SYSTEM SHALL 将焦点移至 #main-content
- [x] EARS-FE-P65-06 WHEN 就绪中心展示部署材料 THE SYSTEM SHALL 提供验收边界说明与外部 runbook 链接列表（不计分）
- [x] EARS-FE-P65-07 WHEN 登录/改密/查询失败 THE SYSTEM SHALL 优先使用 formatCaughtError 友好文案
- [x] EARS-DOC-P65-08 WHEN 更新基线 THE SYSTEM SHALL 记录 P65 与仍未完成部署侧项

## 验证
- `cd cmd/mts-dashboard && npm test && npm run build && npm run test:e2e`
- 仓库根：`make e2e` / `make lint` / `go test ./...`

## 可商用仍未完成（不宣称目标完成）
- 真实边缘证书/HSTS 人工验收执行
- 目标环境 cron/systemd 实装与演练归档
- 跨主机异地备份 + 告警通道真实跑通
