# Dashboard UX P234（2026-07-21）

## 目标
写入超时 e2e + 导出取消 e2e（慢导出钩子）。

## EARS
- [x] EARS-FE-P234-01 `exportYieldMs` 纯函数延迟
- [x] EARS-FE-P234-02 `window.__MTS_E2E_SLOW_EXPORT_MS` 仅 e2e 慢导出（可取消）
- [x] EARS-E2E-P234-03 写入 408 timeout 文案/error 样式
- [x] EARS-E2E-P234-04 查询历史导出 running 时可点 cancel

## 非目标
- 生产默认慢导出
- 宣称可商用完成
