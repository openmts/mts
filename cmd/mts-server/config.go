package main

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/goccy/go-yaml"

	mts "github.com/openmts/mts"
)

var errInvalidConfig = errors.New("invalid config")

type config struct {
	DataDir  string       `yaml:"data_dir"`
	HTTP     httpConfig   `yaml:"http"`
	GRPC     grpcConfig   `yaml:"grpc"`
	Engine   engineConfig `yaml:"engine"`
	Shutdown durationText `yaml:"shutdown_timeout"`
}

type httpConfig struct {
	Enabled bool   `yaml:"enabled"`
	Addr    string `yaml:"addr"`
}

type grpcConfig struct {
	Enabled bool   `yaml:"enabled"`
	Addr    string `yaml:"addr"`
}

type engineConfig struct {
	DefaultDatabase        string       `yaml:"default_database"`
	DefaultRetentionPolicy string       `yaml:"default_retention_policy"`
	ShardDuration          durationText `yaml:"shard_duration"`
	Retention              durationText `yaml:"retention"`
	MemTableMaxSamples     int          `yaml:"memtable_max_samples"`
	FlushSync              bool         `yaml:"flush_sync"`
	Compression            struct {
		Enabled       bool   `yaml:"enabled"`
		Algorithm     string `yaml:"algorithm"`
		MinPageValues int    `yaml:"min_page_values"`
	} `yaml:"compression"`
	Compaction struct {
		Enabled            bool         `yaml:"enabled"`
		BackgroundInterval durationText `yaml:"background_interval"`
		Level0PartLimit    int          `yaml:"level0_part_limit"`
		MaxCascadeSteps    int          `yaml:"max_cascade_steps"`
	} `yaml:"compaction"`
}

type durationText time.Duration

func defaultConfig() config {
	return config{
		DataDir: "./data/mts",
		HTTP: httpConfig{
			Enabled: true,
			Addr:    "127.0.0.1:8086",
		},
		GRPC: grpcConfig{
			Enabled: true,
			Addr:    "127.0.0.1:9096",
		},
		Engine: engineConfig{
			DefaultDatabase:        "default",
			DefaultRetentionPolicy: "autogen",
			ShardDuration:          durationText(time.Hour),
			MemTableMaxSamples:     10000,
			Compaction: struct {
				Enabled            bool         `yaml:"enabled"`
				BackgroundInterval durationText `yaml:"background_interval"`
				Level0PartLimit    int          `yaml:"level0_part_limit"`
				MaxCascadeSteps    int          `yaml:"max_cascade_steps"`
			}{
				Enabled:         true,
				Level0PartLimit: 4,
				MaxCascadeSteps: 8,
			},
		},
		Shutdown: durationText(15 * time.Second),
	}
}

func loadConfig(path string) (config, error) {
	if strings.TrimSpace(path) == "" {
		return config{}, fmt.Errorf("%w: config path is empty", errInvalidConfig)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return config{}, err
	}
	cfg := defaultConfig()
	if err := yaml.UnmarshalWithOptions(data, &cfg, yaml.DisallowUnknownField()); err != nil {
		return config{}, fmt.Errorf("decode config: %w", err)
	}
	if err := cfg.validate(); err != nil {
		return config{}, err
	}
	return cfg, nil
}

func (cfg config) validate() error {
	if strings.TrimSpace(cfg.DataDir) == "" {
		return fmt.Errorf("%w: data_dir is empty", errInvalidConfig)
	}
	if !cfg.HTTP.Enabled && !cfg.GRPC.Enabled {
		return fmt.Errorf("%w: at least one listener must be enabled", errInvalidConfig)
	}
	if cfg.HTTP.Enabled && strings.TrimSpace(cfg.HTTP.Addr) == "" {
		return fmt.Errorf("%w: http addr is empty", errInvalidConfig)
	}
	if cfg.GRPC.Enabled && strings.TrimSpace(cfg.GRPC.Addr) == "" {
		return fmt.Errorf("%w: grpc addr is empty", errInvalidConfig)
	}
	if time.Duration(cfg.Shutdown) <= 0 {
		return fmt.Errorf("%w: shutdown_timeout must be positive", errInvalidConfig)
	}
	return cfg.engineOptions().Validate()
}

func (cfg config) engineOptions() mts.Options {
	opts := mts.DefaultOptions(cfg.DataDir)
	opts.DefaultDatabase = cfg.Engine.DefaultDatabase
	opts.DefaultRetentionPolicy = cfg.Engine.DefaultRetentionPolicy
	opts.ShardDuration = time.Duration(cfg.Engine.ShardDuration)
	opts.Retention = time.Duration(cfg.Engine.Retention)
	opts.MemTableMaxSamples = cfg.Engine.MemTableMaxSamples
	opts.FlushSync = cfg.Engine.FlushSync
	opts.Compression.Enabled = cfg.Engine.Compression.Enabled
	opts.Compression.Algorithm = cfg.Engine.Compression.Algorithm
	opts.Compression.MinPageValues = cfg.Engine.Compression.MinPageValues
	opts.Compaction.Enabled = cfg.Engine.Compaction.Enabled
	opts.Compaction.BackgroundInterval = time.Duration(cfg.Engine.Compaction.BackgroundInterval)
	opts.Compaction.Level0PartLimit = cfg.Engine.Compaction.Level0PartLimit
	opts.Compaction.MaxCascadeSteps = cfg.Engine.Compaction.MaxCascadeSteps
	return opts
}

func (d durationText) MarshalYAML() ([]byte, error) {
	return []byte(time.Duration(d).String()), nil
}

func (d *durationText) UnmarshalYAML(data []byte) error {
	var value string
	if err := yaml.Unmarshal(data, &value); err != nil {
		return err
	}
	duration, err := time.ParseDuration(value)
	if err != nil {
		return err
	}
	*d = durationText(duration)
	return nil
}
