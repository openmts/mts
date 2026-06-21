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
		writeMetricLabels(&builder, metric.Labels)
		builder.WriteByte(' ')
		_, _ = fmt.Fprintf(&builder, "%g", metric.Value)
		builder.WriteByte('\n')
	}
	return builder.String()
}

func sortMetrics(metrics []Metric) {
	sort.Slice(metrics, func(i, j int) bool {
		return metricKey(metrics[i].Name, metrics[i].Labels) <
			metricKey(metrics[j].Name, metrics[j].Labels)
	})
}

func metricKey(name string, labels map[string]string) string {
	if len(labels) == 0 {
		return name
	}
	var builder strings.Builder
	builder.WriteString(name)
	for _, label := range sortedLabelNames(labels) {
		builder.WriteByte(0)
		builder.WriteString(label)
		builder.WriteByte('=')
		builder.WriteString(labels[label])
	}
	return builder.String()
}

func writeMetricLabels(builder *strings.Builder, labels map[string]string) {
	if len(labels) == 0 {
		return
	}
	builder.WriteByte('{')
	for index, label := range sortedLabelNames(labels) {
		if index > 0 {
			builder.WriteByte(',')
		}
		builder.WriteString(label)
		builder.WriteString("=\"")
		builder.WriteString(escapeLabelValue(labels[label]))
		builder.WriteByte('"')
	}
	builder.WriteByte('}')
}

func sortedLabelNames(labels map[string]string) []string {
	names := make([]string, 0, len(labels))
	for name := range labels {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func escapeLabelValue(value string) string {
	value = strings.ReplaceAll(value, "\\", "\\\\")
	value = strings.ReplaceAll(value, "\n", "\\n")
	return strings.ReplaceAll(value, "\"", "\\\"")
}
