# Code Review：Dashboard P201–P202（2026-07-21）

## 范围
- 统一导出进度/取消（exportJob + ExportJobBanner）
- Write AbortController 取消

## 处理
| 问题 | 状态 |
|---|---|
| 大导出无进度/无法取消 | Query/Audit/Users 已接入 |
| Write 长请求无法取消 | 已支持 abort |

## 结论
待门禁后合入。不宣称可商用完成。
