# Dashboard 体验增强 EARS 清单（2026-07-19 P4）

## 范围
- 权限预检（authz self-check）
- 配置 Schema 可浏览
- 查询结果虚拟滚动

## EARS
- [x] EARS-FE-P4-01 WHEN 用户在查询/写入页点击权限预检 THE SYSTEM SHALL 调用 `/api/v1/authz/database/check`；普通用户仅可自检，admin 可代查
- [x] EARS-FE-P4-02 WHEN admin 打开配置页 THE SYSTEM SHALL 展示 `/api/v1/admin/config/schema` 并可过滤
- [x] EARS-FE-P4-03 WHEN 查询返回大量行结果 THE SYSTEM SHALL 使用虚拟列表渲染，避免整表 DOM 膨胀

## 实现备注
- 服务端 `handleAuthzDatabaseCheck` 支持登录用户自检
- `VirtualTable` + `visibleRange` 工具与单测
- ConfigPage 增加 Schema 面板

## 验证
- dashboard test/build 通过
- make e2e / make lint 通过
- make test 通过（pprof 偶发失败与本轮无关，复跑通过）
