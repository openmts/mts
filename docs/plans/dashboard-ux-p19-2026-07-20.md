# Dashboard / 运维可商用 EARS 清单（2026-07-20 P19）

## 范围
- 真实 data_dir 旁路快照/恢复演练自动化
- HTTP TLS 启用时自动 HSTS
- `mts-server doctor` 可商用部署检查增强
- 更新生产清单与 runbook

## EARS
- [x] EARS-BE-P19-01 WHEN 数据目录完成写入并 flush THE SYSTEM SHALL 支持 storagecheck.Snapshot 旁路拷贝
- [x] EARS-BE-P19-02 WHEN 旁路 restore 后打开 Engine THE SYSTEM SHALL 读回与源一致的关键点
- [x] EARS-BE-P19-03 WHEN HTTP TLS 启用 THE SYSTEM SHALL 发送 Strict-Transport-Security
- [x] EARS-BE-P19-04 WHEN HTTP TLS 未启用 THE SYSTEM SHALL 不默认发送 HSTS，并由 doctor 提示边缘 HTTPS
- [x] EARS-BE-P19-05 WHEN 执行 doctor THE SYSTEM SHALL 检查 data/backup 目录与 TLS 配置并输出提示

## 验证
- `go test ./cmd/mts-server -run 'RestoreDrill|DoctorChecks|SecurityHeaders' -count=1`
- `cd cmd/mts-dashboard && npm run test && npm run build`
- `make test` / `make e2e` / `make lint`

## 可商用仍未完成（不宣称目标完成）
- 真实反向代理证书与 HSTS 在生产环境的人工验收
- 跨主机异地拷贝与定时备份编排（部署侧）
