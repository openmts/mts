package main

import (
	"path/filepath"
	"strconv"
)

func buildCases(cfg matrixConfig) []matrixCase {
	total := len(cfg.Sizes) * len(cfg.Compressions) * len(cfg.Durabilities)
	out := make([]matrixCase, 0, total)
	for _, size := range cfg.Sizes {
		for _, compression := range cfg.Compressions {
			for _, durability := range cfg.Durabilities {
				out = append(out, matrixCase{
					Size:        size.Name,
					Points:      size.Points,
					Compression: compression,
					Durability:  durability,
					DataDir: filepath.Join(
						cfg.DataRoot,
						size.Name,
						compression,
						durability,
					),
				})
			}
		}
	}
	return out
}

func runnerArgs(cfg matrixConfig, item matrixCase) []string {
	return []string{
		"-mode", cfg.Mode,
		"-points", strconv.Itoa(item.Points),
		"-batch-size", strconv.Itoa(cfg.BatchSize),
		"-ingest-path", cfg.IngestPath,
		"-memtable-max-samples", strconv.Itoa(cfg.MemTableLimit),
		"-compression-algorithm", item.Compression,
		"-durability", item.Durability,
		"-query-limit", strconv.Itoa(cfg.QueryLimit),
		"-shard-duration", cfg.ShardDuration.String(),
		"-timestamp-step", cfg.TimestampStep.String(),
		"-data-dir", item.DataDir,
	}
}
