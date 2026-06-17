# mts 存储层商用化六大专项 EARS 清单

## 背景

当前 mts 存储层已经具备 LSM 主链路、二进制列式 SSTable、WAL、MemTable、Manifest、Flush、Level Compaction、流式查询、读放大控制、payload 压缩、series 二级索引和基础故障测试能力。其成熟度适合作为研发内核继续演进，但距离商用级存储层还需要系统补齐可靠性、内存治理、Compaction 长稳、查询执行、文件格式治理和生产运维能力。

本文将商用差距拆成六个专项，每个专项都用 EARS 句式描述必须满足的行为。后续实现计划必须逐项映射这些 EARS，不允许用“后续优化”“简单实现”“临时方案”替代验收。

## 总体商用目标

- 系统应在异常掉电、磁盘故障、fsync 失败、WAL/Manifest/SSTable 部分失败时保持已确认写入的数据可恢复。
- 系统应在小内存环境中可通过配置控制存储层内存占用，并在超过阈值前主动降载或拒写。
- 系统应在长期 10M+ 级别 wide10 写入、查询、Compaction 压测下保持 RSS、GC、读放大、写放大和空间放大稳定。
- 系统应对查询、写入、Flush、Compaction、Recovery、Retention、WAL、SSTable、磁盘空间和后台任务提供可观测指标。
- 系统应提供明确的运维接口、健康状态、错误分级和问题定位入口。

## 专项一：可靠性与异常故障矩阵

### 目标

把 WAL、Manifest、SSTable、Flush、Compaction、Retention、Recovery 的故障行为从“局部覆盖”提升到“矩阵覆盖”。商用标准不是“正常流程可用”，而是任意关键持久化步骤失败后，系统都能保持一致性、可恢复性和明确错误。

### EARS 清单

- When WAL append 返回错误时，系统应拒绝本次写入，并保证 MemTable 不应用该批数据。
- When WAL append 成功但 WAL fsync 失败且写入请求要求同步时，系统应返回明确错误，并不得把该批数据声明为已持久化。
- When WAL segment rollover 过程中创建新 segment 失败时，系统应继续保留旧 segment 的可 replay 状态，并返回 rollover 错误。
- When WAL checkpoint 失败时，系统应保留可 replay 的 WAL 文件，并不得删除仍需恢复的数据。
- When WAL replay 遇到截断尾部记录时，系统应跳过未完整提交的尾部记录，并保留此前完整记录。
- When WAL replay 遇到 checksum 不匹配的中间记录时，系统应停止恢复并返回数据损坏错误。
- When MemTable flush 开始前 WAL 已成功写入时，系统应保证重启后能从 WAL 恢复该批数据。
- When SSTable part 写入过程中 timestamps、values、index、metaindex、series index 任一文件写失败时，系统应删除未提交 part 或标记为孤儿，Manifest 不应引用它。
- When SSTable metadata 写失败时，系统应返回 flush 错误，并保证 Manifest 不引用该 part。
- When Manifest 原子写临时文件失败时，系统应保留旧 Manifest 可读。
- When Manifest rename 失败时，系统应保留旧 Manifest，并清理临时 Manifest 文件。
- When Manifest fsync 失败时，系统应返回明确错误，并不得 checkpoint WAL。
- When flush 已写 part 但 Manifest 未提交时进程退出，系统重启后应忽略该孤儿 part，并保证 WAL replay 能恢复数据。
- When Manifest 已提交但 WAL checkpoint 未完成时进程退出，系统重启后应加载 Manifest part，并允许 WAL replay 去重或覆盖重复数据。
- When Compaction 输出 part 写失败时，系统应保留输入 parts 和原 Manifest。
- When Compaction Manifest 切换失败时，系统应保留输入 parts 可读，并清理未引用输出 part。
- When Compaction Manifest 切换成功但删除旧 part 失败时，系统应保持新 Manifest 可读，并将旧 part 作为可清理垃圾暴露给 maintenance。
- When Retention 删除 part 时存在活跃 reader，系统应延迟关闭或保留文件句柄，避免 reader 访问已关闭文件。
- When 磁盘返回 ENOSPC 时，系统应返回磁盘空间不足错误，并避免 Manifest 指向未完整 part。
- When 文件系统出现短写时，系统应检测写入字节不足并返回错误。
- When 文件权限不足导致目录或文件创建失败时，系统应返回路径、操作和底层错误。
- When 进程在 flush、checkpoint、compaction、retention 的每个关键点崩溃时，系统重启后应进入一致状态。
- When 恢复流程发现孤儿 part、临时 Manifest、临时 compaction 输出时，系统应清理可安全删除的文件，并记录清理结果。
- When 恢复流程发现 Manifest 引用缺失 part 时，系统应返回致命恢复错误，而不是静默丢数据。
- When 恢复流程发现 part metadata 与 Manifest 元数据不一致时，系统应返回结构化一致性错误。
- When 故障注入测试执行时，系统应覆盖 WAL、Manifest、PartWriter、FileOps、Compaction、Retention 的成功和失败路径。

