package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/goccy/go-yaml"
	"github.com/urfave/cli/v2"
)

func newApp(stdout io.Writer, stderr io.Writer) *cli.App {
	app := &cli.App{
		Name:  "mts-server",
		Usage: "run a standalone MTS server with HTTP and gRPC APIs",
		Commands: []*cli.Command{
			serveCommand(stdout, stderr),
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
			logger := slog.New(slog.NewTextHandler(stderr, nil))
			return runServer(cliCtx.Context, cfg, logger)
		},
	}
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
	logger.Info("mts-server started", "http", cfg.HTTP.Addr, "grpc", cfg.GRPC.Addr)
	select {
	case <-ctx.Done():
	case err := <-runtime.serveErrors():
		return errors.Join(err, runtime.shutdown(ctx))
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.Shutdown))
	defer cancel()
	if err := runtime.shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}
	return nil
}
