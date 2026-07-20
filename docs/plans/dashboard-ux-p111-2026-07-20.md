# Dashboard / 用户表列排序与粘性表头 EARS（2026-07-20 P111）

## 范围
- Users 表支持按用户名 / 角色 / 状态列排序（升序/降序/默认）
- 表头 sticky，横向滚动时仍可见
- 排序状态本机记忆（localStorage）
- Databases 列表名称排序（升序/降序/默认）

## 边界
- 不改变服务端 API
- 不宣称部署侧验收完成

## EARS
- [x] EARS-FE-P111-01 WHEN 用户点击用户表头列 THE SYSTEM SHALL 在升序/降序/默认间循环切换排序
- [x] EARS-FE-P111-02 WHEN 排序激活 THE SYSTEM SHALL 对当前过滤结果重排并显示方向指示
- [x] EARS-FE-P111-03 WHEN 刷新页面 THE SYSTEM SHALL 恢复本机记忆的用户表排序
- [x] EARS-FE-P111-04 WHEN 用户切换数据库名称排序 THE SYSTEM SHALL 重排过滤后的库列表
- [x] EARS-FE-P111-05 WHEN 商业冒烟运行 THE SYSTEM SHALL 覆盖排序控件
- [x] EARS-DOC-P111-06 WHEN 更新基线 THE SYSTEM SHALL 记录 P111

## 实现备注
- `listSort` 纯函数 + 单测
- key `mts.dashboard.users-sort.prefs.v1` / `mts.dashboard.databases-sort.prefs.v1`
- testid：`users-sort-*` / `databases-sort-name`

## 验证
- `cd cmd/mts-dashboard && npm test && npm run build && npm run test:e2e`
