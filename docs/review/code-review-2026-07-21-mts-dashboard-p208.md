# Code Review：Dashboard P208（2026-07-21）

## 范围
Account 页：改密与密码续期的离线门禁；改密草稿离开守卫。

## 处理
| 问题 | 状态 |
|---|---|
| 离线仍可点改密/续期触发失败请求 | 已拦截 + 按钮禁用 |
| 改密未提交可无提示离开 | dirty badge + routeDirty + beforeunload |

## 结论
门禁已通过并合入。不宣称可商用完成。
