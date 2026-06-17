# e2e

每个端到端用例放在独立子目录中，并实现为 `main` 包。运行方式：

```bash
cd tests/e2e/simple_integrity
go build -o testbin .
timeout 120s ./testbin
rm -f testbin
```

约定：用例只导入 `codeberg.org/mts/mts` 公共 API，失败时返回非零退出码。

新增覆盖：

- `streaming_query`：流式列查询大结果迭代。
- `query_aggregate_window`：聚合、窗口和边界。
- `read_amplification`：读预算超限错误。
- `service_ops`：metrics、health、ready、admin compact。
