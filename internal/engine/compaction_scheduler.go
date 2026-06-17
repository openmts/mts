package engine

import "sync"

const (
	compactionSkipDuplicateCandidate = "duplicate_candidate"
	compactionSkipMemoryBusy         = "memory_busy"
)

type compactionScheduler struct {
	mu       sync.Mutex
	inFlight map[string]struct{}
	stats    compactionSchedulerSnapshot
}

type compactionSchedulerSnapshot struct {
	TotalSkips       int
	DuplicateSkips   int
	MemorySkips      int
	LastSkipReason   string
	InFlightCompacts int
}

func newCompactionScheduler() *compactionScheduler {
	return &compactionScheduler{inFlight: make(map[string]struct{})}
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
	}
}

func (s *compactionScheduler) snapshotCopy() compactionSchedulerSnapshot {
	if s == nil {
		return compactionSchedulerSnapshot{}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.stats
}

func (s *compactionScheduler) snapshot() compactionSchedulerSnapshot {
	return s.snapshotCopy()
}
