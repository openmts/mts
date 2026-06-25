# e2e

每个端到端用例放在独立子目录中，并实现为 `main` 包。运行方式：

```bash
cd tests/e2e/simple_integrity
go build -o testbin .
timeout 120s ./testbin
rm -f testbin
```

约定：用例只导入 `github.com/openmts/mts` 公共 API，失败时返回非零退出码。

新增覆盖：

- `public_api_workflow`：公开 typed batch、Builder、Row/Column iterator、元数据列表和跨重启读取。
- `mts_server_protocols`：启动真实 `mts-server` 进程，验证 HTTP 和 gRPC 协议下 health、write/typed write、rows/columns/explain query、用户权限、配置、metrics、flush、compact 和 downsample dry-run 可用性。
- `streaming_query`：流式列查询大结果迭代。
- `query_aggregate_window`：聚合、窗口和边界。
- `read_amplification`：读预算超限错误。
- `service_ops`：metrics、health、ready、admin compact。
