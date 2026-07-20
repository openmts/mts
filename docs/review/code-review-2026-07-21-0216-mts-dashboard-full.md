# MTS Dashboard 全量前端检视报告（2026-07-21 02:16）

- **时间**: 2026-07-21 02:16（Asia/Chongqing）
- **范围**: `cmd/mts-dashboard` + 与 `cmd/mts-server` HTTP 契约对齐
- **基线提交**: `241edf3`（P149/P150 合入后）
- **性质**: 只读检视 + 本轮实施；**不宣称可商用目标完成**

---

## 1. 结论

Dashboard 在 P133–P150 后主体可商用骨架已齐：鉴权、读写、库/用户、运维/降采样、审计、存储/就绪、命令面板、列表 VT、签核引导、data 面库/RP。

本轮新发现短板（可代码修复）：

| ID | 描述 | 优先级 | 计划 |
|---|---|---|---|
| FE-FULL2-P1-01 | `/databases` 对非 admin 直接 `PermissionDenied`，但 data 面已可列出可读库与 measurement/fields/series | P1 | P151 |
| FE-FULL2-P1-02 | series 元数据全量拉取后客户端截断；HTTP `limit` 会误入 tag 过滤（`queryTags` 未保留字） | P1 | P155 |
| FE-FULL2-P2-01 | Query series 加载不传 tags；服务端 tag 过滤能力未用 | P2 | P152 |
| FE-FULL2-P2-02 | Databases 页 measurement 展开 series 无上限，可能撑爆 DOM | P2 | P153 |
| FE-FULL2-P2-03 | Users 授权面板 e2e 仅 skip 注释，无打开路径 | P2 | P154 |

历史 P0 契约错误与 P137–P150 列表/元数据项：**已修复**，无回退。

| 维度 | 评级 | 说明 |
|---|---|---|
| 功能闭环 | 良 | 主路径可用 |
| 前后端契约 | 良→优（本轮后） | series limit/total + reserved qs |
| 非 admin 体验 | 中→良（本轮后） | 只读库浏览器 |
| 查询表达力 | 中高 | predicates 已有；expr 树仍非目标 |
| 可商用部署 | 中 | 边缘证书/cron/异地备份仍人工 |

---

## 2. 边界（不计分）

1. 生产边缘证书 / HSTS 人工验收  
2. 目标环境 cron/systemd 实装  
3. 跨主机异地备份 + 失败告警  
4. Query expr 树 UI（明确非目标）  
5. `GET /api/v1/admin/config` 与 effective 同内容时不必双 UI（P3 可选）

---

## 3. 本轮处理状态

| ID | 状态 |
|---|---|
| FE-FULL2-P1-01 非 admin 只读库浏览器 | **已完成 P151** |
| FE-FULL2-P1-02 series limit/total + reserved qs | **已完成 P155** |
| FE-FULL2-P2-01 Query series 服务端 tag 过滤 | **已完成 P152** |
| FE-FULL2-P2-02 库页 series 截断 | **已完成 P153** |
| FE-FULL2-P2-03 Users grant e2e | **已完成 P154** |

