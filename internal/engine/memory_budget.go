package engine

import (
	"errors"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"

	"codeberg.org/mts/mts/internal/model"
)

var ErrStorageMemoryLimitExceeded = errors.New("storage memory limit exceeded")

type storageMemoryKind string

const (
	storageMemoryWrite       storageMemoryKind = "write"
	storageMemoryFlush       storageMemoryKind = "flush"
	storageMemoryQuery       storageMemoryKind = "query"
	storageMemoryCompaction  storageMemoryKind = "compaction"
	storageMemoryCompression storageMemoryKind = "compression"
)

type StorageMemorySnapshot struct {
	CurrentBytes          int64
	PeakBytes             int64
	ActiveBytes           int64
	MemTableBytes         int64
	WALBytes              int64
	ReservationBytes      int64
	WriteBytes            int64
	FlushBytes            int64
	QueryBytes            int64
	CompactionBytes       int64
	CompressionBytes      int64
	SoftBytesLimit        int64
	HardBytesLimit        int64
	RejectedWrites        uint64
	RejectedReservations  uint64
	FlushTriggered        uint64
	QueryBytesLimit       int64
	FlushBytesLimit       int64
	CompactionBytesLimit  int64
	CompressionBytesLimit int64
	RuntimeHeapAllocBytes int64
	RuntimeRSSBytes       int64
	RuntimeGapBytes       int64
}

type storageMemoryActive struct {
	MemTableBytes int64
	WALBytes      int64
}

func (a storageMemoryActive) total() int64 {
	return a.MemTableBytes + a.WALBytes
}

type storageMemoryLimiter struct {
	mu                   sync.Mutex
	opts                 model.StorageMemoryOptions
	reservations         map[storageMemoryKind]int64
	totalReserved        int64
	peakBytes            int64
	rejected             uint64
	rejectedReservations uint64
	flushTriggered       uint64
}

type storageMemoryReservation struct {
	limiter *storageMemoryLimiter
	kind    storageMemoryKind
	bytes   int64
	once    sync.Once
}

type storageCompressionBudget struct {
	memory *storageMemoryLimiter
}

func newStorageMemoryLimiter(opts model.StorageMemoryOptions) *storageMemoryLimiter {
	return &storageMemoryLimiter{
		opts:         opts,
		reservations: make(map[storageMemoryKind]int64, 4),
	}
}

