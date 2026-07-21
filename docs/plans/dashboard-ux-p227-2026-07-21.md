# Dashboard UX P227（2026-07-21）

## 目标
超时/取消可见反馈对齐：导出错误路径、Query 取消/超时文案与 Write 一致，并修复取消导致的 loading 卡死。

## EARS

### EARS-FE-P227-01 ExportJob 错误友好化
- **When** 导出过程抛错，**the** `useExportJob` **shall** 使用 `formatCaughtError` 写入失败文案（而非裸 `Error.message`）。

### EARS-FE-P227-02 AbortError 映射为取消
- **When** 捕获 `AbortError` / `name=AbortError`，**the** `formatCaughtError` / `resolveCaughtErrorCode` **shall** 归类为 `canceled`（不得误判为 `timeout`）。

### EARS-FE-P227-03 API 超时仍为 timeout
- **When** `APIClientError` code=`timeout` 或 status=408，**the** 错误路径 **shall** 展示超时友好文案。

### EARS-FE-P227-04 Query 用户取消可恢复
- **When** 用户点击取消查询，**the** 系统 **shall** abort 在途请求、清除 loading，并展示 `queryCancelled`（成功 toast，与写入取消一致）。

### EARS-FE-P227-05 Query 新查询替换旧请求
- **When** 新查询开始时仍有旧请求，**the** `beginRequest` **shall** abort 旧请求并推进 seq，使旧请求不再改写 UI/loading。

### EARS-FE-P227-06 Write 取消判定统一
- **When** 写入被取消，**the** Write 页 **shall** 使用 `isCanceledError` 判定并展示 `writeCancelled`。

### EARS-UT-P227-07
- **When** 运行单元测试，**the** 套件 **shall** 覆盖 AbortError→canceled、timeout/canceled API 错误与噪声 message 过滤。

## 非目标
- NDJSON 空闲超时
- 服务端 refresh token
- 宣称可商用完成

## 实现备注
- [x] `useExportJob` → `formatCaughtError`
- [x] `apiError`：`resolveCaughtErrorCode` / `isCanceledError` / `isTimeoutError`；AbortError≠timeout
- [x] `useQueryWorkbench`：cancel 不错误推进 seq 导致 loading 卡死；`lastQueryErrorCode`
- [x] Query/Write 取消 toast 对齐
- [x] i18n `queryCancelled`