### 验收标准

- `tests/fault` 覆盖所有关键持久化步骤的失败点。
- 每个故障用例都包含重启恢复验证。
- 每个错误路径都断言 Manifest、WAL、part 目录和查询结果的一致性。
- 故障矩阵文档记录失败点、预期行为、测试用例路径和当前状态。

## 专项二：字节级内存治理与 OOM 防护

### 目标

把当前基于样本数的 MemTable 阈值升级为存储层总内存预算治理。商用目标是限制引擎可控内存，而不是依赖 Go runtime 在 RSS 升高后被动 GC。

### EARS 清单

- When 用户配置存储层总内存软阈值时，系统应在达到软阈值前触发 flush、降低 batch 接收或延迟后台任务。
- When 用户配置存储层总内存硬阈值时，系统应在预计写入后超过硬阈值时拒绝写入，并返回明确错误。
- When MemTable 追加样本时，系统应按列缓冲容量估算 float、int、bool、string、timestamp、writeSeq 的内存占用。
- When 字符串字段写入时，系统应将字符串内容字节数和引用开销计入内存预算。
- When map、slice、column buffer 扩容时，系统应按容量而不是长度估算内存。
- When WAL batch buffer 占用内存时，系统应将 pending records、encoded bytes、segment buffer 纳入存储内存预算。
- When SSTable flush 编码产生临时 buffer 时，系统应通过预算 token 获取临时内存额度。
- When payload compression 产生压缩源、目标和工作区时，系统应将临时压缩 buffer 纳入预算。
- When Compaction 读取、merge、编码输出 part 时，系统应使用独立 compaction 内存预算，并受全局预算约束。
- When 查询产生 row/column/aggregate/window/pagination buffer 时，系统应通过 query memory budget 限制结果物化。
- When 多个 shard 同时写入时，系统应按全局 storage memory manager 统一扣减预算，而不是每个 shard 独立无限增长。
- When 后台 compaction 与前台写入争用内存时，系统应优先保证前台写入和 recovery，后台任务应降速或暂停。
- When flush 释放 MemTable snapshot 后，系统应归还内存预算，并允许后续写入继续。
- When buffer pool 归还大对象时，系统应按最大保留容量限制 pool，避免 pool 自身推高 RSS。
- When 内存预算不足但可通过 flush 释放时，系统应先 flush 再决定是否拒写。
- When flush 后仍无法满足预算时，系统应返回 `ErrStorageMemoryLimitExceeded`。
- When 查询超过内存预算时，系统应停止读取并返回 query memory budget 错误。
- When compaction 超过内存预算时，系统应中止本次 compaction 并保留输入 parts。
- When storage memory manager 统计内存时，系统应暴露 current、peak、soft limit、hard limit、rejected writes、flush triggered、throttle duration 指标。
- When 用户未配置内存预算时，系统应保持默认行为，但仍暴露估算内存指标。
- When RSS 与引擎估算内存差距超过阈值时，系统应暴露 runtime memory gap 指标，辅助定位非引擎内存。
- When 长期压测运行时，系统应验证 RSS peak 不随批次线性无限增长。

### 验收标准

- 10M wide10 写入在小内存配置下不会 OOM，而是可控 flush 或明确拒写。
- storage memory metrics 能解释 MemTable、WAL、flush、compression、compaction、query 的主要内存来源。
- pprof memory profile 中临时 map 和临时 buffer 分配路径被持续追踪。
- 内存预算测试覆盖前台写入、查询、flush、compaction 并发场景。

## 专项三：Compaction 长期稳定性与放大控制

### 目标

把 Compaction 从“功能可运行”提升到“长期写入下稳定控制读放大、写放大、空间放大”。商用级 Compaction 必须可调度、可观测、可降载、可恢复。

### EARS 清单

