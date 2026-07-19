package sstable

import (
	"sync"

	"github.com/openmts/mts/internal/model"
)

// pageDecodeCache 缓存压缩 value page 的查询窗口解码结果，加速热查询重复扫同一页。
// key 绑定 block 位置与查询时间窗，避免跨查询误命中。
type pageDecodeCache struct {
	mu    sync.Mutex
	items map[pageDecodeKey][]model.VersionedSample
	order []pageDecodeKey
	limit int
}

type pageDecodeKey struct {
	offset int64
	size   int64
	start  int64
	end    int64
}

const defaultPageDecodeCacheLimit = 256

// pageDecodeCacheMaxSamples 超过该样本数的解码结果不缓存，避免 compact/全表扫灌爆堆。
const pageDecodeCacheMaxSamples = 512

func newPageDecodeCache(limit int) *pageDecodeCache {
	if limit <= 0 {
		limit = defaultPageDecodeCacheLimit
	}
	return &pageDecodeCache{
		items: make(map[pageDecodeKey][]model.VersionedSample, limit),
		order: make([]pageDecodeKey, 0, limit),
		limit: limit,
	}
}

func (c *pageDecodeCache) get(key pageDecodeKey) ([]model.VersionedSample, bool) {
	if c == nil {
		return nil, false
	}
	c.mu.Lock()
	samples, ok := c.items[key]
	c.mu.Unlock()
	if !ok {
		return nil, false
	}
	return cloneVersionedSamples(samples), true
}

func (c *pageDecodeCache) put(key pageDecodeKey, samples []model.VersionedSample) {
	if c == nil || len(samples) == 0 || len(samples) > pageDecodeCacheMaxSamples {
		return
	}
	stored := cloneVersionedSamples(samples)
	c.mu.Lock()
	if _, exists := c.items[key]; exists {
		c.items[key] = stored
		c.mu.Unlock()
		return
	}
	if len(c.order) >= c.limit {
		evict := c.order[0]
		c.order = c.order[1:]
		delete(c.items, evict)
	}
	c.items[key] = stored
	c.order = append(c.order, key)
	c.mu.Unlock()
}

func cloneVersionedSamples(samples []model.VersionedSample) []model.VersionedSample {
	if len(samples) == 0 {
		return nil
	}
	out := make([]model.VersionedSample, len(samples))
	copy(out, samples)
	return out
}
