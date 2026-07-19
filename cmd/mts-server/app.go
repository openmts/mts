package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	yaml "github.com/goccy/go-yaml"
	cli "github.com/urfave/cli/v2"
)

func newApp(stdout io.Writer, stderr io.Writer) *cli.App {
	app := &cli.App{
		Name:  "mts-server",
		Usage: "run a standalone MTS server with HTTP and gRPC APIs",
		Commands: []*cli.Command{
			serveCommand(stdout, stderr),
			validateConfigCommand(stdout),
			doctorCommand(stdout),
			initConfigCommand(stdout),
			versionCommand(stdout),
		},
		DefaultCommand: "serve",
		Writer:         stdout,
		ErrWriter:      stderr,
	}
	return app
}

func serveCommand(stdout io.Writer, stderr io.Writer) *cli.Command {
	return &cli.Command{
		Name:  "serve",
		Usage: "start mts-server from a YAML config file",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "config", Aliases: []string{"c"}, Usage: "path to mts-server YAML config"},
			&cli.BoolFlag{Name: "print-config", Usage: "print the default config YAML and exit"},
		},
		Action: func(cliCtx *cli.Context) error {
			if cliCtx.Bool("print-config") {
				return printDefaultConfig(stdout)
			}
			path := cliCtx.String("config")
			cfg, err := loadConfig(path)
			if err != nil {
				return err
			}
			logger := newLogger(stderr, cfg.Log)
			return runServer(cliCtx.Context, cfg, logger)
		},
	}
}

func validateConfigCommand(stdout io.Writer) *cli.Command {
	return &cli.Command{
		Name:  "validate-config",
		Usage: "validate an mts-server YAML config file",
		Flags: []cli.Flag{&cli.StringFlag{Name: "config", Aliases: []string{"c"}, Required: true}},
		Action: func(cliCtx *cli.Context) error {
			if _, err := loadConfig(cliCtx.String("config")); err != nil {
				return err
			}
			_, err := fmt.Fprintln(stdout, "config ok")
			return err
		},
	}
}

func doctorCommand(stdout io.Writer) *cli.Command {
	return &cli.Command{
		Name:  "doctor",
		Usage: "check config and local runtime prerequisites",
		Flags: []cli.Flag{&cli.StringFlag{Name: "config", Aliases: []string{"c"}, Required: true}},
		Action: func(cliCtx *cli.Context) error {
			cfg, err := loadConfig(cliCtx.String("config"))
			if err != nil {
				return err
			}
			report, err := runDoctorChecks(cfg)
			if err != nil {
				return err
			}
			for _, line := range report {
				if _, err := fmt.Fprintln(stdout, line); err != nil {
					return err
				}
			}
			_, err = fmt.Fprintln(stdout, "doctor ok")
			return err
		},
	}
}

// runDoctorChecks 输出可商用部署前检查项（目录权限、TLS 文件、边缘 HTTPS 提示等）。
func runDoctorChecks(cfg config) ([]string, error) {
	lines := make([]string, 0, 16)
	if err := os.MkdirAll(cfg.DataDir, 0700); err != nil {
		return nil, err
	}
	lines = append(lines, "ok: data_dir ready: "+cfg.DataDir)
	backupDir := cfg.Backup.Dir
	if backupDir == "" {
		backupDir = filepath.Join(cfg.DataDir, "backups")
	}
	if err := os.MkdirAll(backupDir, 0700); err != nil {
		return nil, err
	}
	lines = append(lines, "ok: backup_dir ready: "+backupDir)

	if cfg.HTTP.TLS.Enabled {
		if _, err := buildTLSConfig(cfg.HTTP.TLS); err != nil {
			return nil, fmt.Errorf("http tls: %w", err)
		}
		lines = append(lines, "ok: http tls enabled (HSTS will be emitted by server)")
	} else {
		lines = append(lines, "warn: http tls disabled; terminate HTTPS/HSTS at reverse proxy edge")
	}
	if cfg.GRPC.TLS.Enabled {
		if _, err := buildTLSConfig(cfg.GRPC.TLS); err != nil {
			return nil, fmt.Errorf("grpc tls: %w", err)
		}
		lines = append(lines, "ok: grpc tls enabled")
	} else if cfg.GRPC.Enabled {
		lines = append(lines, "warn: grpc tls disabled; keep gRPC on private network or enable tls")
	}
	if !cfg.User.PasswordAuthDisabled {
		lines = append(lines, "ok: password auth enabled (bootstrap admin requires password change)")
	} else {
		lines = append(lines, "warn: password auth disabled")
	}
	if strings.TrimSpace(cfg.Auth.AdminToken) == "" && !cfg.Auth.RequireUser {
		lines = append(lines, "warn: admin_token empty and require_user=false; tighten auth for production")
	}
	return lines, nil
}

