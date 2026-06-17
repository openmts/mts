package observability

import (
	"fmt"
	"sort"
	"strings"
)

func PrometheusText(metrics []Metric) string {
	sortMetrics(metrics)
	var builder strings.Builder
	for _, metric := range metrics {
		builder.WriteString("# HELP ")
		builder.WriteString(metric.Name)
		builder.WriteByte(' ')
		builder.WriteString(metric.Help)
		builder.WriteByte('\n')
		builder.WriteString("# TYPE ")
		builder.WriteString(metric.Name)
		builder.WriteByte(' ')
		builder.WriteString(metric.Type)
		builder.WriteByte('\n')
		builder.WriteString(metric.Name)
		builder.WriteByte(' ')
		_, _ = fmt.Fprintf(&builder, "%g", metric.Value)
		builder.WriteByte('\n')
	}
	return builder.String()
}

func sortMetrics(metrics []Metric) {
	sort.Slice(metrics, func(i, j int) bool {
		return metrics[i].Name < metrics[j].Name
	})
}
