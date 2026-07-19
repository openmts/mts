//go:build linux

package main

import (
	"os"
	"strconv"
	"strings"
	"syscall"
)

func rssPeakBytes() int64 {
	var usage syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &usage); err != nil {
		return 0
	}
	return usage.Maxrss * 1024
}

// currentRSSBytes 读取进程当前 RSS（VmRSS），用于分阶段内存观测。
// 与 rssPeakBytes（MaxRSS 峰值）互补：峰值单调不减，当前值可反映阶段后瞬时占用。
func currentRSSBytes() int64 {
	data, err := os.ReadFile("/proc/self/status")
	if err != nil {
		return 0
	}
	for _, line := range strings.Split(string(data), "\n") {
		if !strings.HasPrefix(line, "VmRSS:") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			return 0
		}
		kb, err := strconv.ParseInt(fields[1], 10, 64)
		if err != nil {
			return 0
		}
		return kb * 1024
	}
	return 0
}
