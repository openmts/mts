package sstable

import (
	"sync"
)

// blockPayloadCache 缓存查询侧已读出的 block payload（去掉帧头 CRC 后的原始字节）。
// 命中后跳过 ReadAt + CRC，降低冷/热重复扫盘与校验成本。
// 仅用于 OpenPart（查询路径）；OpenPartTrusted（compact）不挂缓存。
type blockPayloadCache struct {
	mu         sync.Mutex
	items      map[blockPayloadKey][]byte
	order      []blockPayloadKey
	limit      int
	maxBytes   int64
	totalBytes int64
}

type blockPayloadKey struct {
	name   string
	offset int64
	size   int64
}

type blockPayloadCacheConfig struct {
	limit    int
	maxBytes int64
}

const (
	defaultBlockPayloadCacheLimit    = 512
	defaultBlockPayloadCacheMaxBytes = 64 << 20 // 64MiB
	maxCachedBlockPayloadBytes       = 1 << 20  // 单块 >1MiB 不缓存
)

var (
	blockCacheConfigMu sync.RWMutex
	blockCacheConfig   = blockPayloadCacheConfig{
		limit:    defaultBlockPayloadCacheLimit,
		maxBytes: defaultBlockPayloadCacheMaxBytes,
	}
)

// ConfigureBlockPayloadCache 设置全局 block payload 缓存。
// limit<=-1 关闭；limit==0 默认 512。
// maxBytes<=0 默认 64MiB。
func ConfigureBlockPayloadCache(limit int, maxBytes int64) {
	blockCacheConfigMu.Lock()
	defer blockCacheConfigMu.Unlock()
	if limit < 0 {
		blockCacheConfig.limit = 0
	} else if limit == 0 {
		blockCacheConfig.limit = defaultBlockPayloadCacheLimit
	} else {
		blockCacheConfig.limit = limit
	}
	if maxBytes <= 0 {
		blockCacheConfig.maxBytes = defaultBlockPayloadCacheMaxBytes
	} else {
		blockCacheConfig.maxBytes = maxBytes
	}
}

func currentBlockPayloadCacheConfig() blockPayloadCacheConfig {
	blockCacheConfigMu.RLock()
	cfg := blockCacheConfig
	blockCacheConfigMu.RUnlock()
	return cfg
}

func newBlockPayloadCacheIfEnabled(enable bool) *blockPayloadCache {
	if !enable {
		return nil
	}
	cfg := currentBlockPayloadCacheConfig()
	if cfg.limit <= 0 {
		return nil
	}
	return &blockPayloadCache{
		items:    make(map[blockPayloadKey][]byte, cfg.limit),
		order:    make([]blockPayloadKey, 0, cfg.limit),
		limit:    cfg.limit,
		maxBytes: cfg.maxBytes,
	}
}

func (c *blockPayloadCache) get(key blockPayloadKey) ([]byte, bool) {
	if c == nil {
		return nil, false
	}
	c.mu.Lock()
	payload, ok := c.items[key]
	c.mu.Unlock()
	if !ok {
		return nil, false
	}
	out := make([]byte, len(payload))
	copy(out, payload)
	return out, true
}

func (c *blockPayloadCache) put(key blockPayloadKey, payload []byte) {
	if c == nil || len(payload) == 0 || int64(len(payload)) > maxCachedBlockPayloadBytes {
		return
	}
	size := int64(len(payload))
	stored := make([]byte, len(payload))
	copy(stored, payload)
	c.mu.Lock()
	if _, exists := c.items[key]; exists {
		old := int64(len(c.items[key]))
		c.items[key] = stored
		c.totalBytes += size - old
		c.mu.Unlock()
		return
	}
	for (len(c.order) >= c.limit || (c.maxBytes > 0 && c.totalBytes+size > c.maxBytes)) && len(c.order) > 0 {
		evict := c.order[0]
		c.order = c.order[1:]
		if old, ok := c.items[evict]; ok {
			c.totalBytes -= int64(len(old))
			delete(c.items, evict)
		}
	}
	c.items[key] = stored
	c.order = append(c.order, key)
	c.totalBytes += size
	c.mu.Unlock()
}

func (c *blockPayloadCache) clear() {
	if c == nil {
		return
	}
	c.mu.Lock()
	c.items = make(map[blockPayloadKey][]byte, c.limit)
	c.order = c.order[:0]
	c.totalBytes = 0
	c.mu.Unlock()
}
