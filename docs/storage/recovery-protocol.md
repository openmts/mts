# mts 存储恢复协议

## 正常启动

1. Engine 校验已存在 root、data root、shard 目录权限不得宽于 `0700`。
2. Shard 读取 `MANIFEST.bin`，加载 Manifest 引用的 SSTable part。
3. `OpenPart` 校验 metadata、component、block ref 和 block CRC。
4. WAL replay 校验 segment header、record length 和 record CRC，并恢复未 flush 数据。

## 异常重启

WAL header 损坏、未知 format 或中间 record CRC 错误会终止恢复并返回明确错误。最后一个 WAL segment 的尾部 partial record 会被截断，已完整提交的 record 保留。

如果 flush 已写出 part 但 Manifest 未提交，该 part 视为孤儿，启动恢复会通过 maintenance 清理或由离线 repair 清理。Manifest 已提交但 WAL checkpoint 未完成时，启动会加载 Manifest part 并 replay WAL，查询合并逻辑按 write sequence 去重。

## 离线检查与修复

`mts-storage check <path>` 扫描 WAL、Manifest、SSTable part 和未知文件，输出 JSON report。报告包含 path、part id、level、time range、series range、reason、offset 和 block type。

`mts-storage repair --dry-run <path>` 只输出安全修复计划。`mts-storage repair --apply <path>` 当前只删除明确识别的孤儿 part，不修复 checksum 错误，不删除 Manifest 引用缺失的 part。

未知文件通过 `--unknown-files=ignore|warn|fatal` 控制。

## 迁移协议

`mts-storage migrate --dry-run <path>` 输出将要生成的 Manifest 备份和 checkpoint 路径。

`mts-storage migrate --apply <path>` 先备份 `MANIFEST.bin` 到 `MANIFEST.bin.bak`，再写入二进制 `MIGRATION.checkpoint`。如果进程中断，再次执行会识别 checkpoint 并以 resume 结果返回，避免重复执行不确定动作。

## 权限要求

所有新建目录使用 `0700`，普通文件使用 `0600`。打开已存在目录时不会自动放宽或静默修正过宽权限，而是返回错误，避免在不安全环境中继续运行。
