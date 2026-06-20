package main

import (
	"flag"
	"fmt"
	"strconv"
	"strings"
	"time"
)

func parseConfig(args []string) (matrixConfig, error) {
	flags := flag.NewFlagSet("storage_matrix", flag.ContinueOnError)
	sizes := flags.String("sizes", defaultSizes, "comma-separated sizes: 100k,1m,10m")
	compressions := flags.String("compressions", defaultCompressions, "comma-separated compression algorithms")
	durabilities := flags.String("durabilities", defaultDurabilities, "comma-separated durability modes")
	dataRoot := flags.String("data-root", "", "matrix data root; temp dir when empty")
	runner := flags.String("runner", "", "storage_10m runner binary; built automatically when empty")
	out := flags.String("out", "", "matrix JSON output path")
	markdown := flags.String("markdown", "", "matrix markdown output path")
	mode := flags.String("mode", "compact", "storage_10m mode")
	ingestPath := flags.String("ingest-path", "typed", "storage_10m ingest path")
	batchSize := flags.Int("batch-size", 4096, "write batch size")
	memTableLimit := flags.Int("memtable-max-samples", 8192, "memtable max samples")
	queryLimit := flags.Int("query-limit", 2000, "query limit")
	shardDuration := flags.Duration("shard-duration", 24*time.Hour, "storage shard duration")
	timestampStep := flags.Duration("timestamp-step", time.Second, "logical timestamp interval between generated rows")
	caseTimeout := flags.Duration("case-timeout", 20*time.Minute, "timeout per matrix case")
	if err := flags.Parse(args); err != nil {
		return matrixConfig{}, err
	}
	parsedSizes, err := parseSizes(*sizes)
	if err != nil {
		return matrixConfig{}, err
	}
	parsedCompressions, err := parseList(*compressions, validCompression)
	if err != nil {
		return matrixConfig{}, fmt.Errorf("compressions: %w", err)
	}
	parsedDurabilities, err := parseList(*durabilities, validDurability)
	if err != nil {
		return matrixConfig{}, fmt.Errorf("durabilities: %w", err)
	}
	if *batchSize <= 0 || *memTableLimit <= 0 ||
		*queryLimit <= 0 || *shardDuration <= 0 ||
		*timestampStep <= 0 || *caseTimeout <= 0 {
		return matrixConfig{}, fmt.Errorf("batch-size, memtable-max-samples, query-limit, shard-duration, timestamp-step and case-timeout must be positive")
	}
	return matrixConfig{
		Sizes:         parsedSizes,
		Compressions:  parsedCompressions,
		Durabilities:  parsedDurabilities,
		DataRoot:      *dataRoot,
		Runner:        *runner,
		Out:           *out,
		Markdown:      *markdown,
		Mode:          *mode,
		IngestPath:    *ingestPath,
		BatchSize:     *batchSize,
		MemTableLimit: *memTableLimit,
		QueryLimit:    *queryLimit,
		ShardDuration: *shardDuration,
		TimestampStep: *timestampStep,
		CaseTimeout:   *caseTimeout,
	}, nil
}

func parseSizes(input string) ([]scaleSize, error) {
	values, err := parseList(input, validSizeName)
	if err != nil {
		return nil, err
	}
	out := make([]scaleSize, 0, len(values))
	for _, value := range values {
		size, err := sizeByName(value)
		if err != nil {
			return nil, err
		}
		out = append(out, size)
	}
	return out, nil
}

func parseList(input string, valid func(string) bool) ([]string, error) {
	parts := strings.Split(input, ",")
	out := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		value := strings.TrimSpace(strings.ToLower(part))
		if value == "" {
			continue
		}
		if !valid(value) {
			return nil, fmt.Errorf("unsupported value %q", value)
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("empty list")
	}
	return out, nil
}

func validSizeName(value string) bool {
	_, err := sizeByName(value)
	return err == nil
}

func sizeByName(value string) (scaleSize, error) {
	switch value {
	case "100k":
		return scaleSize{Name: value, Points: 100_000}, nil
	case "1m":
		return scaleSize{Name: value, Points: 1_000_000}, nil
	case "10m":
		return scaleSize{Name: value, Points: 10_000_000}, nil
	default:
		points, err := strconv.Atoi(value)
		if err == nil && points > 0 {
			return scaleSize{Name: value, Points: points}, nil
		}
		return scaleSize{}, fmt.Errorf("unsupported size %q", value)
	}
}

func validCompression(value string) bool {
	switch value {
	case "off", "none", "snappy", "lz4", "zstd":
		return true
	default:
		return false
	}
}

func validDurability(value string) bool {
	switch value {
	case "buffered", "wal-sync", "write-sync", "strict-flush":
		return true
	default:
		return false
	}
}
