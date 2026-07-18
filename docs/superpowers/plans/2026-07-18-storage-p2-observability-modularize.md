# Storage P2 Observability / Modularize / Gates

**Goal:** 闭环 P2-01/02/03(轻量)/05/06/07，不引入弱实现；P2-04 协议大收敛暂缓为后续。

## Tasks
- [x] P2-01 MemTable OOO/dup 统计与 metrics
- [x] P2-02 SSTable read/encoding 职责拆分
- [x] P2-03 查询代价时间窗启发式
- [x] P2-05 产物清理与 gitignore
- [x] P2-06 Makefile/CI race 与门禁
- [x] P2-07 compatibility.md
- [x] full test/e2e/lint/bench
- [x] P2-04 mts-server 协议 registry（本轮完成）

## 实现备注
- MemTable：`Stats.OutOfOrderSamples/DuplicateSamples/AppendedSamples` + `mts_memtable_*` counters
- SSTable：`read_index.go` / `read_page_match.go` / `encoding_page_index.go` / `encoding_samples.go`
- `estimateQueryCost` 按时间窗小时量级放大（封顶 30 天）
- `make test`/`unit` 后自动 `clean-artifacts`；新增 `make test-race`
- `scripts/ci_gate.sh` 增加核心包 race 与根二进制扫描
- `docs/compatibility.md` + README 链接

### 附带修复
- `clean-artifacts` 不再删除 `dashboard-dist`；`ensure-dashboard-embed` 生成 go:embed 占位资源
- WAL 日志测试并发缓冲 race 修复
