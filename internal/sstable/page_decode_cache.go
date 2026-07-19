package sstable

import (
	"sync"

	"github.com/openmts/mts/internal/model"
)

// pageDecodeCache 缓存压缩 value page 的查询窗口解码结果，加速热查询重复扫同一页。
// key 绑定 block 位置与查询时间窗，避免跨查询误命中。
type pageDecodeCache struct {
	mu         sync.Mutex
	items      map[pageDecodeKey][]model.VersionedSample
	order      []pageDecodeKey
	limit      int
	maxSamples int
}

type pageDecodeKey struct {
	offset int64
	size   int64
	start  int64
	end    int64
}

type pageDecodeCacheConfig struct {
	limit      int
	maxSamples int
}

const (
	defaultPageDecodeCacheLimit      = 256
	defaultPageDecodeCacheMaxSamples = 512
)

var (
	pageCacheConfigMu sync.RWMutex
	pageCacheConfig   = pageDecodeCacheConfig{
		limit:      defaultPageDecodeCacheLimit,
		maxSamples: defaultPageDecodeCacheMaxSamples,
	}
)

// ConfigurePageDecodeCache 设置全局 page 解码缓存参数。
// limit<=-1 关闭缓存；limit==0 使用默认 256；maxSamples<=0 使用默认 512。
func ConfigurePageDecodeCache(limit int, maxSamples int) {
	pageCacheConfigMu.Lock()
	defer pageCacheConfigMu.Unlock()
	if limit < 0 {
		pageCacheConfig.limit = 0
	} else if limit == 0 {
		pageCacheConfig.limit = defaultPageDecodeCacheLimit
	} else {
		pageCacheConfig.limit = limit
	}
	if maxSamples <= 0 {
		pageCacheConfig.maxSamples = defaultPageDecodeCacheMaxSamples
	} else {
		pageCacheConfig.maxSamples = maxSamples
	}
}

func currentPageDecodeCacheConfig() pageDecodeCacheConfig {
	pageCacheConfigMu.RLock()
	cfg := pageCacheConfig
	pageCacheConfigMu.RUnlock()
	return cfg
}

func newPageCacheIfEnabled(enable bool) *pageDecodeCache {
	if !enable {
		return nil
	}
	return newPageDecodeCache(0)
}

func newPageDecodeCache(limit int) *pageDecodeCache {

	cfg := currentPageDecodeCacheConfig()
	if limit < 0 {
		return nil
	}
	if limit == 0 {
		limit = cfg.limit
	}
	if limit <= 0 {
		return nil
	}
	maxSamples := cfg.maxSamples
	if maxSamples <= 0 {
		maxSamples = defaultPageDecodeCacheMaxSamples
	}
	return &pageDecodeCache{
		items:      make(map[pageDecodeKey][]model.VersionedSample, limit),
		order:      make([]pageDecodeKey, 0, limit),
		limit:      limit,
		maxSamples: maxSamples,
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
	if c == nil || len(samples) == 0 || len(samples) > c.maxSamples {
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
