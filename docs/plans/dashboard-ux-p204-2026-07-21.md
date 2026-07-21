# Dashboard UX P204（2026-07-21）

## P204 — AccessMatrix/Metrics/Config/ApiSpec 导出接入 ExportJob
- [x] EARS-FE-P204-01 AccessMatrix JSON/CSV 导出展示进度并可取消
- [x] EARS-FE-P204-02 Metrics raw/JSON 导出展示进度并可取消
- [x] EARS-FE-P204-03 Config effective/schema/error-codes 导出展示进度并可取消
- [x] EARS-FE-P204-04 ApiSpec JSON/Markdown 导出展示进度并可取消
- [x] EARS-E2E-P204-05 商业冒烟：access-matrix 导出出现 export-job-banner（有数据时）

## 非目标
- 服务端 refresh token
- Readiness 多文件流式打包
- 宣称可商用完成

## 验证
- [x] npm test / build / test:e2e
- [x] go test ./...
- [x] make e2e
