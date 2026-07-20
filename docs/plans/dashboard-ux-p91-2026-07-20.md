# Dashboard / 最近固定与通知历史 EARS（2026-07-20 P91）

## 范围
- 最近访问：固定/取消固定（sessionStorage）；清空默认保留固定项
- 通知历史：toast 新建写入 session 历史；顶栏铃铛打开面板可清空
- 商业冒烟覆盖 pin 与 notify-history

## 边界
- 固定项最多 4；总最近项最多 8
- 通知历史最多 40；不跨浏览器长期持久化
- 不宣称部署侧验收完成

## EARS
- [x] EARS-FE-P91-01 WHEN 用户固定最近访问 THE SYSTEM SHALL 将该路径置顶并在再次访问时保持固定
- [x] EARS-FE-P91-02 WHEN 用户清空最近访问 THE SYSTEM SHALL 默认仅移除未固定项
- [x] EARS-FE-P91-03 WHEN 系统推送新 toast THE SYSTEM SHALL 写入本会话通知历史
- [x] EARS-FE-P91-04 WHEN 用户打开通知历史 THE SYSTEM SHALL 展示本会话记录并支持清空
- [x] EARS-FE-P91-05 WHEN 商业冒烟运行 THE SYSTEM SHALL 覆盖 pin 与 notify-history testid
- [x] EARS-DOC-P91-06 WHEN 更新基线 THE SYSTEM SHALL 记录 P91

## 实现备注
- `setRecentRoutePinned` / `notifyHistory` 纯函数 + 单测
- testid：`recent-route-pin-*`、`topbar-notify-history`、`notify-history-panel`

## 验证
- `cd cmd/mts-dashboard && npm test && npm run build && npm run test:e2e`
- 仓库根：`make e2e` / `make lint` / `go test ./...`
