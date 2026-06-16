# Storage Hotpath Allocation Design

## 目标

本轮优化聚焦 pprof 中暴露的存储层热点：查询结果合并、SSTable block 读写、读后排序和写入分组。实现必须保持现有二进制 SSTable 格式兼容，并尽可能减少临时 `map` 分配。

## 需求

- 当多个 part 或 memtable 返回同一 `(series_id, field_id)` 列时，系统应优先使用有序线性合并，并在相同 timestamp 上保留 `WriteSeq` 最大的样本。
- 当输入列或样本乱序时，系统应保留正确性，并只在必要时退回排序或低频 map 逻辑。
- 当读取 SSTable block 时，系统应复用临时 frame buffer，且返回给上层的解码结果不能引用已归还的临时 buffer。
- 当写入 SSTable block 时，系统应避免每个 block 都 seek 文件末尾，并继续复用临时 frame buffer。
- 当写入 series 时间轴时，系统应对常见有序列采用线性去重，避免为时间戳集合创建临时 map。

## 设计

查询合并阶段先按 `(series_id, field_id)` 排序 column 引用，再按相邻分组处理。对组内每条 samples 已按 timestamp 有序的场景，使用 k-way 归并按 timestamp 输出，并在相同 timestamp 中选择最大 `WriteSeq`。只有发现乱序输入时才使用现有 map 去重和排序兜底。

SSTable block 读取阶段新增可释放的 block payload 包装。`readBlockBufferFrom` 负责借用 frame、校验长度和 CRC，并返回 payload 视图以及释放函数；调用方在 decode 完成后释放 frame。保留原 `readBlockFrom` 行为用于测试和兼容调用。

SSTable block 写入阶段新增 `blockWriter`，在打开文件时记录顺序写 offset，`writeBlock` 保留给测试和单次写入，批量写入路径改用 `blockWriter.write`，避免每个 block 调用 `Seek(End)`。

写入分组和时间戳收集阶段减少临时 map。`groupColumns` 先排序 column，再按 series 连续聚合；`collectTimestamps` 对已排序 samples 使用多路合并去重，只有乱序输入才退回 map。

## 验证

定向测试覆盖有序合并、乱序 fallback、相同 timestamp 最新版本、block buffer 生命周期、block writer 顺序 offset。最终执行格式化、全量单元测试、覆盖率、lint 和 `tests/e2e`。
