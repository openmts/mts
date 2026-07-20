# Dashboard / 写入行上限与 RP 手填提示 EARS（2026-07-21 P138）

## 范围
- 表单写行数上限 50；达到后禁用新增并提示 Line/Typed
- 写入页 RP 列表不可用时展示手填提示
- 不改服务端契约（data 面 RP API 仍 open）

## EARS
- [x] EARS-FE-P138-01 WHEN 表单行达上限 THE SYSTEM SHALL 阻止新增并提示
- [x] EARS-FE-P138-02 WHEN RP 列表为空或无权限 THE SYSTEM SHALL 展示手填提示且保留输入
- [x] EARS-FE-P138-03 WHEN 商业冒烟进入 form 写模式 THE SYSTEM SHALL 可见行数指示
- [x] EARS-DOC-P138-04 WHEN 更新基线 THE SYSTEM SHALL 记录 P138

## 验证
- npm test && npm run build && npm run test:e2e ✅
- make e2e + go test ./... ✅
