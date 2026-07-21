# Code Review：Dashboard P207（2026-07-21）

## 范围
剩余小导出统一接入 `useExportJob`（进度 / 取消 / busy / banner）：
- Query history、Overview、About
- Operations stats / maint-errors / action-log
- Storage download、Account snapshot + client prefs
- Write result/draft、NotifyHistory JSON/CSV

## 处理
| 问题 | 状态 |
|---|---|
| 多页仍直接 `downloadJSON`，导出体验不一致 | 已统一 ExportJob |
| Account 偏好导出未接进度/取消 | 已接 `runJSONExport` + banner |
| Write / 通知历史导出无 busy 态 | 已接 ExportJob |

## 安全
- 偏好/账户导出仍不包含 token / 密码字段（沿用既有 export builder）

## 结论
门禁已通过并合入。不宣称可商用完成。
