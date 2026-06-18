package engine

import (
	"context"
	"errors"
	"time"

	"codeberg.org/mts/mts/internal/model"
	"codeberg.org/mts/mts/internal/queryexec"
)

type queryStatsColumnStream struct {
	source   queryexec.ColumnStream
	stats    *model.QueryStats
	finish   func(*model.QueryStats, error)
	current  model.ColumnSeries
	closed   bool
	finished bool
	counted  bool
	position bool
}

func (e *Engine) beginQueryStats(plan QueryPlan) *model.QueryStats {
	stats := &model.QueryStats{
		CandidateShards:  plan.Explain.CandidateShards,
		ShardsScanned:    len(plan.Shards),
		ShardsSkipped:    plan.Explain.SkippedShards,
		StartedUnixNanos: time.Now().UnixNano(),
	}
	e.storeQueryStats(*stats)
	return stats
}

func (e *Engine) finishQueryStats(stats *model.QueryStats, err error) {
	if stats == nil {
		return
	}
	if stats.StartedUnixNanos > 0 {
		stats.DurationNanos = time.Now().UnixNano() - stats.StartedUnixNanos
	}
	if err != nil {
		stats.Errors++
		if errors.Is(err, queryexec.ErrReadBudgetExceeded) {
			stats.BudgetErrors++
		}
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			stats.Cancellations++
		}
	}
	e.storeQueryStats(*stats)
}

func (e *Engine) storeQueryStats(stats model.QueryStats) {
	e.queryStatsMu.Lock()
	defer e.queryStatsMu.Unlock()
	e.lastQueryStats = stats
}

func (e *Engine) QueryStatsSnapshot() model.QueryStats {
	e.queryStatsMu.Lock()
	defer e.queryStatsMu.Unlock()
	return e.lastQueryStats
}

func newQueryStatsColumnStream(
	source queryexec.ColumnStream,
	stats *model.QueryStats,
	finish func(*model.QueryStats, error),
) queryexec.ColumnStream {
	if stats == nil {
		return source
	}
	return &queryStatsColumnStream{source: source, stats: stats, finish: finish}
}

func (s *queryStatsColumnStream) Next() bool {
	if s.closed || s.finished || s.source == nil {
		return false
	}
	if !s.source.Next() {
		s.finishOnce(s.source.Err())
		return false
	}
	s.current = model.ColumnSeries{}
	s.counted = false
	s.position = true
	return true
}

func (s *queryStatsColumnStream) Column() model.ColumnSeries {
	if !s.position || s.source == nil {
		return model.ColumnSeries{}
	}
	if s.counted {
		return s.current
	}
	s.current = s.source.Column()
	s.stats.SamplesReturned += len(s.current.Values)
	s.counted = true
	return s.current
}

func (s *queryStatsColumnStream) Err() error {
	if s.source == nil {
		return nil
	}
	return s.source.Err()
}

func (s *queryStatsColumnStream) Close() error {
	if s.closed {
		return nil
	}
	s.closed = true
	if s.source == nil {
		s.finishOnce(nil)
		return nil
	}
	err := s.source.Close()
	s.finishOnce(err)
	return err
}

func (s *queryStatsColumnStream) finishOnce(err error) {
	if s.finished {
		return
	}
	s.finished = true
	if s.finish != nil {
		s.finish(s.stats, err)
	}
}
