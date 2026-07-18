# MTS vs VM 10M 内容一致性校验

- 日期：2026-07-18 16:25 
- 目标：构造内容一致的 10M 数据（时间戳可不同），交叉查询校验 MTS 是否出现基础功能错误
- 结果：**通过**（`passed=true`）

## 数据构造（逻辑内容一致）

| 项 | 值 |
|---|---|
| 点数 | 10,000,000 |
| series | 100（`host-000`..`host-099`） |
| measurement | `scale` |
| 字段 | `f0..f4,i0..i2,s0,b0` |
| host | `host-%03d(index%100)` |
| f0..f4 | `index`, `index*1.1`..`index*1.4` |
| i0..i2 | `index`, `index+1`, `index+2` |
| s0/b0 | `"ok"` / `index%2==0` |
| MTS 时间戳 | `index * 1s`（ns） |
| VM 时间戳 | `(now-points+index) * 1s`（ns） |

> 时间戳故意不同（避免 VM 丢弃过旧/过新时间戳）；一致性按逻辑主键 `index` 比对字段内容。

## 写入结果

| 引擎 | 写入耗时 | 说明 |
|---|---:|---|
| MTS | 32.115s | WriteTypedBatch + flush + compact（snappy） |
| VM | 30.569s | Influx line `/write?precision=ns` |

## 查询一致性结果

| 窗口 | 期望行数 | MTS 行数 | VM 行数 | MTS 错 | VM 错 | 交叉错 |
|---|---:|---:|---:|---:|---:|---:|
| head | 2000 | 2000 | 2000 | 0 | 0 | 0 |
| middle | 2000 | 2000 | 2000 | 0 | 0 | 0 |
| tail | 2000 | 2000 | 2000 | 0 | 0 | 0 |
| host_spot_host-000 | 20 | 20 | 20 | 0 | 0 | 0 |
| host_spot_host-001 | 20 | 20 | 20 | 0 | 0 | 0 |
| host_spot_host-002 | 20 | 20 | 20 | 0 | 0 | 0 |
| host_spot_host-003 | 20 | 20 | 20 | 0 | 0 | 0 |
| host_spot_host-004 | 20 | 20 | 20 | 0 | 0 | 0 |

### 汇总

| 指标 | 值 |
|---|---:|
| total_mts_mismatch | **0** |
| total_vm_mismatch | **0** |
| total_cross_mismatch | **0** |
| passed | **True** |

## 校验口径

1. **MTS 侧**：对期望公式全字段校验（含 `s0` 字符串）。
2. **VM 侧**：校验 host + 数值/布尔字段（`f0..f4,i0..i2,b0`）。
3. **交叉比对**：同一 `index` 下 MTS/VM 的 host+数值/布尔字段一致。
4. **已知模型差**：
   - Prometheus/VM 不保留字符串 metric 值（`s0` 导出为 `0`），故 `s0` 仅 MTS 强制校验。
   - 时间戳数值不同是设计如此，不计入不一致。

## 排查过程（过程问题，非 MTS 功能缺陷）

1. 首轮 VM 用 `now-48h` 作 base → 尾部落入未来，触发 `big_timestamp` 丢点。
2. 修正 base 为 `now-points` 后，100M samples 全量入库（`ignored=0`）。
3. 比对器曾把 VM 导出的科学计数法整数（如 `5e+06`）误解析为 0；修复 `parseIntish` 后 **0 差异**。

## 结论

在 10M 同逻辑内容下：

- **MTS 查询内容与期望公式完全一致**（head/middle/tail + 5 个 host 抽检，0 mismatch）
- **MTS 与 VM 交叉可比字段完全一致**（0 cross mismatch）
- **未发现 MTS 基础读写内容正确性问题**

## 清理

- 容器 `mts-vm-content` 与 `/tmp/mts-vm-content-compare*` 测试数据在报告落盘后清理。