- When L0 part 数超过阈值时，系统应选择同 shard 内候选 parts 执行 compaction。
- When L0 总大小超过阈值时，系统应按 size-tiered 或 level policy 生成 compaction 计划。
- When L1+ level 内存在时间范围或 series 范围重叠时，系统应暴露 overlap 指标并触发修复 compaction。
- When 同 level part 已按 `(seriesID,time)` 不重叠组织时，系统查询应最多命中必要 part，降低读放大。
- When compaction 输出超过目标 part 大小时，系统应按配置切分输出 part。
- When 下一级 level 达到上限时，系统应级联 compaction，并限制单次级联最大步数。
- When 前台写入压力升高时，后台 compaction 应降低并发或暂停，避免抢占内存和 IO。
- When compaction backlog 超过阈值时，系统应将 health 状态标记为 degraded。
- When compaction 输入 part 被 retention 选择删除时，系统应通过调度锁避免同时操作同一 part。
- When compaction 输入 part 被查询 reader 引用时，系统应允许 reader 继续读取旧文件，直到 reader 关闭。
- When compaction 过程中发生错误时，系统应保留输入 parts 并清理未提交输出。
- When compaction 中途进程退出时，系统重启后应只加载 Manifest 引用的 parts，并清理孤儿输出。
- When compaction 成功提交 Manifest 后，系统应 checkpoint 或记录可安全删除的旧 parts。
- When compaction planner 生成计划时，系统应记录 level、candidate count、input bytes、output estimate、score、reason。
- When compaction executor 运行时，系统应记录 active count、duration、input bytes、output bytes、dropped rows、error count。
- When compaction 处理 tombstone 或重复点时，系统应保留最新 writeSeq 并剔除被删除样本。
- When compaction 遇到损坏 part 时，系统应返回损坏错误并停止提交输出。
- When compaction 输出使用不同层级压缩策略时，系统应按目标 level 的 compression 配置写入。
- When 长期写入产生大量小 SSTable 时，系统应通过 compaction 将 part 数维持在可配置范围。
- When 读放大超过预算时，系统应优先调度降低该 shard/level 读放大的 compaction。
- When 磁盘剩余空间不足以容纳 compaction 输出时，系统应拒绝启动该 compaction。
- When 手动 compact 被调用时，系统应返回任务状态、耗时、影响 part 数和错误。
- When 后台 compaction 周期运行时，系统应避免重复选择同一候选集合。

### 验收标准

- 10M+ 长期写入后 part 数、level 分布、读放大、空间放大保持在配置阈值内。
- Compaction 中断、失败、重启恢复测试全部通过。
- Compaction metrics 能说明 backlog、active、duration、bytes、score 和 errors。
- 压测报告包含写放大、空间放大和查询延迟随时间变化曲线或机器可解析数据。

## 专项四：商用级查询执行器与语义完整性

### 目标

把查询从“可查、可流式”提升到“语义完整、可取消、可下推、可控读放大、可扩展到上层查询语言”。商用级查询必须能解释每次读取了什么、为什么读取、何时停止。

### EARS 清单

- When 查询指定 database、retention policy、measurement、tags 时，系统应在 catalog 阶段解析出精确 seriesID 集合。
- When 查询指定 fields 时，系统应在进入 SSTable value page 前完成 fieldID 过滤。
- When 查询时间范围只命中部分 shard 时，系统应跳过不相交 shard。
- When 查询时间范围只命中部分 part 时，系统应通过 PartMeta 跳过不相交 part。
- When 查询指定 seriesID 时，系统应通过 SSTable series index 定位目标 index row。
- When 查询指定时间范围只命中部分 value page 时，系统应只读取相交 page。
- When 查询带 context cancellation 时，系统应在 catalog、shard、part、page、merge、aggregate、row materialization 阶段快速停止。
- When 查询带 deadline 时，系统应返回 `context.DeadlineExceeded`，并关闭底层 reader。
- When 查询带 limit/offset 时，系统应尽可能在流式执行阶段早停，避免读取无用 page。
- When 查询带聚合函数 count/sum/min/max/avg/first/last 时，系统应按 series、field、window 输出正确结果。
- When 聚合函数不支持字段类型时，系统应返回明确错误。
- When 查询窗口跨 shard 或 part 时，系统应正确合并窗口边界。
- When 查询遇到乱序样本时，系统应输出按 timestamp 排序的结果。
- When 查询遇到重复 timestamp 且 writeSeq 不同时，系统应保留最新 writeSeq。
- When 查询遇到 tombstone 时，系统应过滤被删除样本。
- When row 查询从 column 数据合成时，系统应按 `(seriesID,timestamp)` 合并字段，避免全量 map 物化不可控增长。
- When 查询结果超过 MaxSamples 时，系统应返回读预算错误，而不是继续分配内存。
- When 查询扫描 shard、part、index row、value page、samples 时，系统应记录 query stats。
- When 查询可由 metadata 直接判断为空时，系统应返回空结果并避免打开 value 文件。
- When 查询只需要 first/last 且 metadata/page index 足够时，系统应优先使用边界快路径。
- When 查询执行器内部任意节点出错时，系统应停止后续节点并通过 iterator `Err()` 暴露错误。
- When iterator `Close()` 被调用时，系统应释放 snapshot、part payload、page buffer 和聚合状态。
- When 并发查询运行时，系统应避免共享可变 buffer 造成数据竞争。
- When 查询涉及多个 level 的重叠数据时，系统应按 writeSeq 和 tombstone 语义合并结果。
- When 查询执行计划生成时，系统应可输出 explain 信息，包括命中 shard/part 数、下推条件和预算。

