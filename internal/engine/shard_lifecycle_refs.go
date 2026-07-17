package engine

import "errors"

// ErrShardBusy 表示 shard 仍有进行中的读引用，无法关闭或执行破坏性维护。
var ErrShardBusy = errors.New("shard busy")

func (s *Shard) acquireReadLocked() {
	s.readRefs++
}

func (s *Shard) releaseRead() {
	s.lifecycleMu.Lock()
	if s.readRefs > 0 {
		s.readRefs--
	}
	s.lifecycleMu.Unlock()
}

func (s *Shard) hasActiveReadersLocked() bool {
	return s.readRefs > 0
}
