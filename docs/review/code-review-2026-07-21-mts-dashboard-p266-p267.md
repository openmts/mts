# 代码检视：mts-dashboard P266–P267（2026-07-21）

## 结论

- measurement 失败不再只依赖顶层 action banner，就地 soft-fail。
- Config 分项错误与 effective 解耦，避免 schema/error-codes 拖垮整页。

## 验证

见提交前 npm/e2e 门禁。