### 验收标准

- 查询语义测试覆盖 tag、field、time、series、window、aggregate、limit、offset、cancel、tombstone、重复点、跨 shard。
- 读放大测试能断言跳过 shard/part/page 的数量。
- 查询 pprof 显示大查询不会一次性物化全部 rows。
- 查询错误路径均可通过 iterator `Err()` 或 API error 返回。

## 专项五：文件格式治理、恢复协议与工具化

### 目标

概念设计阶段可以暂时去掉版本兼容，但商用前必须重新建立文件格式治理。这里的目标不是保留历史包袱，而是保证格式可识别、可验证、可迁移、可诊断、可修复。

### EARS 清单

- When 系统写入 WAL segment 时，文件应包含 magic、format id、checksum 和 record 边界。
- When 系统读取 WAL segment 时，应验证 magic、checksum、record length 和截断状态。
- When 系统写入 SSTable metadata 时，应包含 magic、format id、part id、level、time range、series range、row count、block refs 和 checksum。
- When 系统读取 SSTable metadata 时，应验证所有 block ref 不越界。
- When 系统写入 Manifest 时，应包含 magic、format id、manifest sequence、parts 列表和 checksum。
- When 系统读取 Manifest 时，应验证 sequence 单调性和 part 引用存在。
- When 新增 SSTable 文件组件时，系统应在 metadata 中明确引用该组件，并在 OpenPart 时校验存在。
- When 文件格式发生不兼容变化时，系统应拒绝打开未知格式，并返回明确错误。
- When 文件格式发生兼容扩展时，系统应提供明确的字段默认值和测试覆盖。
- When 用户执行离线校验工具时，系统应扫描 WAL、Manifest、SSTable、series index、value pages 并输出一致性报告。
- When 离线校验发现孤儿 part 时，工具应标记为可清理。
- When 离线校验发现 Manifest 引用缺失 part 时，工具应标记为致命错误。
- When 离线校验发现 checksum 错误时，工具应定位到文件、offset、block type。
- When 用户执行修复工具时，系统应只执行显式确认的安全修复动作。
- When 修复工具删除孤儿文件时，应先输出 dry-run 计划。
- When 系统升级需要格式迁移时，应提供离线迁移工具和回滚前置检查。
- When 迁移工具运行时，应先备份 Manifest 或生成可恢复 checkpoint。
- When 迁移中断时，系统应能根据 checkpoint 恢复到迁移前或迁移后的一致状态。
- When 运行 e2e 测试时，系统应验证存储目录不包含 JSON 数据文件。
- When 文件权限创建时，目录应为 `0700`，普通文件应为 `0600`。
- When 工具输出诊断报告时，应包含文件路径、part id、level、time range、series range、错误原因。
- When 工具遇到未知文件时，应按 ignore、warn、fatal 策略配置处理。
- When 系统打开数据目录时，应先校验基础目录结构和权限。
- When SSTable series index、metaindex、index、timestamps、values 任一文件不一致时，系统应拒绝加载该 part。

### 验收标准

- 所有持久化文件都有 magic、format id、checksum 或等价校验。
- 提供 `check`、`repair --dry-run`、`repair --apply` 级别的工具入口。
- 文件格式文档描述每个文件的二进制布局和字段语义。
- 恢复协议文档覆盖正常启动、异常重启、迁移中断和修复流程。

## 专项六：生产级 Metrics、服务化运维与长稳门禁

### 目标

商用存储层必须能被运维系统理解：能看见、能告警、能诊断、能限流、能压测、能证明长稳。该专项把 metrics、health、pprof、admin、scale gate 和基线管理统一起来。

### EARS 清单

