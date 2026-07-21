# Code Review：Dashboard P215–P216（2026-07-21）

## 范围
统一 mutation 门禁（离线 + 会话 critical/expired）；Storage 删除入口门禁。

## 处理
| 问题 | 状态 |
|---|---|
| 会话即将过期仍可提交写操作 | 已 writeBlocked |
| 各页离线门禁重复且不一致 | useMutationGuard 统一 |
| Storage 删除可点开确认框 | 已 disabled + open 拦截 |
| 续期被会话门禁误伤 | Account/Login 仍仅 offline |

## 结论
门禁已通过并合入。不宣称可商用完成。
