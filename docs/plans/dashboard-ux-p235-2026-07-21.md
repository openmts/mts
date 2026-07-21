# Dashboard UX P235（2026-07-21）

## 目标
Query/Write 结果条无障碍 live region + 剪贴板错误友好化。

## EARS
- [x] EARS-FE-P235-01 错误 `role=alert` assertive；取消 `role=status` polite
- [x] EARS-FE-P235-02 clipboard 错误不抛原始对象字符串
- [x] EARS-E2E-P235-03 查询取消/超时 role 断言

## 非目标
- 全站所有 mts-alert 改造
- 宣称可商用完成
