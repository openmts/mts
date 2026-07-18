package engine

import "sync"

const (
	compactionSkipDuplicateCandidate = "duplicate_candidate"
	compactionSkipMemoryBusy         = "memory_busy"
	compactionSkipConcurrencyLimit   = "concurrency_limit"
)

type compactionScheduler struct {
	mu            sync.Mutex
	maxConcurrent int
	inFlight      map[string]struct{}
	stats         compactionSchedulerSnapshot
}

type compactionSchedulerSnapshot struct {
	TotalSkips       int
	DuplicateSkips   int
	MemorySkips      int
	ConcurrencySkips int
	LastSkipReason   string
	InFlightCompacts int
	MaxConcurrent    int
}

func newCompactionScheduler(maxConcurrent int) *compactionScheduler {
	if maxConcurrent <= 0 {
		maxConcurrent = defaultMaxConcurrentCompaction
	}
	return &compactionScheduler{
		maxConcurrent: maxConcurrent,
		inFlight:      make(map[string]struct{}),
		stats: compactionSchedulerSnapshot{
			MaxConcurrent: maxConcurrent,
		},
	}
}

func (s *compactionScheduler) start(signature string) bool {
	if s == nil || signature == "" {
		return true
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.inFlight[signature]; ok {
		s.recordSkipLocked(compactionSkipDuplicateCandidate)
		return false
	}
	if s.maxConcurrent > 0 && len(s.inFlight) >= s.maxConcurrent {
		s.recordSkipLocked(compactionSkipConcurrencyLimit)
		return false
	}
	s.inFlight[signature] = struct{}{}
	s.stats.InFlightCompacts = len(s.inFlight)
	return true
}

func (s *compactionScheduler) finish(signature string) {
	if s == nil || signature == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.inFlight, signature)
	s.stats.InFlightCompacts = len(s.inFlight)
}

func (s *compactionScheduler) recordSkip(reason string) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.recordSkipLocked(reason)
}

func (s *compactionScheduler) recordSkipLocked(reason string) {
	s.stats.TotalSkips++
	s.stats.LastSkipReason = reason
	switch reason {
	case compactionSkipDuplicateCandidate:
		s.stats.DuplicateSkips++
	case compactionSkipMemoryBusy:
		s.stats.MemorySkips++
	case compactionSkipConcurrencyLimit:
		s.stats.ConcurrencySkips++
	}
}

func (s *compactionScheduler) snapshotCopy() compactionSchedulerSnapshot {
	if s == nil {
		return compactionSchedulerSnapshot{}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	out := s.stats
	out.InFlightCompacts = len(s.inFlight)
	out.MaxConcurrent = s.maxConcurrent
	return out
}

func (s *compactionScheduler) snapshot() compactionSchedulerSnapshot {
	return s.snapshotCopy()
}
