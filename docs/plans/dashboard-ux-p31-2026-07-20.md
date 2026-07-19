# Dashboard / 运维可商用 EARS 清单（2026-07-20 P31）

## 范围
- 就绪中心：验收材料一键导出包（JSON + Markdown）
- 降采样页：名称/路径筛选 + 启用状态筛选 + 多选批量启停
- 文档与冒烟同步

## 边界
- 验收包聚合浏览器侧已有材料（就绪状态/评分/doctor 摘要、可选运维操作历史、客户端/服务端版本），不替代生产人工验收
- 批量启停复用既有 enable/disable API，不新增服务端批量接口
- 仍不宣称可商用完成

## EARS
- [x] EARS-FE-P31-01 WHEN 管理员在就绪中心点击导出验收包 THE SYSTEM SHALL 下载 versioned JSON，包含 readiness 归档、客户端版本、可选服务端版本与运维操作历史
- [x] EARS-FE-P31-02 WHEN 导出验收包 THE SYSTEM SHALL 同时提供 Markdown 摘要便于交接
- [x] EARS-FE-P31-03 WHEN 降采样页输入筛选词或启用状态 THE SYSTEM SHALL 过滤策略列表；无匹配时 EmptyState
- [x] EARS-FE-P31-04 WHEN 管理员勾选策略并批量启用/禁用 THE SYSTEM SHALL 逐条调用 enable/disable 并汇总结果；禁用前确认
- [x] EARS-DOC-P31-05 WHEN 更新基线 THE SYSTEM SHALL 记录 P31 与仍未完成项

## 验证
- `cd cmd/mts-dashboard && npm run test && npm run build`
- `make test` / `make e2e` / `make lint`（与改动相关时）

## 可商用仍未完成（不宣称目标完成）
- 真实边缘证书/HSTS 人工验收执行
- 目标环境 cron/systemd 实装与演练归档
- 跨主机异地备份 + 告警通道真实跑通
