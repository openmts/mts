package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/openmts/mts/internal/storagecheck"
)

func main() {
	os.Exit(runMain(os.Args[1:], os.Stdout, os.Stderr))
}

func runMain(args []string, stdout io.Writer, stderr io.Writer) int {
	// stdout 用于输出 JSON 结果，结构化日志写入 stderr 避免污染结果
	logger := slog.New(slog.NewTextHandler(stderr, nil))
	slog.SetDefault(logger)
	return run(args, stdout, stderr)
}

func run(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) == 0 {
		_, _ = fmt.Fprintln(stderr, "usage: mts-storage <check|repair|migrate|snapshot|restore> [flags] <path>")
		return 2
	}
	switch args[0] {
	case "check":
		return runCheck(args[1:], stdout, stderr)
	case "repair":
		return runRepair(args[1:], stdout, stderr)
	case "migrate":
		return runMigrate(args[1:], stdout, stderr)
	case "snapshot":
		return runSnapshot(args[1:], stdout, stderr)
	case "restore":
		return runRestore(args[1:], stdout, stderr)
	default:
		_, _ = fmt.Fprintf(stderr, "unknown command %q\n", args[0])
		return 2
	}
}

func runCheck(args []string, stdout io.Writer, stderr io.Writer) int {
	flags := flag.NewFlagSet("check", flag.ContinueOnError)
	flags.SetOutput(stderr)
	unknown := flags.String("unknown-files", string(storagecheck.UnknownFileIgnore), "unknown file policy: ignore, warn, fatal")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() != 1 {
		_, _ = fmt.Fprintln(stderr, "usage: mts-storage check [--unknown-files=ignore|warn|fatal] <path>")
		return 2
	}
	report, err := storagecheck.Check(flags.Arg(0), storagecheck.Options{
		UnknownFiles: storagecheck.UnknownFilePolicy(*unknown),
	})
	return writeJSON(stdout, stderr, report, err)
}

func runRepair(args []string, stdout io.Writer, stderr io.Writer) int {
	flags := flag.NewFlagSet("repair", flag.ContinueOnError)
	flags.SetOutput(stderr)
	dryRun := flags.Bool("dry-run", false, "print repair plan without applying")
	apply := flags.Bool("apply", false, "apply safe repair actions")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() != 1 || (*dryRun && *apply) {
		_, _ = fmt.Fprintln(stderr, "usage: mts-storage repair [--dry-run|--apply] <path>")
		return 2
	}
	plan, err := storagecheck.Repair(flags.Arg(0), storagecheck.RepairOptions{Apply: *apply})
	return writeJSON(stdout, stderr, plan, err)
}

func runMigrate(args []string, stdout io.Writer, stderr io.Writer) int {
	flags := flag.NewFlagSet("migrate", flag.ContinueOnError)
	flags.SetOutput(stderr)
	dryRun := flags.Bool("dry-run", false, "print migration plan without applying")
	apply := flags.Bool("apply", false, "apply migration checkpoint")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() != 1 || (*dryRun && *apply) {
		_, _ = fmt.Fprintln(stderr, "usage: mts-storage migrate [--dry-run|--apply] <path>")
		return 2
	}
	result, err := storagecheck.Migrate(flags.Arg(0), storagecheck.MigrateOptions{Apply: *apply})
	return writeJSON(stdout, stderr, result, err)
}

func runSnapshot(args []string, stdout io.Writer, stderr io.Writer) int {
	flags := flag.NewFlagSet("snapshot", flag.ContinueOnError)
	flags.SetOutput(stderr)
	overwrite := flags.Bool("overwrite", false, "overwrite target directory")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() != 2 {
		_, _ = fmt.Fprintln(stderr, "usage: mts-storage snapshot [--overwrite] <source> <target>")
		return 2
	}
	result, err := storagecheck.Snapshot(
		flags.Arg(0),
		flags.Arg(1),
		storagecheck.SnapshotOptions{Overwrite: *overwrite},
	)
	return writeJSON(stdout, stderr, result, err)
}

func runRestore(args []string, stdout io.Writer, stderr io.Writer) int {
	flags := flag.NewFlagSet("restore", flag.ContinueOnError)
	flags.SetOutput(stderr)
	overwrite := flags.Bool("overwrite", false, "overwrite target directory")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() != 2 {
		_, _ = fmt.Fprintln(stderr, "usage: mts-storage restore [--overwrite] <snapshot> <target>")
		return 2
	}
	result, err := storagecheck.Restore(
		flags.Arg(0),
		flags.Arg(1),
		storagecheck.SnapshotOptions{Overwrite: *overwrite},
	)
	return writeJSON(stdout, stderr, result, err)
}

func writeJSON(stdout io.Writer, stderr io.Writer, value any, err error) int {
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}
	encoder := json.NewEncoder(stdout)
	if encodeErr := encoder.Encode(value); encodeErr != nil {
		_, _ = fmt.Fprintln(stderr, encodeErr)
		return 1
	}
	return 0
}
