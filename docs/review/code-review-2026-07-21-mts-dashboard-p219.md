# Code Review — Dashboard P219 离开文案分流（2026-07-21）

## 范围
- routeDirty kind + leaveDirtyMessage
- Readiness local 注册
- router + i18n

## 结论
- form/local 分流，避免 Readiness 误报「未保存」
- 默认 kind=form 保持旧行为

## 处理状态
| 项 | 状态 |
|----|------|
| API + 单测 | 已处理 |
| Readiness/router/i18n | 已处理 |
