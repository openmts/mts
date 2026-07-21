# Code Review：Dashboard P209（2026-07-21）

## 范围
Login 离线门禁；ForceChangePassword 离线门禁 + 改密脏守卫。

## 处理
| 问题 | 状态 |
|---|---|
| 离线仍可提交登录 | 已拦截/禁用 |
| 强制改密离线失败体验差 | 已拦截/禁用 |
| 强制改密未提交可无提示离开 | dirty + beforeunload |

## 结论
门禁已通过并合入。不宣称可商用完成。