- When Engine 打开时，系统应注册 WAL、MemTable、SSTable、Query、Compaction、Retention、Recovery、StorageMemory、Runtime 指标。
- When WAL 写入时，系统应记录 append latency、fsync latency、segment count、pending bytes、checkpoint count、replay records。
- When MemTable 写入时，系统应记录 sample count、estimated bytes、series count、field count、flush trigger reason。
- When SSTable 写入时，系统应记录 part count、level distribution、data bytes、index bytes、compression ratio。
- When 查询执行时，系统应记录 query duration、shards scanned、parts scanned、pages read、samples returned、budget errors、cancellations。
- When Compaction 执行时，系统应记录 active count、queue length、score、input bytes、output bytes、duration、errors。
- When Retention 执行时，系统应记录 expired parts、deleted bytes、delete errors、active readers blocked。
- When Recovery 执行时，系统应记录 replay duration、replayed records、orphan parts cleaned、recovery errors。
- When StorageMemory 预算变化时，系统应记录 current bytes、peak bytes、soft/hard limit、flush triggered、rejected writes。
- When runtime 指标采集时，系统应暴露 heap、RSS、GC count、pause、goroutines、fd count。
- When 服务启动时，系统应提供 `/metrics`、`/healthz`、`/readyz`、`/debug/pprof/`、`/admin/compact`。
- When `/healthz` 被调用时，系统应返回进程存活状态。
- When `/readyz` 被调用时，系统应检查 WAL、Manifest、disk space、compaction backlog、memory budget、maintenance errors。
- When `/readyz` 非 ready 时，系统应返回结构化原因。
- When `/admin/compact` 被调用时，系统应要求 context timeout，并返回任务 id、状态和错误。
- When 管理端点执行操作时，系统应记录结构化审计日志。
- When pprof 开启时，系统应允许通过配置限制监听地址。
- When metrics 暴露时，系统应避免高基数字段标签，如 raw series key、measurement 全量名。
- When 运行 10M wide10 写入压测时，系统应输出吞吐、duration、RSS peak、heap、GC、data bytes、SSTable count、compaction backlog。
- When 运行 10M 查询压测时，系统应输出 cold/hot latency、query stats、RSS peak、allocs、GC、errors。
- When 运行 10M compaction 压测时，系统应输出 write amplification、space amplification、level distribution、backlog drain time。
- When 压测指标超过基线阈值时，系统应以非零退出码失败。
- When 压测结束时，系统应清理临时目录、二进制、profile、coverage，除非显式保留。
- When CI 或本地 gate 运行时，系统应能按 quick、standard、soak 三档执行不同压测规模。
- When 长稳测试连续运行时，系统应按周期输出机器可解析报告，便于趋势分析。
- When 生产发生错误时，系统日志应包含 operation、shard、part、level、path、duration、error kind。

### 验收标准

- metrics 指标文档列出名称、类型、标签、含义和告警建议。
- health/ready/admin/pprof 服务端点有 e2e 测试。
- 10M+ scale tests 能输出 JSON 报告并支持基线回归判断。
- 长稳测试覆盖 write/query/compact/restart/recovery 五类工作负载。
- 运维接口默认安全收敛，不暴露敏感路径和高基数标签。

## 专项依赖顺序

1. 文件格式治理与故障注入接口先行，否则可靠性矩阵无法精确覆盖失败点。
2. 字节级内存治理应与查询执行器和 compaction executor 同步接入，避免只管写入不管后台任务。
3. 查询执行器语义完整性先于读放大和长稳门禁，否则压测结果无法解释。
4. Compaction 长稳依赖 metrics、读放大统计和故障恢复协议。
5. 生产级 metrics 与服务化运维应贯穿全部专项，而不是最后补仪表盘。
6. 长稳门禁作为最终验收层，覆盖全部专项的交叉行为。

## 总体验收门槛

- 所有专项 EARS 都能映射到测试用例、指标或工具命令。
- `go test ./... -timeout 10m`、`golangci-lint run --timeout 10m`、`goimports-reviser -rm-unused -format ./...` 通过。
- `tests/e2e` 全部用例通过。
- `tests/fault` 故障矩阵通过。
- `tests/scale` 至少包含 10M write、10M query、10M compact、restart recovery、soak 五类报告。
- 压测报告必须包含吞吐、延迟、RSS peak、heap、GC、SSTable count、data bytes、读放大、写放大、空间放大。
- 每个商用门槛都不能只以“功能可用”验收，必须包含异常路径、资源上限和长期稳定性验证。
