# Dashboard / 运维可商用 EARS 清单（2026-07-20 P30）

## 范围
- 路由级脏表单离开确认（Query/Write 注册 dirty 源）
- 用户列表：名称筛选 + 角色筛选 + 空态
- 数据库列表：名称筛选 + 空态
- 文档与冒烟同步

## 边界
- 脏态仅覆盖已有 formDirty 的查询/写入页；不强制 Config 只读 JSON
- 不新增服务端列表 API
- 仍不宣称可商用完成

## EARS
- [x] EARS-FE-P30-01 WHEN 查询或写入表单脏且用户导航离开 THE SYSTEM SHALL 弹出确认，取消则留在当前页
- [x] EARS-FE-P30-02 WHEN 表单干净 THE SYSTEM SHALL 允许自由导航且不弹确认
- [x] EARS-FE-P30-03 WHEN 用户页输入筛选词 THE SYSTEM SHALL 过滤用户名/显示名并可按角色过滤
- [x] EARS-FE-P30-04 WHEN 数据库页输入筛选词 THE SYSTEM SHALL 过滤库名；无匹配时 EmptyState
- [x] EARS-DOC-P30-05 WHEN 更新基线 THE SYSTEM SHALL 记录 P30 与仍未完成项

## 验证
- `cd cmd/mts-dashboard && npm run test && npm run build`
- `make test` / `make e2e` / `make lint`

## 可商用仍未完成（不宣称目标完成）
- 真实边缘证书/HSTS 人工验收
- 目标环境 cron/systemd 与跨主机备份实装
