package main

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestParseSizes(t *testing.T) {
	got, err := parseSizes("100k,1m,10m")
	if err != nil {
		t.Fatalf("parseSizes() error = %v", err)
	}
	want := []scaleSize{
		{Name: "100k", Points: 100_000},
		{Name: "1m", Points: 1_000_000},
		{Name: "10m", Points: 10_000_000},
	}
	if len(got) != len(want) {
		t.Fatalf("sizes len = %d, want %d", len(got), len(want))
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("size[%d] = %#v, want %#v", index, got[index], want[index])
		}
	}
	if _, err := parseSizes("bad"); err == nil {
		t.Fatal("parseSizes(bad) error = nil, want error")
	}
}

func TestBuildCasesUsesIndependentDataDirs(t *testing.T) {
	cfg := matrixConfig{
		Sizes:         []scaleSize{{Name: "100k", Points: 100_000}, {Name: "1m", Points: 1_000_000}},
		Compressions:  []string{"off", "snappy"},
		Durabilities:  []string{"buffered", "write-sync"},
		DataRoot:      "/tmp/mts-matrix",
		Mode:          "compact",
		IngestPath:    "typed",
		BatchSize:     4096,
		MemTableLimit: 8192,
		QueryLimit:    2000,
		ShardDuration: 24 * time.Hour,
		TimestampStep: time.Second,
		CaseTimeout:   time.Minute,
	}
	cases := buildCases(cfg)
	if len(cases) != 8 {
		t.Fatalf("case count = %d, want 8", len(cases))
	}
	first := cases[0]
	if first.DataDir != "/tmp/mts-matrix/100k/off/buffered" {
		t.Fatalf("first data dir = %q, want nested matrix path", first.DataDir)
	}
	last := cases[len(cases)-1]
	if last.DataDir != "/tmp/mts-matrix/1m/snappy/write-sync" {
		t.Fatalf("last data dir = %q, want nested matrix path", last.DataDir)
	}
}

func TestRunnerArgsIncludeDurabilityCompressionAndLimit(t *testing.T) {
	cfg := matrixConfig{
		Mode:          "compact",
		IngestPath:    "typed",
		BatchSize:     4096,
		MemTableLimit: 8192,
		QueryLimit:    2000,
		ShardDuration: 24 * time.Hour,
		TimestampStep: time.Second,
	}
	item := matrixCase{
		Size:        "100k",
		Points:      100_000,
		Compression: "zstd",
		Durability:  "strict-flush",
		DataDir:     "/tmp/mts/100k/zstd/strict-flush",
	}
	args := runnerArgs(cfg, item)
	text := strings.Join(args, " ")
	for _, want := range []string{
		"-points 100000",
		"-compression-algorithm zstd",
		"-durability strict-flush",
		"-query-limit 2000",
		"-shard-duration 24h0m0s",
		"-timestamp-step 1s",
		"-data-dir /tmp/mts/100k/zstd/strict-flush",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("runner args %q missing %q", text, want)
		}
	}
}

func TestMarkdownReportIncludesCoreColumns(t *testing.T) {
	report := matrixReport{
		Cases: []matrixCaseResult{
			{
				Case: matrixCase{Size: "100k", Compression: "off", Durability: "buffered"},
				Report: storageReport{
					WriteDurationNanos:      int64(time.Second),
					CompactionDurationNanos: int64(2 * time.Second),
					ColdQueryLatency:        int64(3 * time.Millisecond),
					HotQueryLatency:         int64(4 * time.Millisecond),
					RSSPeakBytes:            64 << 20,
					DataBytes:               95 << 20,
					ShardCount:              3,
					SSTableBefore:           10,
					SSTableAfter:            1,
				},
			},
		},
	}
	got := markdownReport(report)
	for _, want := range []string{"| size | compression | durability |", "100k", "buffered", "1s", "64.0MiB", "3"} {
		if !strings.Contains(got, want) {
			t.Fatalf("markdown report missing %q in:\n%s", want, got)
		}
	}
}

func TestParseConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{name: "valid minimal", args: []string{"-sizes", "100k", "-compressions", "off", "-durabilities", "buffered"}},
		{name: "invalid compression", args: []string{"-compressions", "bad"}, wantErr: "compressions"},
		{name: "invalid durability", args: []string{"-durabilities", "bad"}, wantErr: "durabilities"},
		{name: "invalid positive options", args: []string{"-batch-size", "0"}, wantErr: "must be positive"},
		{name: "invalid shard duration", args: []string{"-shard-duration", "0"}, wantErr: "must be positive"},
		{name: "invalid timestamp step", args: []string{"-timestamp-step", "0"}, wantErr: "must be positive"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseConfig(tt.args)
			if tt.wantErr == "" && err != nil {
				t.Fatalf("parseConfig() error = %v", err)
			}
			if tt.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tt.wantErr)) {
				t.Fatalf("parseConfig() error = %v, want contains %q", err, tt.wantErr)
			}
		})
	}
}

func TestParseListDeduplicatesAndAllowsCustomSizes(t *testing.T) {
	got, err := parseList(" off,OFF,snappy ", validCompression)
	if err != nil {
		t.Fatalf("parseList() error = %v", err)
	}
	if strings.Join(got, ",") != "off,snappy" {
		t.Fatalf("parseList() = %v, want [off snappy]", got)
	}
	size, err := sizeByName("42")
	if err != nil {
		t.Fatalf("sizeByName(custom) error = %v", err)
	}
	if size.Points != 42 || size.Name != "42" {
		t.Fatalf("sizeByName(custom) = %#v, want 42 points", size)
	}
}

func TestRunWritesOutputsWithFakeRunner(t *testing.T) {
	dir := t.TempDir()
	runner := buildFakeRunner(t, successRunnerSource)
	out := filepath.Join(dir, "reports", "matrix.json")
	markdown := filepath.Join(dir, "reports", "matrix.md")
	err := run(singleCaseArgs(runner, filepath.Join(dir, "data"), out, markdown))
	if err != nil {
		t.Fatalf("run() error = %v", err)
	}
	report := readMatrixReport(t, out)
	if len(report.Cases) != 1 {
		t.Fatalf("case count = %d, want 1", len(report.Cases))
	}
	if report.Cases[0].Report.Rows != 2000 {
		t.Fatalf("rows = %d, want 2000", report.Cases[0].Report.Rows)
	}
	assertFileMode(t, out, 0600)
	assertContainsFile(t, markdown, "buffered")
}

func TestRunReturnsCaseErrorAndStillWritesReport(t *testing.T) {
	dir := t.TempDir()
	runner := buildFakeRunner(t, failingRunnerSource)
	out := filepath.Join(dir, "matrix.json")
	err := run(singleCaseArgs(runner, filepath.Join(dir, "data"), out, ""))
	if err == nil || !strings.Contains(err.Error(), "matrix case failed") {
		t.Fatalf("run() error = %v, want matrix case failed", err)
	}
	report := readMatrixReport(t, out)
	if len(report.Cases) != 1 || !strings.Contains(report.Cases[0].Error, "boom") {
		t.Fatalf("case error = %#v, want runner stderr", report.Cases)
	}
}

func TestRunRejectsBadJSONRunner(t *testing.T) {
	dir := t.TempDir()
	runner := buildFakeRunner(t, badJSONRunnerSource)
	err := run(singleCaseArgs(runner, filepath.Join(dir, "data"), "", ""))
	if err == nil || !strings.Contains(err.Error(), "decode report") {
		t.Fatalf("run() error = %v, want decode report", err)
	}
}

func TestMainSuccessPath(t *testing.T) {
	dir := t.TempDir()
	runner := buildFakeRunner(t, successRunnerSource)
	oldArgs := os.Args
	os.Args = append([]string{"storage_matrix"}, singleCaseArgs(runner, filepath.Join(dir, "data"), "", "")...)
	t.Cleanup(func() { os.Args = oldArgs })
	main()
}

func TestRunRejectsInvalidArgs(t *testing.T) {
	if err := run([]string{"-sizes", "bad"}); err == nil {
		t.Fatal("run(invalid args) error = nil, want error")
	}
}

