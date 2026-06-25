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
	ConfigPath    string              `yaml:"-"`
	DataDir       string              `yaml:"data_dir"`
	HTTP          httpConfig          `yaml:"http"`
	GRPC          grpcConfig          `yaml:"grpc"`
	Auth          authConfig          `yaml:"auth"`
	Limits        limitsConfig        `yaml:"limits"`
	Observability observabilityConfig `yaml:"observability"`
	Backup        backupConfig        `yaml:"backup"`
	Log           logConfig           `yaml:"log"`
	Engine        engineConfig        `yaml:"engine"`
	Shutdown      durationText        `yaml:"shutdown_timeout"`
}

type httpConfig struct {
	Enabled           bool         `yaml:"enabled"`
	Addr              string       `yaml:"addr"`
	TLS               tlsConfig    `yaml:"tls"`
	ReadTimeout       durationText `yaml:"read_timeout"`
	ReadHeaderTimeout durationText `yaml:"read_header_timeout"`
	WriteTimeout      durationText `yaml:"write_timeout"`
	IdleTimeout       durationText `yaml:"idle_timeout"`
}

type grpcConfig struct {
	Enabled         bool      `yaml:"enabled"`
	Addr            string    `yaml:"addr"`
	TLS             tlsConfig `yaml:"tls"`
	MaxRecvMsgBytes int       `yaml:"max_recv_msg_bytes"`
	MaxSendMsgBytes int       `yaml:"max_send_msg_bytes"`
}

type authConfig struct {
	AdminToken  string   `yaml:"admin_token"`
	DataTokens  []string `yaml:"data_tokens"`
	RequireUser bool     `yaml:"require_user"`
}

type tlsConfig struct {
	Enabled      bool   `yaml:"enabled"`
	CertFile     string `yaml:"cert_file"`
	KeyFile      string `yaml:"key_file"`
	ClientCAFile string `yaml:"client_ca_file"`
	ClientAuth   bool   `yaml:"client_auth"`
}

type limitsConfig struct {
	MaxRequestBodyBytes int64        `yaml:"max_request_body_bytes"`
	MaxWritePoints      int          `yaml:"max_write_points"`
	DefaultQueryLimit   int          `yaml:"default_query_limit"`
	MaxQueryLimit       int          `yaml:"max_query_limit"`
	RequestTimeout      durationText `yaml:"request_timeout"`
	MaxConcurrentHTTP   int          `yaml:"max_concurrent_http"`
	MaxConcurrentGRPC   int          `yaml:"max_concurrent_grpc"`
}

type observabilityConfig struct {
	AccessLog bool        `yaml:"access_log"`
	Pprof     pprofConfig `yaml:"pprof"`
}

type pprofConfig struct {
	Enabled bool `yaml:"enabled"`
}

type backupConfig struct {
	Dir string `yaml:"dir"`
}

type logConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
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
			Enabled:           true,
			Addr:              "127.0.0.1:8086",
			ReadHeaderTimeout: durationText(5 * time.Second),
			ReadTimeout:       durationText(30 * time.Second),
			WriteTimeout:      durationText(30 * time.Second),
			IdleTimeout:       durationText(2 * time.Minute),
		},
		GRPC: grpcConfig{
			Enabled:         true,
			Addr:            "127.0.0.1:9096",
			MaxRecvMsgBytes: 16 << 20,
			MaxSendMsgBytes: 16 << 20,
		},
		Limits: limitsConfig{
			MaxRequestBodyBytes: 16 << 20,
			MaxWritePoints:      10000,
			DefaultQueryLimit:   10000,
			MaxQueryLimit:       100000,
			RequestTimeout:      durationText(30 * time.Second),
			MaxConcurrentHTTP:   1024,
			MaxConcurrentGRPC:   1024,
		},
		Observability: observabilityConfig{
			AccessLog: true,
		},
		Backup: backupConfig{
			Dir: "./data/mts-server/backups",
		},
		Log: logConfig{
			Level:  "info",
			Format: "text",
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
	cfg.ConfigPath = path
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
	if err := validateTLSConfig("http", cfg.HTTP.TLS); err != nil {
		return err
	}
	if err := validateTLSConfig("grpc", cfg.GRPC.TLS); err != nil {
		return err
	}
	if cfg.Limits.MaxRequestBodyBytes < 0 || cfg.Limits.MaxWritePoints < 0 ||
		cfg.Limits.DefaultQueryLimit < 0 || cfg.Limits.MaxQueryLimit < 0 ||
		cfg.Limits.MaxConcurrentHTTP < 0 || cfg.Limits.MaxConcurrentGRPC < 0 {
		return fmt.Errorf("%w: limits must not be negative", errInvalidConfig)
	}
	if cfg.Limits.MaxQueryLimit > 0 && cfg.Limits.DefaultQueryLimit > cfg.Limits.MaxQueryLimit {
		return fmt.Errorf("%w: default_query_limit exceeds max_query_limit", errInvalidConfig)
	}
	if time.Duration(cfg.Limits.RequestTimeout) < 0 {
		return fmt.Errorf("%w: request_timeout must not be negative", errInvalidConfig)
	}
	if cfg.GRPC.MaxRecvMsgBytes < 0 || cfg.GRPC.MaxSendMsgBytes < 0 {
		return fmt.Errorf("%w: grpc message sizes must not be negative", errInvalidConfig)
	}
	if strings.TrimSpace(cfg.Log.Level) == "" {
		return fmt.Errorf("%w: log level is empty", errInvalidConfig)
	}
	if cfg.Log.Format != "" && cfg.Log.Format != "text" && cfg.Log.Format != "json" {
		return fmt.Errorf("%w: log format must be text or json", errInvalidConfig)
	}
	return cfg.engineOptions().Validate()
}

func validateTLSConfig(name string, cfg tlsConfig) error {
	if !cfg.Enabled {
		return nil
	}
	if strings.TrimSpace(cfg.CertFile) == "" || strings.TrimSpace(cfg.KeyFile) == "" {
		return fmt.Errorf("%w: %s tls cert_file and key_file are required", errInvalidConfig, name)
	}
	if cfg.ClientAuth && strings.TrimSpace(cfg.ClientCAFile) == "" {
		return fmt.Errorf("%w: %s tls client_ca_file is required when client_auth is true", errInvalidConfig, name)
	}
	return nil
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
