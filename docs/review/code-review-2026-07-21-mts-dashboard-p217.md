# Code Review — Dashboard P217 Readiness 脏守卫（2026-07-21）

## 范围
- `cmd/mts-dashboard/src/utils/readinessState.ts`：`readinessComparable` / `isReadinessDirty`
- `cmd/mts-dashboard/src/pages/ReadinessPage.vue`：基线、badge、routeDirty、beforeunload、导入后 clean
- i18n + commercial-smoke e2e

## 结论
- 语义：相对**进页基线**的会话脏（本地已自动持久化，文案已说明）
- 与 Account/Write/Config 的 `registerDirtyChecker` 模式一致
- 导入成功重置基线，避免「导入后无法离开」误报

## 处理状态
| 项 | 状态 |
|----|------|
| dirty util + 单测 | 已处理 |
| 页面门禁 + badge | 已处理 |
| e2e | 已处理 |
| 宣称可商用完成 | 不做 |

## 残余风险
- 全局离开文案仍用 `unsavedLeaveConfirm`（「未保存」措辞对自动落盘页略宽泛；badge title 已澄清）
- 不宣称 goal complete
