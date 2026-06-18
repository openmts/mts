//go:build windows

package engine

import (
	"syscall"
	"unsafe"
)

const maxInt64 = int64(^uint64(0) >> 1)

var getDiskFreeSpaceEx = syscall.NewLazyDLL("kernel32.dll").NewProc("GetDiskFreeSpaceExW")

func availableBytes(path string) (int64, error) {
	pointer, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return 0, err
	}
	var available uint64
	result, _, err := getDiskFreeSpaceEx.Call(
		uintptr(unsafe.Pointer(pointer)),
		uintptr(unsafe.Pointer(&available)),
		0,
		0,
	)
	if result == 0 {
		if err != syscall.Errno(0) {
			return 0, err
		}
		return 0, syscall.EINVAL
	}
	if available > uint64(maxInt64) {
		return maxInt64, nil
	}
	return int64(available), nil
}
