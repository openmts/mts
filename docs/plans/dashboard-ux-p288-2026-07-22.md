# Dashboard UX P288（2026-07-22）

## 目标
- 写入/查询进行中：提交按钮文案与 aria-busy 明确
- 取消按钮 idle/busy title 与 aria-label 可感知

## 验收
- [x] write-submit / query-run aria-busy + submitting 文案
- [x] write-cancel / query-cancel title 区分 idle/busy
- [x] e2e 校验 idle cancel title 与 aria-busy=false
- [x] npm test / build / e2e 通过后合入
