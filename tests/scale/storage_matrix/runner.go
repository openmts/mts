package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

func runMatrix(ctx context.Context, cfg matrixConfig) (matrixReport, error) {
	root, cleanup, err := prepareRoot(cfg.DataRoot)
	if err != nil {
		return matrixReport{}, err
	}
	cfg.DataRoot = root
	runner, cleanupRunner, err := prepareRunner(ctx, cfg.Runner)
	if err != nil {
		return matrixReport{}, errors.Join(err, cleanup())
	}
	report := matrixReport{StartedAt: time.Now(), DataRoot: root}
	for _, item := range buildCases(cfg) {
		result := runCase(ctx, cfg, runner, item)
		report.Cases = append(report.Cases, result)
	}
	report.FinishedAt = time.Now()
	return report, errors.Join(firstCaseError(report), cleanupRunner(), cleanup())
}

func prepareRoot(root string) (string, func() error, error) {
	if root != "" {
		if err := os.MkdirAll(root, 0700); err != nil {
			return "", nil, fmt.Errorf("create data root: %w", err)
		}
		return root, func() error { return nil }, nil
	}
	dir, err := os.MkdirTemp("", "mts-storage-matrix-*")
	if err != nil {
		return "", nil, fmt.Errorf("create temp data root: %w", err)
	}
	if err := os.Chmod(dir, 0700); err != nil {
		_ = os.RemoveAll(dir)
		return "", nil, fmt.Errorf("chmod temp data root: %w", err)
	}
	return dir, func() error { return os.RemoveAll(dir) }, nil
}

func prepareRunner(ctx context.Context, runner string) (string, func() error, error) {
	if runner != "" {
		return runner, func() error { return nil }, nil
	}
	root, err := findRepoRoot()
	if err != nil {
		return "", nil, err
	}
	dir, err := os.MkdirTemp("", "mts-storage-matrix-runner-*")
	if err != nil {
		return "", nil, fmt.Errorf("create temp runner dir: %w", err)
	}
	if err := os.Chmod(dir, 0700); err != nil {
		_ = os.RemoveAll(dir)
		return "", nil, fmt.Errorf("chmod temp runner dir: %w", err)
	}
	bin := filepath.Join(dir, "storage_10m")
	cmd := exec.CommandContext(ctx, "go", "build", "-o", bin, "./tests/scale/storage_10m")
	cmd.Dir = root
	if output, err := cmd.CombinedOutput(); err != nil {
		_ = os.RemoveAll(dir)
		return "", nil, fmt.Errorf("build storage_10m runner: %w output=%s", err, output)
	}
	return bin, func() error { return os.RemoveAll(dir) }, nil
}

func findRepoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("get working directory: %w", err)
	}
	for {
		mod := filepath.Join(dir, "go.mod")
		if _, err := os.Stat(mod); err == nil {
			return dir, nil
		} else if !os.IsNotExist(err) {
			return "", fmt.Errorf("stat go.mod: %w", err)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("find repository root from %s: go.mod not found", dir)
		}
		dir = parent
	}
}

func runCase(ctx context.Context, cfg matrixConfig, runner string, item matrixCase) matrixCaseResult {
	result := matrixCaseResult{Case: item, StartedAt: time.Now()}
	caseCtx, cancel := context.WithTimeout(ctx, cfg.CaseTimeout)
	defer cancel()
	if err := recreateDataDir(item.DataDir); err != nil {
		result.Error = err.Error()
		result.FinishedAt = time.Now()
		result.Duration = result.FinishedAt.Sub(result.StartedAt)
		return result
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd := exec.CommandContext(caseCtx, runner, runnerArgs(cfg, item)...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	result.FinishedAt = time.Now()
	result.Duration = result.FinishedAt.Sub(result.StartedAt)
	if err != nil {
		result.Error = commandError(err, stderr.String())
		return result
	}
	if err := json.Unmarshal(stdout.Bytes(), &result.Report); err != nil {
		result.Error = fmt.Sprintf("decode report: %v stdout=%s", err, stdout.String())
	}
	return result
}

func recreateDataDir(dir string) error {
	if err := os.RemoveAll(dir); err != nil {
		return fmt.Errorf("remove data dir: %w", err)
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("create data dir: %w", err)
	}
	return nil
}

func commandError(err error, stderr string) string {
	if stderr == "" {
		return err.Error()
	}
	return fmt.Sprintf("%v stderr=%s", err, stderr)
}

func firstCaseError(report matrixReport) error {
	for _, item := range report.Cases {
		if item.Error != "" {
			return fmt.Errorf("matrix case failed: %s/%s/%s: %s",
				item.Case.Size,
				item.Case.Compression,
				item.Case.Durability,
				item.Error,
			)
		}
	}
	return nil
}
