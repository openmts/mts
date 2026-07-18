package engine

import (
	"errors"
	"fmt"
	"sync"

	"github.com/openmts/mts/internal/model"
)

type writeIngestStats struct {
	mu              sync.Mutex
	parallelBatches uint64
	parallelShards  uint64
	parallelErrors  uint64
	batchesTotal    uint64
}

func (e *Engine) nextWriteSeq() uint64 {
	return e.writeSeq.Add(1)
}

func (e *Engine) observeWriteSeq(maxSeq uint64) {
	for {
		current := e.writeSeq.Load()
		if maxSeq <= current {
			return
		}
		if e.writeSeq.CompareAndSwap(current, maxSeq) {
			return
		}
	}
}

func (s *writeIngestStats) record(batches int, parallel bool, err error) {
	if s == nil || batches <= 0 {
		return
	}
	s.mu.Lock()
	s.batchesTotal += uint64(batches)
	if parallel {
		s.parallelBatches++
		s.parallelShards += uint64(batches)
		if err != nil {
			s.parallelErrors++
		}
	}
	s.mu.Unlock()
}

func (s *writeIngestStats) snapshot() (batchesTotal, parallelBatches, parallelShards, parallelErrors uint64) {
	if s == nil {
		return 0, 0, 0, 0
	}
	s.mu.Lock()
	batchesTotal = s.batchesTotal
	parallelBatches = s.parallelBatches
	parallelShards = s.parallelShards
	parallelErrors = s.parallelErrors
	s.mu.Unlock()
	return batchesTotal, parallelBatches, parallelShards, parallelErrors
}

func writeShardBatchesTracked(e *Engine, batches []shardBatch, syncWrite bool) error {
	err := writeShardBatches(batches, syncWrite)
	if e != nil {
		e.writeIngest.record(len(batches), len(batches) > 1, err)
	}
	return err
}

func writeTypedShardBatchesTracked(
	e *Engine,
	batch model.ResolvedTypedBatch,
	batches []typedShardBatch,
	syncWrite bool,
) error {
	err := writeTypedShardBatches(batch, batches, syncWrite)
	if e != nil {
		e.writeIngest.record(len(batches), len(batches) > 1, err)
	}
	return err
}

func writeShardBatches(batches []shardBatch, syncWrite bool) error {
	if len(batches) == 0 {
		return nil
	}
	if len(batches) == 1 {
		return batches[0].shard.WriteBatch(batches[0].points, syncWrite)
	}
	errCh := make(chan error, len(batches))
	var wg sync.WaitGroup
	for index := range batches {
		batch := batches[index]
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := batch.shard.WriteBatch(batch.points, syncWrite)
			if err != nil {
				errCh <- fmt.Errorf(
					"parallel write shard %s: %w",
					shardID(batch.shard.opts.Database, batch.shard.opts.RetentionPolicy, batch.shard.opts.Start),
					err,
				)
				return
			}
			errCh <- nil
		}()
	}
	wg.Wait()
	close(errCh)
	var errs []error
	for err := range errCh {
		if err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func writeTypedShardBatches(
	batch model.ResolvedTypedBatch,
	batches []typedShardBatch,
	syncWrite bool,
) error {
	if len(batches) == 0 {
		return nil
	}
	if len(batches) == 1 {
		return batches[0].shard.WriteTypedBatch(batch, batches[0].rows, syncWrite)
	}
	errCh := make(chan error, len(batches))
	var wg sync.WaitGroup
	for index := range batches {
		shardBatch := batches[index]
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := shardBatch.shard.WriteTypedBatch(batch, shardBatch.rows, syncWrite)
			if err != nil {
				errCh <- fmt.Errorf(
					"parallel typed write shard %s: %w",
					shardID(shardBatch.shard.opts.Database, shardBatch.shard.opts.RetentionPolicy, shardBatch.shard.opts.Start),
					err,
				)
				return
			}
			errCh <- nil
		}()
	}
	wg.Wait()
	close(errCh)
	var errs []error
	for err := range errCh {
		if err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
