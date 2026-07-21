# Dashboard UX P219（2026-07-21）

## P219 — 路由离开文案分流（form vs local）

### 背景
Readiness 等页变更已自动写入 localStorage，全局 `unsavedLeaveConfirm`「未保存」措辞易误导。

### EARS
- [x] EARS-FE-P219-01 `registerDirtyChecker` 支持 `DirtyKind`（form|local，默认 form）
- [x] EARS-FE-P219-02 `leaveDirtyMessage`：有 form 脏优先 unsaved；仅 local 用 localDirtyLeaveConfirm
- [x] EARS-FE-P219-03 Readiness 注册 kind=`local`
- [x] EARS-FE-P219-04 router beforeEach 使用 leaveDirtyMessage
- [x] EARS-UT-P219-05 routeDirty 单测覆盖文案分流

### 非目标
- beforeunload 自定义文案（浏览器限制）
- 宣称可商用完成

### 验证
- [x] npm test / build / test:e2e
- [x] go test ./...
- [x] make e2e
