# Review: mts-dashboard P249–P250（2026-07-21）

## 范围
- Storage 写操作失败 ActionResult 可重试
- Config schema / Users grant DB 列表失败可见可重试

## 结论
通过 unit/build/e2e + go test + make e2e 后合入。

## 备注
- 写门禁阻断错误不提供重试（需先恢复在线/会话）
- delete 重试会再走 confirmDelete（若对话框已关会先 reopen）
