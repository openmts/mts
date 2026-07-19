# MTS Dashboard 可商用基线（进行中）

## 已具备
- 前后端 API 契约对齐（P0–P3）
- 查询工作台：历史/快捷键/虚拟滚动/列偏好/耗时水线
- 管理页统一结果条与空状态（P9–P10）
- 全局 loading、会话过期提示（P11）
- 安全响应头默认集 + 前端契约（P12）

## 建议上线前再确认
1. 反向代理 HTTPS、HSTS（由边缘层配置）
2. 修改默认 admin 密码策略已提示；生产强制改密
3. 备份/快照与恢复演练
4. 浏览器冒烟：登录 → 查询 → 写入 → 运维 flush（人工或 Playwright）
5. 监控：`/healthz` `/readyz` `/metrics` 接入

## 核心冒烟路径
- `/login`
- `/` 概览
- `/query`
- `/write`
- `/databases`（admin）
- `/operations`（admin）