func (l *storageMemoryLimiter) Reserve(
	kind storageMemoryKind,
	activeBytes int64,
	bytes int64,
) (func(), error) {
	if l == nil || bytes <= 0 {
		return func() {}, nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if err := l.checkLocked(kind, activeBytes, bytes); err != nil {
		l.recordRejectLocked(kind)
		return nil, err
	}
	l.reservations[kind] += bytes
	l.totalReserved += bytes
	l.updatePeakLocked(activeBytes)
	reservation := &storageMemoryReservation{limiter: l, kind: kind, bytes: bytes}
	return reservation.Release, nil
}

func (l *storageMemoryLimiter) Snapshot(active storageMemoryActive) StorageMemorySnapshot {
	activeBytes := active.total()
	runtimeSnapshot := runtimeStorageMemorySnapshot(activeBytes)
	if l == nil {
		runtimeSnapshot.ActiveBytes = activeBytes
		runtimeSnapshot.CurrentBytes = activeBytes
		runtimeSnapshot.MemTableBytes = active.MemTableBytes
		runtimeSnapshot.WALBytes = active.WALBytes
		return runtimeSnapshot
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	current := activeBytes + l.totalReserved
	if current > l.peakBytes {
		l.peakBytes = current
	}
	runtimeSnapshot.CurrentBytes = current
	runtimeSnapshot.PeakBytes = l.peakBytes
	runtimeSnapshot.ActiveBytes = activeBytes
	runtimeSnapshot.MemTableBytes = active.MemTableBytes
	runtimeSnapshot.WALBytes = active.WALBytes
	runtimeSnapshot.ReservationBytes = l.totalReserved
	runtimeSnapshot.WriteBytes = l.reservations[storageMemoryWrite]
	runtimeSnapshot.FlushBytes = l.reservations[storageMemoryFlush]
	runtimeSnapshot.QueryBytes = l.reservations[storageMemoryQuery]
	runtimeSnapshot.CompactionBytes = l.reservations[storageMemoryCompaction]
	runtimeSnapshot.CompressionBytes = l.reservations[storageMemoryCompression]
	runtimeSnapshot.SoftBytesLimit = l.opts.SoftBytesLimit
	runtimeSnapshot.HardBytesLimit = l.opts.HardBytesLimit
	runtimeSnapshot.RejectedWrites = l.rejected
	runtimeSnapshot.RejectedReservations = l.rejectedReservations
	runtimeSnapshot.FlushTriggered = l.flushTriggered
	runtimeSnapshot.QueryBytesLimit = l.opts.QueryBytesLimit
	runtimeSnapshot.FlushBytesLimit = l.opts.FlushBytesLimit
	runtimeSnapshot.CompactionBytesLimit = l.opts.CompactionBytesLimit
	runtimeSnapshot.CompressionBytesLimit = l.opts.CompressionBytesLimit
	return runtimeSnapshot
}

func (l *storageMemoryLimiter) RecordFlushTriggered() {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.flushTriggered++
}

func (l *storageMemoryLimiter) checkLocked(
	kind storageMemoryKind,
	activeBytes int64,
	bytes int64,
) error {
	if limit := l.operationLimit(kind); limit > 0 && l.reservations[kind]+bytes > limit {
		return storageMemoryLimitError(kind, l.reservations[kind]+bytes, limit)
	}
	if limit := l.opts.HardBytesLimit; limit > 0 && activeBytes+l.totalReserved+bytes > limit {
		return storageMemoryLimitError(kind, activeBytes+l.totalReserved+bytes, limit)
	}
	return nil
}

func (l *storageMemoryLimiter) operationLimit(kind storageMemoryKind) int64 {
	switch kind {
	case storageMemoryQuery:
		return l.opts.QueryBytesLimit
	case storageMemoryFlush:
		return l.opts.FlushBytesLimit
	case storageMemoryCompaction:
		return l.opts.CompactionBytesLimit
	case storageMemoryCompression:
		return l.opts.CompressionBytesLimit
	default:
		return 0
	}
}

func (l *storageMemoryLimiter) recordRejectLocked(kind storageMemoryKind) {
	if kind == storageMemoryWrite {
		l.rejected++
		return
	}
	l.rejectedReservations++
}

func (l *storageMemoryLimiter) updatePeakLocked(activeBytes int64) {
	current := activeBytes + l.totalReserved
	if current > l.peakBytes {
		l.peakBytes = current
	}
}

func (r *storageMemoryReservation) Release() {
	if r == nil || r.limiter == nil || r.bytes <= 0 {
		return
	}
	r.once.Do(func() {
		r.limiter.mu.Lock()
		defer r.limiter.mu.Unlock()
		current := r.limiter.reservations[r.kind]
		if current <= r.bytes {
			delete(r.limiter.reservations, r.kind)
		} else {
			r.limiter.reservations[r.kind] = current - r.bytes
		}
		if r.limiter.totalReserved <= r.bytes {
			r.limiter.totalReserved = 0
		} else {
			r.limiter.totalReserved -= r.bytes
		}
	})
}

func storageMemoryLimitError(kind storageMemoryKind, actual int64, limit int64) error {
	return fmt.Errorf("%w: kind=%s actual_bytes=%d limit_bytes=%d", ErrStorageMemoryLimitExceeded, kind, actual, limit)
}

func (b storageCompressionBudget) ReserveCompressionBytes(bytes int64) (func(), error) {
	if b.memory == nil {
		return func() {}, nil
	}
	return b.memory.Reserve(storageMemoryCompression, 0, bytes)
}

func runtimeStorageMemorySnapshot(engineBytes int64) StorageMemorySnapshot {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	rss := readRuntimeRSSBytes()
	if rss == 0 {
		rss = int64(mem.HeapAlloc)
	}
	gap := rss - engineBytes
	if gap < 0 {
		gap = 0
	}
	return StorageMemorySnapshot{
		RuntimeHeapAllocBytes: int64(mem.HeapAlloc),
		RuntimeRSSBytes:       rss,
		RuntimeGapBytes:       gap,
	}
}

func readRuntimeRSSBytes() int64 {
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
