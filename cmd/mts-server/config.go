package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	yaml "github.com/goccy/go-yaml"

	mts "github.com/openmts/mts"
)

var errInvalidConfig = errors.New("invalid config")

type config struct {
	ConfigPath    string              `yaml:"-"             json:"config_path"`
	DataDir       string              `yaml:"data_dir"      json:"data_dir"`
	HTTP          httpConfig          `yaml:"http"          json:"http"`
	GRPC          grpcConfig          `yaml:"grpc"          json:"grpc"`
	Auth          authConfig          `yaml:"auth"          json:"auth"`
	User          userConfig          `yaml:"user"          json:"user"`
	Limits        limitsConfig        `yaml:"limits"        json:"limits"`
	Observability observabilityConfig `yaml:"observability" json:"observability"`
	Backup        backupConfig        `yaml:"backup"        json:"backup"`
	Log           logConfig           `yaml:"log"           json:"log"`
	Engine        engineConfig        `yaml:"engine"        json:"engine"`
	Shutdown      durationText        `yaml:"shutdown_timeout" json:"shutdown_timeout"`
}

type httpConfig struct {
	Enabled           bool         `yaml:"enabled"              json:"enabled"`
	Addr              string       `yaml:"addr"                 json:"addr"`
	TLS               tlsConfig    `yaml:"tls"                  json:"tls"`
	ReadTimeout       durationText `yaml:"read_timeout"         json:"read_timeout"`
	ReadHeaderTimeout durationText `yaml:"read_header_timeout"  json:"read_header_timeout"`
	WriteTimeout      durationText `yaml:"write_timeout"        json:"write_timeout"`
	IdleTimeout       durationText `yaml:"idle_timeout"         json:"idle_timeout"`
}

type grpcConfig struct {
	Enabled         bool      `yaml:"enabled"             json:"enabled"`
	Addr            string    `yaml:"addr"                json:"addr"`
	TLS             tlsConfig `yaml:"tls"                 json:"tls"`
	MaxRecvMsgBytes int       `yaml:"max_recv_msg_bytes"  json:"max_recv_msg_bytes"`
	MaxSendMsgBytes int       `yaml:"max_send_msg_bytes"  json:"max_send_msg_bytes"`
}

type authConfig struct {
	AdminToken  string   `yaml:"admin_token"   json:"admin_token"`
	DataTokens  []string `yaml:"data_tokens"   json:"data_tokens"`
	RequireUser bool     `yaml:"require_user"  json:"require_user"`
}

type userConfig struct {
	Endpoint             string `yaml:"endpoint"                  json:"endpoint"`
	PasswordAuthDisabled bool   `yaml:"password_auth_disabled"    json:"password_auth_disabled"`
}

type tlsConfig struct {
	Enabled      bool   `yaml:"enabled"          json:"enabled"`
	CertFile     string `yaml:"cert_file"        json:"cert_file"`
	KeyFile      string `yaml:"key_file"         json:"key_file"`
	ClientCAFile string `yaml:"client_ca_file"   json:"client_ca_file"`
	ClientAuth   bool   `yaml:"client_auth"      json:"client_auth"`
}

type limitsConfig struct {
	MaxRequestBodyBytes int64        `yaml:"max_request_body_bytes" json:"max_request_body_bytes"`
	MaxWritePoints      int          `yaml:"max_write_points"       json:"max_write_points"`
	DefaultQueryLimit   int          `yaml:"default_query_limit"    json:"default_query_limit"`
	MaxQueryLimit       int          `yaml:"max_query_limit"        json:"max_query_limit"`
	RequestTimeout      durationText `yaml:"request_timeout"        json:"request_timeout"`
	MaxConcurrentHTTP   int          `yaml:"max_concurrent_http"    json:"max_concurrent_http"`
	MaxConcurrentGRPC   int          `yaml:"max_concurrent_grpc"    json:"max_concurrent_grpc"`
}

type observabilityConfig struct {
	AccessLog bool        `yaml:"access_log" json:"access_log"`
	Pprof     pprofConfig `yaml:"pprof"      json:"pprof"`
}

type pprofConfig struct {
	Enabled bool `yaml:"enabled" json:"enabled"`
}

type backupConfig struct {
	Dir string `yaml:"dir" json:"dir"`
}

type logConfig struct {
	Level  string `yaml:"level"  json:"level"`
	Format string `yaml:"format" json:"format"`
}

type engineConfig struct {
	DefaultDatabase        string       `yaml:"default_database"          json:"default_database"`
	DefaultRetentionPolicy string       `yaml:"default_retention_policy"  json:"default_retention_policy"`
	ShardDuration          durationText `yaml:"shard_duration"            json:"shard_duration"`
	Retention              durationText `yaml:"retention"                 json:"retention"`
	MemTableMaxSamples     int          `yaml:"memtable_max_samples"      json:"memtable_max_samples"`
	FlushSync              bool         `yaml:"flush_sync"                json:"flush_sync"`
	Compression            struct {
		Enabled       bool   `yaml:"enabled"          json:"enabled"`
		Algorithm     string `yaml:"algorithm"        json:"algorithm"`
		MinPageValues int    `yaml:"min_page_values"  json:"min_page_values"`
	} `yaml:"compression" json:"compression"`
	Compaction struct {
		Enabled            bool         `yaml:"enabled"             json:"enabled"`
		BackgroundInterval durationText `yaml:"background_interval" json:"background_interval"`
		Level0PartLimit    int          `yaml:"level0_part_limit"   json:"level0_part_limit"`
		MaxCascadeSteps    int          `yaml:"max_cascade_steps"   json:"max_cascade_steps"`
	} `yaml:"compaction" json:"compaction"`
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
		User: userConfig{
			Endpoint: "local",
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
				Enabled            bool         `yaml:"enabled"             json:"enabled"`
				BackgroundInterval durationText `yaml:"background_interval" json:"background_interval"`
				Level0PartLimit    int          `yaml:"level0_part_limit"   json:"level0_part_limit"`
				MaxCascadeSteps    int          `yaml:"max_cascade_steps"   json:"max_cascade_steps"`
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
	cfg.User.Endpoint = strings.TrimSpace(cfg.User.Endpoint)
	if cfg.User.Endpoint == "" {
		cfg.User.Endpoint = "local"
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
	opts.User.Endpoint = cfg.User.Endpoint
	opts.User.PasswordAuthDisabled = cfg.User.PasswordAuthDisabled
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

func (d durationText) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Duration(d).String())
}

func (d *durationText) UnmarshalJSON(data []byte) error {
	var value string
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	duration, err := time.ParseDuration(value)
	if err != nil {
		return err
	}
	*d = durationText(duration)
	return nil
}
