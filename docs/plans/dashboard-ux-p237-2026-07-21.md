# Dashboard UX P237（2026-07-21）

## 目标
加载失败可重试：`ActionResultBanner` 支持 retry + 关键列表页接入。

## EARS
- [x] EARS-FE-P237-01 Banner `retryable` + `action-result-retry` + emit retry
- [x] EARS-FE-P237-02 i18n `retry`
- [x] EARS-FE-P237-03 Overview/Metrics/About/Users/Databases/AccessGrants/Audit/Downsample 接入

## 非目标
- 所有错误条强制重试
- 宣称可商用完成

## 验证说明
- Metrics `/metrics` 与 SPA 同名路径，route mock 易拦截文档导航；e2e 不强制 mock，功能以接线与 build 验证。
