package engine

// completeCommittedFlush 在 Manifest 已提交后收尾。
// 此时 SSTable part 已是权威数据，禁止把 snapshot 回灌 MemTable。
// WAL checkpoint 失败只记录恢复问题并返回错误，由后续 replay + WriteSeq 去重保证正确性。
func (s *Shard) completeCommittedFlush(snapshot memSnapshot, release func()) error {
	var checkpointErr error
	if s.testHooks.afterManifestBeforeWALTrunc != nil {
		checkpointErr = s.testHooks.afterManifestBeforeWALTrunc()
	}
	if checkpointErr == nil && len(s.tombstones) == 0 {
		checkpointErr = s.wal.Checkpoint()
	}
	snapshot.Release()
	release()
	if checkpointErr == nil {
		return nil
	}
	s.recordRecoveryIssue(RecoveryIssue{
		Kind:    RecoveryIssueWALCheckpointFailed,
		Path:    s.opts.Dir,
		Message: "manifest committed but wal checkpoint failed",
		Err:     checkpointErr,
	})
	return checkpointErr
}
