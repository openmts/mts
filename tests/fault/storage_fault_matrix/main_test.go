package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"codeberg.org/mts/mts/internal/faultinject"
)

func TestRun(t *testing.T) {
	if err := run(); err != nil {
		t.Fatalf("run() error = %v", err)
	}
}

func TestRunMatrixReportSchema(t *testing.T) {
	report, err := runMatrix(context.Background(), t.TempDir())
	if err != nil {
		t.Fatalf("runMatrix() error = %v", err)
	}
	if !report.OK {
		t.Fatal("report.OK = false, want true")
	}
	if len(report.Cases) == 0 {
		t.Fatal("report cases = 0, want cases")
	}
	seen := make(map[string]struct{}, len(report.Cases))
	for _, item := range report.Cases {
		seen[item.Name] = struct{}{}
		if item.Name == "" || item.Operation == "" || item.Stage == "" || item.Expected == "" {
			t.Fatalf("case has empty schema field: %#v", item)
		}
		if !item.Recovered {
			t.Fatalf("case %s recovered = false, want true", item.Name)
		}
		if item.Rows != 5 {
			t.Fatalf("case %s rows = %d, want 5", item.Name, item.Rows)
		}
		if item.MaintenanceIssues < 0 {
			t.Fatalf("case %s maintenance issues = %d, want non-negative", item.Name, item.MaintenanceIssues)
		}
	}
	for _, name := range []string{"wal-checkpoint-remove-failure", "wal-checkpoint-sync-failure"} {
		if _, ok := seen[name]; !ok {
			t.Fatalf("report missing case %s", name)
		}
	}
}

func TestMainFunction(t *testing.T) {
	main()
}

func TestExpectFaultRejectsMissingOrWrongError(t *testing.T) {
	if err := expectFault(nil, faultinject.OpWrite); err == nil {
		t.Fatal("expectFault(nil) error = nil, want error")
	}
	if err := expectFault(errors.New("plain"), faultinject.OpWrite); err == nil {
		t.Fatal("expectFault(plain) error = nil, want error")
	}
}

func TestVerifyFaultOperationRejectsUnsupportedOperation(t *testing.T) {
	if err := verifyFaultOperation(t.TempDir(), faultinject.Operation("unknown")); err == nil {
		t.Fatal("verifyFaultOperation(unknown) error = nil, want error")
	}
}

func TestRunMatrixRejectsInvalidRoot(t *testing.T) {
	filePath := filepath.Join(t.TempDir(), "file")
	if err := writeTestFile(filePath); err != nil {
		t.Fatalf("writeTestFile() error = %v", err)
	}
	if _, err := runMatrix(context.Background(), filepath.Join(filePath, "child")); err == nil {
		t.Fatal("runMatrix(invalid root) error = nil, want error")
	}
}

func TestRecoveryHelpersRejectInvalidDirectory(t *testing.T) {
	filePath := filepath.Join(t.TempDir(), "file")
	if err := writeTestFile(filePath); err != nil {
		t.Fatalf("writeTestFile() error = %v", err)
	}
	badDir := filepath.Join(filePath, "child")
	if err := writeStableDataset(context.Background(), badDir, "bad"); err == nil {
		t.Fatal("writeStableDataset(invalid) error = nil, want error")
	}
	if _, err := verifyEngineRecovery(context.Background(), badDir); err == nil {
		t.Fatal("verifyEngineRecovery(invalid) error = nil, want error")
	}
}

func TestEngineFaultCasesRejectUntriggeredFaults(t *testing.T) {
	ctx := context.Background()
	if err := runOpenCreateFailure(ctx, t.TempDir(), faultinject.OpWrite); err == nil {
		t.Fatal("runOpenCreateFailure(untriggered) error = nil, want error")
	}
	if err := runReopenFailure(ctx, t.TempDir(), faultinject.OpWrite); err == nil {
		t.Fatal("runReopenFailure(untriggered) error = nil, want error")
	}
}

func TestEngineFaultCasesRejectInvalidDirectory(t *testing.T) {
	ctx := context.Background()
	filePath := filepath.Join(t.TempDir(), "file")
	if err := writeTestFile(filePath); err != nil {
		t.Fatalf("writeTestFile() error = %v", err)
	}
	badDir := filepath.Join(filePath, "child")
	for name, fn := range map[string]func(context.Context, string, faultinject.Operation) error{
		"write":          runWriteFailure,
		"flush_rename":   runFlushRenameFailure,
		"compact_remove": runCompactRemoveFailure,
	} {
		t.Run(name, func(t *testing.T) {
			if err := fn(ctx, badDir, faultinject.OpWrite); err == nil {
				t.Fatal("fault case invalid dir error = nil, want error")
			}
		})
	}
}

func writeTestFile(path string) error {
	return os.WriteFile(path, []byte("x"), 0600)
}
