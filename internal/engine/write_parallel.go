package engine

import (
	"sync"

	"github.com/openmts/mts/internal/model"
)

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
			errCh <- batch.shard.WriteBatch(batch.points, syncWrite)
		}()
	}
	wg.Wait()
	close(errCh)
	var first error
	for err := range errCh {
		if err != nil && first == nil {
			first = err
		}
	}
	return first
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
			errCh <- shardBatch.shard.WriteTypedBatch(batch, shardBatch.rows, syncWrite)
		}()
	}
	wg.Wait()
	close(errCh)
	var first error
	for err := range errCh {
		if err != nil && first == nil {
			first = err
		}
	}
	return first
}
