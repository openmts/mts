package catalog

import (
	"sort"
	"strings"
)

func seriesKey(measurement string, tags map[string]string) string {
	if len(tags) == 0 {
		return measurement
	}
	if len(tags) == 1 {
		for key, value := range tags {
			return measurement + "\xff" + key + "=" + value
		}
	}
	parts := make([]string, 0, len(tags)+1)
	parts = append(parts, measurement)
	keys := make([]string, 0, len(tags))
	for key := range tags {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		parts = append(parts, key+"="+tags[key])
	}
	return strings.Join(parts, "\xff")
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
