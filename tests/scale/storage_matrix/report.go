package main

import (
	"fmt"
	"strings"
	"time"
)

func markdownReport(report matrixReport) string {
	var builder strings.Builder
	builder.WriteString("# MTS Storage Performance Matrix\n\n")
	builder.WriteString("| size | compression | durability | status | write | compaction | cold query | hot query | rss peak | data bytes | shards | sstable before | sstable after |\n")
	builder.WriteString("| --- | --- | --- | --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |\n")
	for _, item := range report.Cases {
		status := "ok"
		if item.Error != "" {
			status = item.Error
		}
		fmt.Fprintf(
			&builder,
			"| %s | %s | %s | %s | %s | %s | %s | %s | %s | %s | %d | %d | %d |\n",
			item.Case.Size,
			item.Case.Compression,
			item.Case.Durability,
			status,
			formatDuration(item.Report.WriteDurationNanos),
			formatDuration(item.Report.CompactionDurationNanos),
			formatDuration(item.Report.ColdQueryLatency),
			formatDuration(item.Report.HotQueryLatency),
			formatBytes(item.Report.RSSPeakBytes),
			formatBytes(item.Report.DataBytes),
			item.Report.ShardCount,
			item.Report.SSTableBefore,
			item.Report.SSTableAfter,
		)
	}
	return builder.String()
}

func formatDuration(nanos int64) string {
	if nanos <= 0 {
		return "0s"
	}
	return time.Duration(nanos).String()
}

func formatBytes(bytes int64) string {
	const mib = 1 << 20
	if bytes <= 0 {
		return "0MiB"
	}
	return fmt.Sprintf("%.1fMiB", float64(bytes)/mib)
}
