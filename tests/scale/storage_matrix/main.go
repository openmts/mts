package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	cfg, err := parseConfig(args)
	if err != nil {
		return err
	}
	report, matrixErr := runMatrix(context.Background(), cfg)
	if err := writeOutputs(cfg, report); err != nil {
		return errors.Join(matrixErr, err)
	}
	if matrixErr != nil {
		return matrixErr
	}
	return json.NewEncoder(os.Stdout).Encode(report)
}

func writeOutputs(cfg matrixConfig, report matrixReport) error {
	if cfg.Out != "" {
		data, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return fmt.Errorf("encode matrix report: %w", err)
		}
		if err := writeFile(cfg.Out, data); err != nil {
			return err
		}
	}
	if cfg.Markdown != "" {
		if err := writeFile(cfg.Markdown, []byte(markdownReport(report))); err != nil {
			return err
		}
	}
	return nil
}

func writeFile(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("write output %s: %w", path, err)
	}
	return nil
}
