# Dashboard / 运维可商用 EARS 清单（2026-07-20 P21）

## 范围
- 真实 data_dir 快照 API（storagecheck.Snapshot）
- 旁路 restore drill API（恢复到 backups 下独立目录并校验）
- Storage 页一键编排 + 备份演练清单联动
- 文档与可商用基线同步

## EARS
- [x] EARS-BE-P21-01 WHEN 管理员调用 data-snapshot THE SYSTEM SHALL 将 data_dir 拷贝到 backup 下独立目录并返回 files/bytes
- [x] EARS-BE-P21-02 WHEN 管理员调用 restore-drill THE SYSTEM SHALL 旁路 restore 到 backup 子目录且不得覆盖 live data_dir
- [x] EARS-BE-P21-03 WHEN restore-drill 完成 THE SYSTEM SHALL 对目标目录执行 storagecheck 并返回是否通过
- [x] EARS-FE-P21-04 WHEN Storage 页操作旁路恢复 THE SYSTEM SHALL 调用上述 API 并更新演练进度
- [x] EARS-DOC-P21-05 WHEN 更新基线 THE SYSTEM SHALL 记录 P21 与仍未完成项

## 验证
- `go test ./cmd/mts-server -run 'DataSnapshot|RestoreDrill|Doctor' -count=1`
- `cd cmd/mts-dashboard && npm run test && npm run build`
- `make test` / `make e2e` / `make lint`

## 可商用仍未完成（不宣称目标完成）
- 生产边缘证书/HSTS 人工验收执行
- 跨主机异地拷贝与定时备份编排（部署侧）
