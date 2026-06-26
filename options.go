package mts

import (
	"fmt"
	"log/slog"
	"strings"
	"time"
)

const (
	defaultOptionsDatabase        = "default"
	defaultOptionsRetentionPolicy = "autogen"
	defaultOptionsShardDuration   = time.Hour
	defaultOptionsMemTableSamples = 10000
	defaultOptionsLevelPartLimit  = 4
	defaultOptionsCascadeSteps    = 8
)

// Options 控制 Engine 打开和运行参数。
//
// 推荐外部用户优先使用 DefaultOptions(path) 获取安全默认值，再按需覆盖字段。
// 零值 Options 仍由 Open 做内部默认归一化，以保持历史兼容。
type Options struct {
	Path                   string
	DefaultDatabase        string
	DefaultRetentionPolicy string
	ShardDuration          time.Duration
	Retention              time.Duration
	MemTableMaxSamples     int
	WAL                    WALOptions
	FlushSync              bool
	Compaction             CompactionOptions
	Compression            CompressionOptions
	StorageMemory          StorageMemoryOptions
	Logger                 *slog.Logger
	User                   UserOptions
	UserManager            UserManager
}

type UserOptions struct {
	Endpoint             string
	PasswordAuthDisabled bool
}

// StorageMemoryOptions 控制存储层内存预算。
type StorageMemoryOptions struct {
	SoftSampleLimit       int
	HardSampleLimit       int
	SoftBytesLimit        int64
	HardBytesLimit        int64
	QueryBytesLimit       int64
	FlushBytesLimit       int64
	CompactionBytesLimit  int64
	CompressionBytesLimit int64
}

// WALOptions 控制 WAL 写入策略。
type WALOptions struct {
	Sync          bool
	SegmentBytes  int64
	BatchRecords  int
	BatchBytes    int64
	BatchInterval time.Duration
	Logger        *slog.Logger
}

// CompactionOptions 控制单机 compaction 策略。
type CompactionOptions struct {
	Enabled                    bool
	Level0PartLimit            int
	Level0SizeLimit            int64
	MaxOutputPartBytes         int64
	Levels                     []CompactionLevelOptions
	MaxCascadeSteps            int
	BackgroundInterval         time.Duration
	ReadAmplificationPartLimit int
	BacklogDegradedThreshold   int
	DiskSpaceReserveBytes      int64
	MinFreeBytes               int64
}

// CompactionLevelOptions 控制一个 compaction level 的阈值。
type CompactionLevelOptions struct {
	Level              int
	PartLimit          int
	SizeLimit          int64
	MaxOutputPartBytes int64
	Compression        CompressionOptions
}

// CompressionOptions 控制 SSTable 数据压缩策略。
type CompressionOptions struct {
	Enabled       bool
	Timestamp     string
	Float         string
	Int           string
	String        string
	Algorithm     string
	MinPageValues int
}

// DefaultOptions 返回适合本地单机嵌入式使用的推荐配置。
func DefaultOptions(path string) Options {
	return Options{
		Path:                   path,
		DefaultDatabase:        defaultOptionsDatabase,
		DefaultRetentionPolicy: defaultOptionsRetentionPolicy,
		ShardDuration:          defaultOptionsShardDuration,
		MemTableMaxSamples:     defaultOptionsMemTableSamples,
		Compaction: CompactionOptions{
			Level0PartLimit: defaultOptionsLevelPartLimit,
			MaxCascadeSteps: defaultOptionsCascadeSteps,
			Levels: []CompactionLevelOptions{
				{Level: 0, PartLimit: defaultOptionsLevelPartLimit},
			},
		},
	}
}

// Validate 检查 Options 是否包含明显非法的显式配置。
func (opts Options) Validate() error {
	if strings.TrimSpace(opts.Path) == "" {
		return invalidOptions("path is empty")
	}
	if err := validateNonNegativeDuration("shard duration", opts.ShardDuration); err != nil {
		return err
	}
	if err := validateNonNegativeDuration("retention", opts.Retention); err != nil {
		return err
	}
	if err := validateNonNegativeInt("memtable max samples", opts.MemTableMaxSamples); err != nil {
		return err
	}
	if err := validateStorageMemoryOptions(opts.StorageMemory); err != nil {
		return err
	}
	if err := validateWALOptions(opts.WAL); err != nil {
		return err
	}
	return validateCompactionOptions(opts.Compaction)
}

