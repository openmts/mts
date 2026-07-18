# MTS vs VictoriaMetrics 10M 读写对比

- 日期：2026-07-18 14:56 
- 镜像：`docker.io/victoriametrics/victoria-metrics:latest`（podman 拉取，代理 `http://127.0.0.1:1080`）
- 规模：10,000,000 条逻辑点（每点 10 字段，100 个 host series）
- 数据模型对齐 `tests/scale/storage_10m`：
  - measurement=`scale`
  - tag=`host`（`host-000`..`host-099`）
  - fields=`f0..f4,i0..i2,s0,b0`
  - timestamp=`index * 1s`（纳秒精度）
  - batch_size=4096

## 测试方法

### MTS
- 命令：`go run ./tests/scale/storage_10m -points 10000000 -batch-size 4096 -mode write-query-compact -ingest-path typed -durability buffered -verify=false`
- 写路径：进程内 `WriteTypedBatch`
- 查询：中间窗口 2000 行 `QueryRowIterator`（整行 10 字段）
- 含 flush + full compact

### VictoriaMetrics
- 容器：`podman run ... -retentionPeriod=100y -httpListenAddr=:8428`
- 写路径：HTTP Influx line protocol `/write?precision=ns`（每逻辑点 1 行、10 field → 共 100M samples）
- 查询：同时间窗 export `scale_f0`（2000 samples）；另测 10 字段 export
- 注意：默认 retention 会丢弃 1970 附近时间戳，本轮必须 `-retentionPeriod=100y`

## 结果摘要

| 指标 | MTS | VictoriaMetrics | MTS/VM |
|---|---:|---:|---:|
| 写入耗时 | 20.863s | 29.267s | 0.71x |
| 写入吞吐 | 479316 pts/s | 341677 pts/s | 1.40x |
| 冷查询（~2000 行/点） | 16.40ms | 6.82ms（单字段 export） | 2.41x |
| 热查询 | 17.91ms | 2.73ms | 6.56x |
| 数据体积 | 1062.1 MiB | 9.3 MiB | 114.2x |
| MTS compact | 5.405s | n/a（后台 merge） | - |
| MTS RSS peak | 253.8 MiB | 容器进程另计 | - |

VM 多字段 export 诊断：`vm multi-field export samples=20000 duration=25.660346ms hot_rows=2000 hot_samples=2000`

## 解读

1. **写入吞吐**：本轮 MTS 进程内 TypedBatch 写路径约 **1.40x** 高于 VM 的 HTTP Influx 写入（MTS ~479k pts/s vs VM ~342k pts/s）。差距包含协议/解析成本差异，不能直接等同“存储引擎内核”纯算力对比。
2. **查询延迟**：同窗口 2000 点，MTS 整行查询约 **16.4ms**，VM 单字段 export 冷查约 **6.8ms**、热查约 **2.7ms**；MTS 冷查约为 VM 单字段冷查的 **2.4x**。VM 10 字段 export 约 25.7ms，接近/略慢于 MTS 整行查询量级。
3. **存储占用**：MTS 约 **1.06 GiB**，VM 约 **9.3 MiB**（约 **114x** 差距）。VM 在压缩与列式编码上显著领先；MTS 本轮 compression=off，且 POC 格式未做极致压缩。
4. **可比性边界**：
   - MTS：本地库内 API；VM：跨 HTTP 文本协议。
   - MTS 查询返回多字段行；VM 主指标用单字段 export，多字段另测。
   - 时间戳从 epoch 起，VM 需超长 retention 才不丢点。
   - 单机、同机、一次性压测，非长时间稳态。

## 结论

在 **相同逻辑数据模型 / 10M 点** 下：
- **写入**：MTS（TypedBatch 本地）略快于 VM（HTTP Influx）。
- **点查/窗口导出**：VM 更优（尤其热查与压缩后扫描）。
- **磁盘**：VM 压缩优势极大，MTS 仍有数量级差距。

后续若要更公平的内核对比，建议：
1. 为 VM 增加 native/protobuf 写或本机 embed 路径；
2. MTS 打开压缩并固定 flush/compact 策略后复测；
3. 查询统一为“单字段扫描 / 多字段投影 / 全表导出”三组；
4. 增加 100M 与高基数场景。

## 产物与清理

- 原始报告：`/tmp/mts-vm-compare/reports/mts-10m.json`、`vm-10m.json`（测试后清理）
- 容器名：`mts-vm-bench`
