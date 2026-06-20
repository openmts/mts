# storage_matrix

`storage_matrix` 编排 `tests/scale/storage_10m`，用于比较不同数据规模、压缩算法和写入持久化策略下的存储层性能。

示例：

```bash
go run ./tests/scale/storage_matrix \
  -sizes 100k,1m \
  -compressions off,snappy,zstd \
  -durabilities buffered,write-sync,strict-flush \
  -data-root /tmp/mts-storage-matrix \
  -out /tmp/mts-storage-matrix.json \
  -markdown /tmp/mts-storage-matrix.md
```

全量矩阵包含 100K、1M、10M 三档规模，五种压缩算法和四种写入策略。10M strict flush 场景耗时较高，日常验证应通过 flags 过滤范围。

默认生成策略使用 `-timestamp-step 1s` 和 `-shard-duration 24h`。该口径让 10M 数据自然跨多个 shard，避免把全部样本压入单个 shard 后得到不真实的全量 compaction 结果。需要复现旧的单 shard 压力口径时，可显式设置更小的 timestamp step 或更大的 shard duration。
