# Dashboard UX P217（2026-07-21）

## P217 — Readiness 签核/清单会话脏离开守卫

### 背景
Readiness 清单与签核在变更时**立即写入** `localStorage`，并非传统「未提交服务端」表单。
为与其它管理页交互对齐，本页引入**相对进入页面时基线**的会话脏标记：
- 路由离开 `registerDirtyChecker` + 全局 `unsavedLeaveConfirm`
- `beforeunload` 提示
- 标题旁 dirty badge

### EARS
- [x] EARS-FE-P217-01 `readinessComparable` / `isReadinessDirty`：忽略 `updatedAt`，比较清单 + deployKit + signoff
- [x] EARS-FE-P217-02 `ReadinessPage` 进页快照基线；toggle/签核输入后 dirty；恢复基线后 clean
- [x] EARS-FE-P217-03 `registerDirtyChecker('readiness')` + `beforeunload`
- [x] EARS-FE-P217-04 导入成功后 `markReadinessClean`（导入结果成为新基线）
- [x] EARS-FE-P217-05 UI `readiness-dirty-badge` + i18n
- [x] EARS-E2E-P217-06 商业冒烟：清单切换与签核编辑 dirty/restore

### 部署验收文档对齐（POC）
- [x] EARS-DOC-P217-07 baseline 同步 P217；就绪中心 ↔ 运维清单对照入口保留（不实现边缘证书/cron 实装）

### 非目标
- 服务端独立 refresh token
- 真实边缘证书 / HSTS / cron / 跨主机备份
- 宣称可商用完成

### 验证
- [x] npm test / build / test:e2e
- [x] go test ./...
- [x] make e2e
