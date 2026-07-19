# Dashboard 体验增强 EARS 清单（2026-07-19 P10）

## 范围
向可商用后台收敛：管理页结果条 / 空状态 / 加载态统一

- Storage
- Users
- Config
- Overview（错误条）
- Databases（结果条）

## EARS
- [x] EARS-FE-P10-01 WHEN 存储验证/快照/导出/删除完成 THE SYSTEM SHALL 以 ActionResultBanner 展示结果
- [x] EARS-FE-P10-02 WHEN 快照列表为空或加载中 THE SYSTEM SHALL 使用 EmptyState
- [x] EARS-FE-P10-03 WHEN 用户管理动作成功或失败 THE SYSTEM SHALL 使用统一结果条；用户列表为空时展示引导
- [x] EARS-FE-P10-04 WHEN 配置验证/热重载/Token 操作完成 THE SYSTEM SHALL 写入统一结果条；schema/错误码为空时 EmptyState
- [x] EARS-FE-P10-05 WHEN 概览加载失败或部分统计不可用 THE SYSTEM SHALL 使用 ActionResultBanner
- [x] EARS-FE-P10-06 WHEN 数据库管理动作失败/成功 THE SYSTEM SHALL 使用统一结果条

## 验证
- `cd cmd/mts-dashboard && npm run test && npm run build`
- `make test` / `make e2e` / `make lint`
