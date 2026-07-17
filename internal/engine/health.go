package engine

import (
	"fmt"
)

type HealthSnapshot struct {
	Healthy bool
	Ready   bool
	Reasons []string
	Checks  []HealthCheck
}

type HealthCheck struct {
	Name   string
	Status string
	Reason string
}

func (e *Engine) HealthSnapshot() HealthSnapshot {
	e.mu.Lock()
	defer e.mu.Unlock()
	health := HealthSnapshot{Healthy: true, Ready: true, Reasons: []string{}}
	health.Checks = append(health.Checks,
		HealthCheck{Name: "wal", Status: "ok"},
		HealthCheck{Name: "manifest", Status: "ok"},
		HealthCheck{Name: "disk", Status: "ok"},
		HealthCheck{Name: "compaction", Status: "ok"},
		HealthCheck{Name: "memory", Status: "ok"},
		HealthCheck{Name: "maintenance", Status: "ok"},
		HealthCheck{Name: "downsample", Status: "ok"},
	)
	for id, shard := range e.shards {
		if shard.wal == nil {
			markHealthCheck(&health, "wal", "failed", "wal unavailable on shard "+id, true)
		}
		if len(shard.manifest.Parts) != len(shard.parts) {
			markHealthCheck(&health, "manifest", "failed", "manifest part reader mismatch on shard "+id, true)
		}
		if shard.deps.files != nil && shard.opts.Dir != "" {
			if _, err := shard.deps.files.AvailableBytes(shard.opts.Dir); err != nil {
				markHealthCheck(&health, "disk", "failed", "disk check failed on shard "+id+": "+err.Error(), true)
			}
		}
		backlog, err := shard.compactionBacklogSnapshot(e.opts.Compaction)
		if err != nil {
			markHealthCheck(&health, "compaction", "failed", "compaction backlog check failed: "+err.Error(), true)
			continue
		}
		if backlog.Degraded {
			markHealthCheck(&health, "compaction", "degraded", "compaction degraded on shard "+id, true)
		}
		if shard.maintenanceErr != nil {
			markHealthCheck(&health, "maintenance", "failed", "maintenance error on shard "+id+": "+shard.maintenanceErr.Error(), true)
		}
	}
	e.recordMemoryHealthLocked(&health)
	e.recordDownsampleHealth(&health)
	return health
}

func (e *Engine) recordDownsampleHealth(health *HealthSnapshot) {
	stats := e.DownsampleStatsSnapshot()
	if stats.LastError != "" {
		reason := "last downsample error: " + stats.LastError
		if stats.LastPolicy != "" {
			reason = "last downsample error on policy " + stats.LastPolicy + ": " + stats.LastError
		}
		markHealthCheck(health, "downsample", "degraded", reason, true)
	}
}

func (e *Engine) recordMemoryHealthLocked(health *HealthSnapshot) {
	active := e.safeStorageMemoryActiveLocked()
	current := active.total()
	if e.memory != nil {
		e.memory.mu.Lock()
		current += e.memory.totalReserved
		e.memory.mu.Unlock()
	}
	if limit := e.opts.StorageMemory.HardBytesLimit; limit > 0 && current > limit {
		markHealthCheck(health, "memory", "failed", fmt.Sprintf("storage memory hard limit exceeded: current=%d limit=%d", current, limit), true)
		return
	}
	if limit := e.opts.StorageMemory.SoftBytesLimit; limit > 0 && current > limit {
		markHealthCheck(health, "memory", "degraded", fmt.Sprintf("storage memory soft limit exceeded: current=%d limit=%d", current, limit), false)
	}
}

func (e *Engine) safeStorageMemoryActiveLocked() storageMemoryActive {
	var active storageMemoryActive
	for _, shard := range e.shards {
		if shard == nil {
			continue
		}
		if shard.mem != nil {
			active.MemTableBytes += shard.ApproxMemTableMemoryBytes()
		}
		if shard.wal != nil {
			active.WALBytes += shard.ApproxWALMemoryBytes()
		}
	}
	return active
}

func markHealthCheck(health *HealthSnapshot, name string, status string, reason string, notReady bool) {
	if health == nil {
		return
	}
	for index := range health.Checks {
		if health.Checks[index].Name != name {
			continue
		}
		health.Checks[index].Status = status
		health.Checks[index].Reason = reason
		break
	}
	if notReady {
		health.Healthy = false
		health.Ready = false
	}
	if reason != "" {
		health.Reasons = append(health.Reasons, reason)
	}
}
