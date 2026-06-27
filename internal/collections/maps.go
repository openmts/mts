package collections

import (
	"cmp"
	"slices"
)

func CloneMap[K comparable, V any](values map[K]V) map[K]V {
	if values == nil {
		return nil
	}
	out := make(map[K]V, len(values))
	for key, value := range values {
		out[key] = value
	}
	return out
}

func CloneMapNilIfEmpty[K comparable, V any](values map[K]V) map[K]V {
	if len(values) == 0 {
		return nil
	}
	return CloneMap(values)
}

func CloneSlice[T any](values []T) []T {
	if values == nil {
		return nil
	}
	return slices.Clone(values)
}

func CloneSliceNilIfEmpty[T any](values []T) []T {
	if len(values) == 0 {
		return nil
	}
	return slices.Clone(values)
}

func SortedKeys[K cmp.Ordered, V any](values map[K]V) []K {
	keys := make([]K, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys
}
