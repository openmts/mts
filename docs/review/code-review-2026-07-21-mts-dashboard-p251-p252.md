# Review: mts-dashboard P251–P252（2026-07-21）

## 范围
- Operations 运维动作与分项统计失败可恢复
- Downsample / Databases 关键写/读详情失败 ActionResult 可重试

## 结论
unit/build/e2e + go test + make e2e 通过后合入。

## 备注
- 校验类错误（非法 duration 等）不进入 lastFailed，避免无意义重试
- 写门禁阻断不提供重试按钮