func TestRunCaseReportsDataDirError(t *testing.T) {
	dir := t.TempDir()
	parentFile := filepath.Join(dir, "file-parent")
	if err := os.WriteFile(parentFile, []byte("x"), 0600); err != nil {
		t.Fatalf("write parent file: %v", err)
	}
	cfg := matrixConfig{CaseTimeout: time.Second}
	item := matrixCase{Size: "100k", Compression: "off", Durability: "buffered", DataDir: filepath.Join(parentFile, "child")}
	result := runCase(context.Background(), cfg, buildFakeRunner(t, successRunnerSource), item)
	if result.Error == "" || !strings.Contains(result.Error, "data dir") {
		t.Fatalf("runCase() error = %q, want data dir error", result.Error)
	}
}

func TestPrepareRootTempLifecycle(t *testing.T) {
	root, cleanup, err := prepareRoot("")
	if err != nil {
		t.Fatalf("prepareRoot() error = %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(root) })
	assertDirMode(t, root, 0700)
	if err := cleanup(); err != nil {
		t.Fatalf("cleanup() error = %v", err)
	}
	if _, err := os.Stat(root); !os.IsNotExist(err) {
		t.Fatalf("temp root stat error = %v, want not exist", err)
	}
}

func TestPrepareRootReportsCreateError(t *testing.T) {
	dir := t.TempDir()
	parentFile := filepath.Join(dir, "file-parent")
	if err := os.WriteFile(parentFile, []byte("x"), 0600); err != nil {
		t.Fatalf("write parent file: %v", err)
	}
	_, _, err := prepareRoot(filepath.Join(parentFile, "child"))
	if err == nil || !strings.Contains(err.Error(), "create data root") {
		t.Fatalf("prepareRoot() error = %v, want create data root", err)
	}
}

func TestPrepareRunnerBuildsDefaultRunner(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	t.Cleanup(cancel)
	runner, cleanup, err := prepareRunner(ctx, "")
	if err != nil {
		t.Fatalf("prepareRunner() error = %v", err)
	}
	t.Cleanup(func() { _ = cleanup() })
	if _, err := os.Stat(runner); err != nil {
		t.Fatalf("runner stat error = %v", err)
	}
	dir := filepath.Dir(runner)
	if err := cleanup(); err != nil {
		t.Fatalf("cleanup() error = %v", err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("runner dir stat error = %v, want not exist", err)
	}
}

func TestPrepareRunnerReportsBuildError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _, err := prepareRunner(ctx, "")
	if err == nil || !strings.Contains(err.Error(), "build storage_10m runner") {
		t.Fatalf("prepareRunner(canceled) error = %v, want build error", err)
	}
}

func TestFindRepoRootReportsMissingGoMod(t *testing.T) {
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("get wd: %v", err)
	}
	temp := t.TempDir()
	if err := os.Chdir(temp); err != nil {
		t.Fatalf("chdir temp: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldWd) })
	_, err = findRepoRoot()
	if err == nil || !strings.Contains(err.Error(), "go.mod not found") {
		t.Fatalf("findRepoRoot() error = %v, want missing go.mod", err)
	}
}

func TestWriteOutputsReportsFileErrors(t *testing.T) {
	dir := t.TempDir()
	outDir := filepath.Join(dir, "existing-dir")
	if err := os.Mkdir(outDir, 0700); err != nil {
		t.Fatalf("mkdir output dir: %v", err)
	}
	err := writeOutputs(matrixConfig{Out: outDir}, matrixReport{})
	if err == nil || !strings.Contains(err.Error(), "write output") {
		t.Fatalf("writeOutputs(dir) error = %v, want write output", err)
	}
	parentFile := filepath.Join(dir, "file-parent")
	if err := os.WriteFile(parentFile, []byte("x"), 0600); err != nil {
		t.Fatalf("write parent file: %v", err)
	}
	err = writeFile(filepath.Join(parentFile, "child", "out.json"), []byte("{}"))
	if err == nil || !strings.Contains(err.Error(), "create output dir") {
		t.Fatalf("writeFile() error = %v, want create output dir", err)
	}
}

func TestReportFormattingHelpers(t *testing.T) {
	if got := formatDuration(0); got != "0s" {
		t.Fatalf("formatDuration(0) = %q, want 0s", got)
	}
	if got := formatBytes(0); got != "0MiB" {
		t.Fatalf("formatBytes(0) = %q, want 0MiB", got)
	}
	if got := commandError(context.Canceled, "stderr text"); !strings.Contains(got, "stderr text") {
		t.Fatalf("commandError() = %q, want stderr", got)
	}
	if got := commandError(context.Canceled, ""); !strings.Contains(got, "canceled") {
		t.Fatalf("commandError(no stderr) = %q, want error text", got)
	}
	report := matrixReport{Cases: []matrixCaseResult{{Case: matrixCase{Size: "100k", Compression: "off", Durability: "buffered"}, Error: "failed"}}}
	if err := firstCaseError(report); err == nil || !strings.Contains(err.Error(), "100k/off/buffered") {
		t.Fatalf("firstCaseError() = %v, want case id", err)
	}
	if err := firstCaseError(matrixReport{}); err != nil {
		t.Fatalf("firstCaseError(empty) = %v, want nil", err)
	}
	if got := markdownReport(report); !strings.Contains(got, "failed") {
		t.Fatalf("markdownReport(error) = %q, want failed status", got)
	}
}

func singleCaseArgs(runner string, dataRoot string, out string, markdown string) []string {
	args := []string{
		"-runner", runner,
		"-sizes", "100k",
		"-compressions", "off",
		"-durabilities", "buffered",
		"-data-root", dataRoot,
		"-case-timeout", "5s",
	}
	if out != "" {
		args = append(args, "-out", out)
	}
	if markdown != "" {
		args = append(args, "-markdown", markdown)
	}
	return args
}

func buildFakeRunner(t *testing.T, source string) string {
	t.Helper()
	dir := t.TempDir()
	src := filepath.Join(dir, "runner.go")
	if err := os.WriteFile(src, []byte(source), 0600); err != nil {
		t.Fatalf("write fake runner source: %v", err)
	}
	bin := filepath.Join(dir, "runner")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	cmd := exec.Command("go", "build", "-o", bin, src)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build fake runner: %v output=%s", err, output)
	}
	return bin
}

func readMatrixReport(t *testing.T, path string) matrixReport {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read report: %v", err)
	}
	var report matrixReport
	if err := json.Unmarshal(data, &report); err != nil {
		t.Fatalf("decode report: %v", err)
	}
	return report
}

func assertContainsFile(t *testing.T, path string, want string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if !strings.Contains(string(data), want) {
		t.Fatalf("%s missing %q", path, want)
	}
}

func assertFileMode(t *testing.T, path string, want os.FileMode) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat file: %v", err)
	}
	if got := info.Mode().Perm(); got != want {
		t.Fatalf("file mode = %v, want %v", got, want)
	}
}

func assertDirMode(t *testing.T, path string, want os.FileMode) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat dir: %v", err)
	}
	if got := info.Mode().Perm(); got != want {
		t.Fatalf("dir mode = %v, want %v", got, want)
	}
}

const successRunnerSource = `package main

import (
	"encoding/json"
	"os"
)

func main() {
	_ = json.NewEncoder(os.Stdout).Encode(map[string]any{
		"write_duration_nanos": 1000,
		"compaction_duration_nanos": 2000,
		"cold_query_latency_nanos": 3000,
		"hot_query_latency_nanos": 4000,
		"rss_peak_bytes": 1048576,
		"data_bytes": 2048,
		"sstable_count_before_compaction": 2,
		"sstable_count_after_compaction": 1,
		"rows": 2000,
	})
}
`

const failingRunnerSource = `package main

import (
	"fmt"
	"os"
)

func main() {
	_, _ = fmt.Fprint(os.Stderr, "boom")
	os.Exit(3)
}
`

const badJSONRunnerSource = `package main

import "fmt"

func main() {
	fmt.Print("{bad-json")
}
`
