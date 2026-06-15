# e2e

每个端到端用例放在独立子目录中，并实现为 `main` 包。运行方式：

```bash
cd tests/e2e/simple_integrity
go build -o simple_integrity .
./simple_integrity
rm -f simple_integrity
```

约定：用例只导入 `codeberg.org/mts/mts` 公共 API，失败时返回非零退出码。
