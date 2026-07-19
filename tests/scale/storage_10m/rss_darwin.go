//go:build darwin

package main

import "syscall"

func rssPeakBytes() int64 {
	var usage syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &usage); err != nil {
		return 0
	}
	return usage.Maxrss
}

func currentRSSBytes() int64 {
	return rssPeakBytes()
}
