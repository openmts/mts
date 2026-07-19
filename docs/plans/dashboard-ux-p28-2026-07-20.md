# Dashboard / 运维可商用 EARS 清单（2026-07-20 P28）

## 范围
- 全局命令面板（Ctrl/Cmd+K）：按关键词过滤页面并导航
- 审计页：快捷时间范围、客户端二次筛选、清空筛选、导出空态提示
- 键盘：命令面板 focus trap + Esc 关闭
- 文档与冒烟同步

## 边界
- 命令面板仅导航已有路由，不执行写操作
- 不新增服务端审计 API；时间快捷为前端 since/until 填充
- 仍不宣称可商用完成

## EARS
- [x] EARS-FE-P28-01 WHEN 用户按下 Ctrl/Cmd+K THE SYSTEM SHALL 打开命令面板（输入框外亦可）
- [x] EARS-FE-P28-02 WHEN 用户在命令面板输入关键词 THE SYSTEM SHALL 过滤导航项（中英 label/path/keywords）
- [x] EARS-FE-P28-03 WHEN 用户选择导航项 THE SYSTEM SHALL 跳转并关闭面板
- [x] EARS-FE-P28-04 WHEN 审计页选择快捷时间范围 THE SYSTEM SHALL 填充 since/until 并触发查询
- [x] EARS-FE-P28-05 WHEN 审计结果加载后 THE SYSTEM SHALL 支持 detail/action 客户端二次筛选与清空
- [x] EARS-DOC-P28-06 WHEN 更新基线 THE SYSTEM SHALL 记录 P28 与仍未完成项

## 验证
- `cd cmd/mts-dashboard && npm run test && npm run build`
- `make test` / `make e2e` / `make lint`

## 可商用仍未完成（不宣称目标完成）
- 真实边缘证书/HSTS 人工验收
- 目标环境 cron/systemd 与跨主机备份实装
