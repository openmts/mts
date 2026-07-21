# Review: mts-dashboard P253–P254（2026-07-21）

## 范围
- AccessGrants 分项失败可重试
- Users/Config 写操作失败可重试
- 抽取 createActionRetry 公共状态机

## 结论
npm test/build/e2e + go test + make e2e 通过后合入。

## 备注
- Storage/Ops/Downsample/Databases 仍为页面内 lastFailed 实现；后续可逐步迁到 createActionRetry
