package engine

import (
	"fmt"

	"github.com/openmts/mts/internal/model"
)

const memApplyRetryLimit = 1

// applyMemAfterWAL 在 WAL 已成功后应用 MemTable。
// WAL 已 durable，不回滚；失败时重试一次，仍失败则记录恢复问题。
func (s *Shard) applyMemAfterWAL(points []model.ResolvedPoint) error {
	err := s.mem.ApplyBatch(points)
	if err == nil {
		return nil
	}
	for range memApplyRetryLimit {
		err = s.mem.ApplyBatch(points)
		if err == nil {
			return nil
		}
	}
	s.recordRecoveryIssue(RecoveryIssue{
		Kind:    RecoveryIssueMemApplyFailed,
		Path:    s.opts.Dir,
		Message: "wal durable but memtable apply failed",
		Err:     err,
	})
	return fmt.Errorf("memtable apply after wal: %w", err)
}

func (s *Shard) applyTypedMemAfterWAL(batch model.ResolvedTypedBatch, rows []int) error {
	err := s.applyTypedMemTable(batch, rows)
	if err == nil {
		return nil
	}
	for range memApplyRetryLimit {
		err = s.applyTypedMemTable(batch, rows)
		if err == nil {
			return nil
		}
	}
	s.recordRecoveryIssue(RecoveryIssue{
		Kind:    RecoveryIssueMemApplyFailed,
		Path:    s.opts.Dir,
		Message: "wal durable but typed memtable apply failed",
		Err:     err,
	})
	return fmt.Errorf("typed memtable apply after wal: %w", err)
}