func initConfigCommand(stdout io.Writer) *cli.Command {
	return &cli.Command{
		Name:  "init-config",
		Usage: "write a default mts-server YAML config file",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "output", Aliases: []string{"o"}, Required: true},
			&cli.BoolFlag{Name: "force"},
		},
		Action: func(cliCtx *cli.Context) error {
			path := cliCtx.String("output")
			if !cliCtx.Bool("force") {
				if _, err := os.Stat(path); err == nil {
					return fmt.Errorf("%w: output config already exists", errInvalidConfig)
				} else if !errors.Is(err, os.ErrNotExist) {
					return err
				}
			}
			if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
				return err
			}
			data, err := yaml.MarshalWithOptions(defaultConfig(), yaml.Indent(2))
			if err != nil {
				return err
			}
			if err := os.WriteFile(path, data, 0600); err != nil {
				return err
			}
			_, err = fmt.Fprintln(stdout, path)
			return err
		},
	}
}

var (
	version = "dev"
	commit  = "unknown"
	builtAt = "unknown"
)

func versionCommand(stdout io.Writer) *cli.Command {
	return &cli.Command{
		Name:  "version",
		Usage: "print mts-server version information",
		Action: func(_ *cli.Context) error {
			_, err := fmt.Fprintf(stdout, "mts-server %s commit=%s built_at=%s\n", version, commit, builtAt)
			return err
		},
	}
}

func newLogger(writer io.Writer, cfg logConfig) *slog.Logger {
	level := slog.LevelInfo
	switch cfg.Level {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}
	opts := &slog.HandlerOptions{Level: level}
	if cfg.Format == "json" {
		return slog.New(slog.NewJSONHandler(writer, opts))
	}
	return slog.New(slog.NewTextHandler(writer, opts))
}

func printDefaultConfig(writer io.Writer) error {
	data, err := yaml.MarshalWithOptions(defaultConfig(), yaml.Indent(2))
	if err != nil {
		return err
	}
	_, err = writer.Write(data)
	return err
}

func runServer(ctx context.Context, cfg config, logger *slog.Logger) (err error) {
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()
	runtime, err := openRuntime(ctx, cfg)
	if err != nil {
		return err
	}
	if err := runtime.start(); err != nil {
		return errors.Join(err, runtime.shutdown(ctx))
	}
	runtime.setLogger(logger)
	sighup := make(chan os.Signal, 1)
	signal.Notify(sighup, syscall.SIGHUP)
	defer signal.Stop(sighup)
	logger.Info("mts-server started", "http", cfg.HTTP.Addr, "grpc", cfg.GRPC.Addr)
	for {
		select {
		case <-ctx.Done():
			return shutdownRuntime(runtime, cfg)
		case err := <-runtime.serveErrors():
			return errors.Join(err, runtime.shutdown(ctx))
		case <-sighup:
			if _, err := runtime.reloadConfig(); err != nil {
				logger.Warn("reload config failed", "error", err)
			} else {
				logger.Info("config reloaded")
			}
		}
	}
}

func shutdownRuntime(runtime *serverRuntime, cfg config) error {
	shutdownCtx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.Shutdown))
	defer cancel()
	if err := runtime.shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}
	return nil
}
