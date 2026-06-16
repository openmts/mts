package catalog

import (
	"sort"
	"strings"
)

func seriesKey(measurement string, tags map[string]string) string {
	key, _ := seriesKeyWithScratch(measurement, tags, nil)
	return key
}

func seriesKeyWithScratch(measurement string, tags map[string]string, scratch []string) (string, []string) {
	if len(tags) == 0 {
		return measurement, scratch
	}
	if len(tags) == 1 {
		for key, value := range tags {
			return measurement + "\xff" + key + "=" + value, scratch
		}
	}
	scratch = scratch[:0]
	size := len(measurement)
	for key, value := range tags {
		scratch = append(scratch, key)
		size += 1 + len(key) + 1 + len(value)
	}
	sort.Strings(scratch)
	var builder strings.Builder
	builder.Grow(size)
	builder.WriteString(measurement)
	for _, key := range scratch {
		builder.WriteByte('\xff')
		builder.WriteString(key)
		builder.WriteByte('=')
		builder.WriteString(tags[key])
	}
	return builder.String(), scratch
}

func fieldKey(measurement string, name string) string {
	return measurement + "\xff" + name
}

func cloneTags(tags map[string]string) map[string]string {
	out := make(map[string]string, len(tags))
	for key, value := range tags {
		out[key] = value
	}
	return out
}

func tagsMatch(seriesTags map[string]string, filter map[string]string) bool {
	for key, want := range filter {
		if seriesTags[key] != want {
			return false
		}
	}
	return true
}
