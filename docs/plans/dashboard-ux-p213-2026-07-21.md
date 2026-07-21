# Dashboard UX P213（2026-07-21）

## P213 — Users/Databases 行级与批量写按钮离线禁用

- [x] EARS-FE-P213-01 Users 创建/批量/改密/启停/删除 离线禁用或拦截
- [x] EARS-FE-P213-02 UserModals / UserGrantPanel 提交与撤销离线禁用
- [x] EARS-FE-P213-03 Databases 删除/RP 添加离线禁用；删除确认打开拦截
- [x] EARS-E2E-P213-04 商业冒烟：users-create-open / databases-create-btn 离线禁用

## 非目标
- 服务端 refresh token / 边缘证书 / cron
- 宣称可商用完成

## 验证
- [x] npm test / build / test:e2e
- [x] go test ./...
- [x] make e2e
