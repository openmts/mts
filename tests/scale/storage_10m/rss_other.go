//go:build !linux && !darwin

package main

func rssPeakBytes() int64 {
	return 0
}

func currentRSSBytes() int64 {
	return 0
}
