//go:build !linux

package engine

func readRuntimeRSSBytes() int64 {
	return 0
}
