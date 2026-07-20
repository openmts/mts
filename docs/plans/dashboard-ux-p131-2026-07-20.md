# Dashboard / Ops 动作日志虚拟滚动 EARS（2026-07-20 P131）

## 范围
- 运维动作日志接入 VirtualTable
- 提升本会话历史上限（50→200）
- 增加 kind/status/文本筛选；导出覆盖筛选结果
- 修复商业冒烟种子写入 sessionStorage（与实现一致）

## 边界
- 不改 flush/compact/retention 写路径确认门禁
- 历史仍仅本机会话级，不上传服务端

## EARS
- [x] EARS-FE-P131-01 WHEN 动作日志非空 THE SYSTEM SHALL 虚拟渲染可视行
- [x] EARS-FE-P131-02 WHEN 用户筛选日志 THE SYSTEM SHALL 按 kind/status/文本过滤
- [x] EARS-FE-P131-03 WHEN 用户导出日志 THE SYSTEM SHALL 导出当前筛选结果
- [x] EARS-FE-P131-04 WHEN 新增记录 THE SYSTEM SHALL 最多保留 200 条
- [x] EARS-FE-P131-05 WHEN 商业冒烟种子历史 THE SYSTEM SHALL 写入 sessionStorage
- [x] EARS-DOC-P131-06 WHEN 更新基线 THE SYSTEM SHALL 记录 P131

## 实现备注
- ACTION_ROW_HEIGHT=48 / LIST_HEIGHT=288
- testid：ops-action-virtual-list / ops-action-virtual-hint / ops-action-filter-*

## 验证
- npm test && npm run build && npm run test:e2e
- make e2e + go test ./...
