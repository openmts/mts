# Dashboard UX P203（2026-07-21）

## P203 — 列表导出全面接入 ExportJob
- [x] EARS-FE-P203-01 Databases JSON/CSV 导出展示进度并可取消
- [x] EARS-FE-P203-02 Downsample JSON/CSV 导出展示进度并可取消
- [x] EARS-FE-P203-03 AccessGrants JSON/CSV 导出展示进度并可取消
- [x] EARS-FE-P203-04 Operations 操作历史 JSON 导出展示进度并可取消
- [x] EARS-E2E-P203-05 商业冒烟：databases 导出出现 export-job-banner

## 非目标
- Readiness 多文件打包流式进度
- Metrics/Config 小导出强制进度条
- 宣称可商用完成

## 验证
- [x] npm test / build / test:e2e
- [x] go test ./...
- [x] make e2e