func validateStorageMemoryOptions(opts StorageMemoryOptions) error {
	intFields := []struct {
		name  string
		value int
	}{
		{name: "soft sample limit", value: opts.SoftSampleLimit},
		{name: "hard sample limit", value: opts.HardSampleLimit},
	}
	for _, field := range intFields {
		if err := validateNonNegativeInt(field.name, field.value); err != nil {
			return err
		}
	}
	int64Fields := []struct {
		name  string
		value int64
	}{
		{name: "soft bytes limit", value: opts.SoftBytesLimit},
		{name: "hard bytes limit", value: opts.HardBytesLimit},
		{name: "query bytes limit", value: opts.QueryBytesLimit},
		{name: "flush bytes limit", value: opts.FlushBytesLimit},
		{name: "compaction bytes limit", value: opts.CompactionBytesLimit},
		{name: "compression bytes limit", value: opts.CompressionBytesLimit},
	}
	for _, field := range int64Fields {
		if err := validateNonNegativeInt64(field.name, field.value); err != nil {
			return err
		}
	}
	if opts.HardSampleLimit > 0 && opts.SoftSampleLimit > opts.HardSampleLimit {
		return invalidOptions("soft sample limit exceeds hard sample limit")
	}
	if opts.HardBytesLimit > 0 && opts.SoftBytesLimit > opts.HardBytesLimit {
		return invalidOptions("soft bytes limit exceeds hard bytes limit")
	}
	return nil
}

func validateWALOptions(opts WALOptions) error {
	if err := validateNonNegativeInt64("wal segment bytes", opts.SegmentBytes); err != nil {
		return err
	}
	if err := validateNonNegativeInt("wal batch records", opts.BatchRecords); err != nil {
		return err
	}
	if err := validateNonNegativeInt64("wal batch bytes", opts.BatchBytes); err != nil {
		return err
	}
	return validateNonNegativeDuration("wal batch interval", opts.BatchInterval)
}

func validateCompactionOptions(opts CompactionOptions) error {
	fields := []struct {
		name  string
		value int
	}{
		{name: "level0 part limit", value: opts.Level0PartLimit},
		{name: "max cascade steps", value: opts.MaxCascadeSteps},
		{name: "read amplification part limit", value: opts.ReadAmplificationPartLimit},
		{name: "backlog degraded threshold", value: opts.BacklogDegradedThreshold},
	}
	for _, field := range fields {
		if err := validateNonNegativeInt(field.name, field.value); err != nil {
			return err
		}
	}
	if err := validateCompactionByteOptions(opts); err != nil {
		return err
	}
	if err := validateNonNegativeDuration("background interval", opts.BackgroundInterval); err != nil {
		return err
	}
	for _, level := range opts.Levels {
		if err := validateCompactionLevelOptions(level); err != nil {
			return err
		}
	}
	return nil
}

func validateCompactionByteOptions(opts CompactionOptions) error {
	fields := []struct {
		name  string
		value int64
	}{
		{name: "level0 size limit", value: opts.Level0SizeLimit},
		{name: "max output part bytes", value: opts.MaxOutputPartBytes},
		{name: "disk space reserve bytes", value: opts.DiskSpaceReserveBytes},
		{name: "min free bytes", value: opts.MinFreeBytes},
	}
	for _, field := range fields {
		if err := validateNonNegativeInt64(field.name, field.value); err != nil {
			return err
		}
	}
	return nil
}

func validateCompactionLevelOptions(opts CompactionLevelOptions) error {
	if opts.Level < 0 {
		return invalidOptions("compaction level must not be negative")
	}
	if err := validateNonNegativeInt("compaction level part limit", opts.PartLimit); err != nil {
		return err
	}
	if err := validateNonNegativeInt64("compaction level size limit", opts.SizeLimit); err != nil {
		return err
	}
	if err := validateNonNegativeInt64("compaction level max output part bytes", opts.MaxOutputPartBytes); err != nil {
		return err
	}
	return validateCompressionOptions(opts.Compression)
}

func validateCompressionOptions(opts CompressionOptions) error {
	return validateNonNegativeInt("compression min page values", opts.MinPageValues)
}

func validateNonNegativeDuration(name string, value time.Duration) error {
	if value < 0 {
		return invalidOptions(name + " must not be negative")
	}
	return nil
}

func validateNonNegativeInt(name string, value int) error {
	if value < 0 {
		return invalidOptions(name + " must not be negative")
	}
	return nil
}

func validateNonNegativeInt64(name string, value int64) error {
	if value < 0 {
		return invalidOptions(name + " must not be negative")
	}
	return nil
}

func invalidOptions(message string) error {
	return fmt.Errorf("%w: %s", ErrInvalidOptions, message)
}
