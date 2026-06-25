package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"

	mts "github.com/openmts/mts"
)

const grpcServiceName = "mts.v1.MTSServer"

const (
	e2eAdminToken = "e2e-admin-token"
	e2eDataToken  = "e2e-data-token"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("mts_server_protocols failed: %v", err)
	}
	log.Print("mts_server_protocols passed")
}

func run() (err error) {
	root, err := os.MkdirTemp("", "mts-server-e2e-*")
	if err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}
	if err := os.Chmod(root, 0700); err != nil {
		_ = os.RemoveAll(root)
		return fmt.Errorf("chmod temp dir: %w", err)
	}
	defer func() {
		err = errors.Join(err, os.RemoveAll(root))
	}()
	return runWithDir(root)
}

func runWithDir(root string) error {
	repoRoot, err := findRepoRoot()
	if err != nil {
		return err
	}
	binary := filepath.Join(root, "mts-server")
	if err := buildServerBinary(repoRoot, binary); err != nil {
		return err
	}
	config, httpAddr, grpcAddr, err := writeServerConfig(root)
	if err != nil {
		return err
	}
	server, err := startServer(binary, config)
	if err != nil {
		return err
	}
	defer server.stop()
	if err := waitHTTPReady(httpAddr, server); err != nil {
		return err
	}
	if err := exerciseHTTP(httpAddr); err != nil {
		return err
	}
	if err := exerciseGRPC(grpcAddr); err != nil {
		return err
	}
	return nil
}

func findRepoRoot() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("get working dir: %w", err)
	}
	for dir := wd; ; dir = filepath.Dir(dir) {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("repo root not found from %s", wd)
		}
	}
}

func buildServerBinary(repoRoot string, binary string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "build", "-o", binary, "./cmd/mts-server")
	cmd.Dir = repoRoot
	cmd.Env = append(os.Environ(), "GOSUMDB=sum.golang.org")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("build mts-server: %w output=%s", err, string(output))
	}
	if err := os.Chmod(binary, 0700); err != nil {
		return fmt.Errorf("chmod mts-server binary: %w", err)
	}
	return nil
}

func writeServerConfig(root string) (string, string, string, error) {
	httpAddr, err := freeTCPAddr()
	if err != nil {
		return "", "", "", err
	}
	grpcAddr, err := freeTCPAddr()
	if err != nil {
		return "", "", "", err
	}
	config := filepath.Join(root, "mts-server.yaml")
	dataDir := filepath.Join(root, "data")
	body := fmt.Sprintf(`data_dir: %s
http:
  enabled: true
  addr: %s
grpc:
  enabled: true
  addr: %s
auth:
  admin_token: %s
  data_tokens: [%s]
  require_user: true
limits:
  default_query_limit: 1000
  max_query_limit: 10000
  max_write_points: 1000
  max_request_body_bytes: 1048576
observability:
  access_log: false
backup:
  dir: %s
engine:
  default_database: default
  default_retention_policy: autogen
  shard_duration: 1h
  retention: 0s
  memtable_max_samples: 1000
  flush_sync: false
  compression:
    enabled: true
    algorithm: snappy
    min_page_values: 1
  compaction:
    enabled: true
    background_interval: 0s
    level0_part_limit: 4
    max_cascade_steps: 8
shutdown_timeout: 2s
`, filepath.ToSlash(dataDir), httpAddr, grpcAddr, e2eAdminToken, e2eDataToken, filepath.ToSlash(filepath.Join(root, "backups")))
	if err := os.WriteFile(config, []byte(body), 0600); err != nil {
		return "", "", "", fmt.Errorf("write server config: %w", err)
	}
	return config, httpAddr, grpcAddr, nil
}

func freeTCPAddr() (string, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", fmt.Errorf("listen free addr: %w", err)
	}
	addr := listener.Addr().String()
	if err := listener.Close(); err != nil {
		return "", fmt.Errorf("close free addr listener: %w", err)
	}
	return addr, nil
}

type serverProcess struct {
	cmd    *exec.Cmd
	stdout bytes.Buffer
	stderr bytes.Buffer
}

func startServer(binary string, config string) (*serverProcess, error) {
	cmd := exec.Command(binary, "serve", "--config", config)
	process := &serverProcess{cmd: cmd}
	cmd.Stdout = &process.stdout
	cmd.Stderr = &process.stderr
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start mts-server: %w", err)
	}
	return process, nil
}

func (p *serverProcess) stop() {
	if p == nil || p.cmd == nil || p.cmd.Process == nil {
		return
	}
	_ = p.cmd.Process.Signal(os.Interrupt)
	done := make(chan struct{})
	go func() {
		_ = p.cmd.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		_ = p.cmd.Process.Kill()
		<-done
	}
}

func (p *serverProcess) output() string {
	if p == nil {
		return ""
	}
	return "stdout=" + p.stdout.String() + " stderr=" + p.stderr.String()
}

func waitHTTPReady(addr string, process *serverProcess) error {
	client := http.Client{Timeout: 500 * time.Millisecond}
	url := "http://" + addr + "/readyz"
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if process.cmd.ProcessState != nil && process.cmd.ProcessState.Exited() {
			return fmt.Errorf("mts-server exited before ready: %s", process.output())
		}
		resp, err := client.Get(url)
		if err == nil {
			body, readErr := io.ReadAll(resp.Body)
			closeErr := resp.Body.Close()
			if readErr != nil {
				return fmt.Errorf("read ready response: %w", readErr)
			}
			if closeErr != nil {
				return fmt.Errorf("close ready response: %w", closeErr)
			}
			if resp.StatusCode == http.StatusOK && bytes.Contains(body, []byte("Healthy")) {
				return nil
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("wait ready timeout: %s", process.output())
}

func exerciseHTTP(addr string) error {
	client := http.Client{Timeout: 5 * time.Second}
	if err := assertHTTPHealth(client, addr, "/healthz"); err != nil {
		return err
	}
	if err := assertHTTPHealth(client, addr, "/readyz"); err != nil {
		return err
	}
	if err := postHTTPJSON(client, addr, "/api/v1/users", mts.User{Name: "http-e2e"}, &okResponse{}); err != nil {
		return err
	}
	if err := postHTTPJSON(client, addr, "/api/v1/users/http-e2e/database-permissions/default/read", emptyRequest{}, &okResponse{}); err != nil {
		return err
	}
	if err := postHTTPJSON(client, addr, "/api/v1/users/http-e2e/database-permissions/default/write", emptyRequest{}, &okResponse{}); err != nil {
		return err
	}
	point := testPoint("http_cpu", "http-host", 1, 0.42)
	if err := postHTTPJSON(client, addr, "/api/v1/data/write", writeRequest{Points: []mts.Point{point}}, &writeResponse{}); err != nil {
		return err
	}
	batch := testBatch("http_typed_cpu", "http-typed", 3, 0.64)
	if err := postHTTPJSON(client, addr, "/api/v1/data/write/typed", typedWriteRequest{Batch: batch}, &writeResponse{}); err != nil {
		return err
	}
	query, err := testQuery("http_cpu")
	if err != nil {
		return err
	}
	var rows queryRowsResponse
	if err := postHTTPJSON(client, addr, "/api/v1/data/query/rows", queryRowsRequest{Query: query}, &rows); err != nil {
		return err
	}
	if err := assertRows(rows.Rows, "http_cpu", "http-host", 0.42); err != nil {
		return fmt.Errorf("http query rows: %w", err)
	}
	columnsQuery, err := testQuery("http_typed_cpu")
	if err != nil {
		return err
	}
	var columns queryColumnsResponse
	if err := postHTTPJSON(client, addr, "/api/v1/data/query/columns", queryRequest{Query: columnsQuery}, &columns); err != nil {
		return err
	}
	if len(columns.Columns) != 1 || len(columns.Columns[0].Timestamps) != 1 {
		return fmt.Errorf("http query columns=%#v, want one column", columns.Columns)
	}
	var explain queryExplainResponse
	if err := postHTTPJSON(client, addr, "/api/v1/data/query/explain", queryRequest{Query: columnsQuery}, &explain); err != nil {
		return err
	}
	if explain.Result.Explain.Measurement != "http_typed_cpu" {
		return fmt.Errorf("http explain=%#v, want http_typed_cpu", explain.Result.Explain)
	}
	var cfg configResponse
	if err := getHTTPJSON(client, addr, "/api/v1/admin/config/effective", &cfg); err != nil {
		return err
	}
	if cfg.Config.DataDir == "" {
		return fmt.Errorf("http config data_dir empty")
	}
	var spec apiSpecResponse
	if err := getHTTPJSON(client, addr, "/api/v1/admin/api-spec", &spec); err != nil {
		return err
	}
	if spec.Version != "v1" {
		return fmt.Errorf("api spec=%#v, want v1", spec)
	}
	var storage storageValidateResponse
	if err := postHTTPJSON(client, addr, "/api/v1/admin/storage/validate", emptyRequest{}, &storage); err != nil {
		return err
	}
	if !storage.OK {
		return fmt.Errorf("storage validate=%#v, want ok", storage)
	}
	if err := assertHTTPMetrics(client, addr); err != nil {
		return err
	}
	if err := exerciseHTTPUsersAndDownsample(client, addr); err != nil {
		return err
	}
	var flush maintenanceResponse
	if err := postHTTPJSON(client, addr, "/api/v1/admin/flush", emptyRequest{}, &flush); err != nil {
		return err
	}
	if !flush.OK {
		return fmt.Errorf("http flush ok=false")
	}
	var compact maintenanceResponse
	if err := postHTTPJSON(client, addr, "/api/v1/admin/compact", emptyRequest{}, &compact); err != nil {
		return err
	}
	if !compact.OK {
		return fmt.Errorf("http compact ok=false")
	}
	return nil
}

func exerciseHTTPUsersAndDownsample(client http.Client, addr string) error {
	if err := postHTTPJSON(client, addr, "/api/v1/users", mts.User{Name: "http-user"}, &okResponse{}); err != nil {
		return err
	}
	if err := postHTTPJSON(client, addr, "/api/v1/users/http-user/database-permissions/default/read", emptyRequest{}, &okResponse{}); err != nil {
		return err
	}
	var user userResponse
	if err := getHTTPJSON(client, addr, "/api/v1/users/http-user", &user); err != nil {
		return err
	}
	if user.User.Name != "http-user" {
		return fmt.Errorf("http user=%#v, want http-user", user.User)
	}
	policy := testDownsamplePolicy("http_rollup_cpu", "http_typed_cpu")
	if err := postHTTPJSON(client, addr, "/api/v1/admin/downsample/policies", policy, &okResponse{}); err != nil {
		return err
	}
	var policies downsamplePoliciesResponse
	if err := getHTTPJSON(client, addr, "/api/v1/admin/downsample/policies", &policies); err != nil {
		return err
	}
	if len(policies.Policies) == 0 {
		return fmt.Errorf("http downsample policies empty")
	}
	var dryRun downsampleDryRunResponse
	return postHTTPJSON(client, addr, "/api/v1/admin/downsample/policies/http_rollup_cpu/dry-run", downsampleRangeRequest{
		StartUnix: 1,
		EndUnix:   int64(time.Hour),
	}, &dryRun)
}

func assertHTTPHealth(client http.Client, addr string, path string) error {
	resp, err := client.Get("http://" + addr + path)
	if err != nil {
		return fmt.Errorf("get %s: %w", path, err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s status=%d, want 200", path, resp.StatusCode)
	}
	var health mts.HealthSnapshot
	if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
		return fmt.Errorf("decode %s health: %w", path, err)
	}
	if !health.Healthy || !health.Ready {
		return fmt.Errorf("%s health=%#v, want healthy and ready", path, health)
	}
	return nil
}

func postHTTPJSON(client http.Client, addr string, path string, in any, out any) error {
	body, err := json.Marshal(in)
	if err != nil {
		return fmt.Errorf("marshal %s request: %w", path, err)
	}
	req, err := http.NewRequest(http.MethodPost, "http://"+addr+path, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("new post %s: %w", path, err)
	}
	req.Header.Set("Content-Type", "application/json")
	setHTTPAuthHeaders(req, path)
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("post %s: %w", path, err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	if resp.StatusCode != http.StatusOK {
		response, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("post %s status=%d body=%s", path, resp.StatusCode, string(response))
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode %s response: %w", path, err)
	}
	return nil
}

func getHTTPJSON(client http.Client, addr string, path string, out any) error {
	req, err := http.NewRequest(http.MethodGet, "http://"+addr+path, nil)
	if err != nil {
		return fmt.Errorf("new get %s: %w", path, err)
	}
	setHTTPAuthHeaders(req, path)
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("get %s: %w", path, err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	if resp.StatusCode != http.StatusOK {
		response, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("get %s status=%d body=%s", path, resp.StatusCode, string(response))
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode %s response: %w", path, err)
	}
	return nil
}

func setHTTPAuthHeaders(req *http.Request, path string) {
	if strings.HasPrefix(path, "/api/v1/admin") || strings.HasPrefix(path, "/api/v1/users") || strings.HasPrefix(path, "/api/v1/authz") {
		req.Header.Set("Authorization", "Bearer "+e2eAdminToken)
	}
	if strings.HasPrefix(path, "/api/v1/data") {
		req.Header.Set("X-MTS-Data-Token", e2eDataToken)
		req.Header.Set("X-MTS-User", "http-e2e")
	}
}

func assertHTTPMetrics(client http.Client, addr string) error {
	resp, err := client.Get("http://" + addr + "/metrics")
	if err != nil {
		return fmt.Errorf("get metrics: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read metrics: %w", err)
	}
	if resp.StatusCode != http.StatusOK || !bytes.Contains(body, []byte("mts_health_ready")) {
		return fmt.Errorf("metrics status=%d body=%s", resp.StatusCode, string(body))
	}
	return nil
}

func exerciseGRPC(addr string) error {
	conn, err := grpc.NewClient(
		addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(grpc.ForceCodec(jsonCodec{})),
	)
	if err != nil {
		return fmt.Errorf("new grpc client: %w", err)
	}
	defer func() {
		_ = conn.Close()
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	adminCtx := metadata.NewOutgoingContext(ctx, metadata.Pairs("authorization", "Bearer "+e2eAdminToken))
	dataCtx := metadata.NewOutgoingContext(ctx, metadata.Pairs("x-mts-data-token", e2eDataToken, "x-mts-user", "grpc-e2e"))
	var health mts.HealthSnapshot
	if err := invokeGRPC(ctx, conn, "Health", emptyRequest{}, &health); err != nil {
		return fmt.Errorf("grpc health: %w", err)
	}
	if !health.Healthy || !health.Ready {
		return fmt.Errorf("grpc health=%#v, want healthy and ready", health)
	}
	if err := invokeGRPC(adminCtx, conn, "CreateUser", mts.User{Name: "grpc-e2e"}, &okResponse{}); err != nil {
		return fmt.Errorf("grpc create user: %w", err)
	}
	if err := invokeGRPC(adminCtx, conn, "GrantDatabasePermission", databasePermissionRequest{UserName: "grpc-e2e", Database: "default", Permission: mts.DatabasePermissionRead}, &okResponse{}); err != nil {
		return fmt.Errorf("grpc grant read: %w", err)
	}
	if err := invokeGRPC(adminCtx, conn, "GrantDatabasePermission", databasePermissionRequest{UserName: "grpc-e2e", Database: "default", Permission: mts.DatabasePermissionWrite}, &okResponse{}); err != nil {
		return fmt.Errorf("grpc grant write: %w", err)
	}
	point := testPoint("grpc_cpu", "grpc-host", 2, 0.84)
	var write writeResponse
	if err := invokeGRPC(dataCtx, conn, "Write", writeRequest{Points: []mts.Point{point}}, &write); err != nil {
		return fmt.Errorf("grpc write: %w", err)
	}
	if !write.OK {
		return fmt.Errorf("grpc write ok=false")
	}
	query, err := testQuery("grpc_cpu")
	if err != nil {
		return err
	}
	var rows queryRowsResponse
	if err := invokeGRPC(dataCtx, conn, "QueryRows", queryRowsRequest{Query: query}, &rows); err != nil {
		return fmt.Errorf("grpc query rows: %w", err)
	}
	if err := assertRows(rows.Rows, "grpc_cpu", "grpc-host", 0.84); err != nil {
		return fmt.Errorf("grpc query rows: %w", err)
	}
	batch := testBatch("grpc_typed_cpu", "grpc-typed", 4, 0.93)
	if err := invokeGRPC(dataCtx, conn, "WriteTypedBatch", typedWriteRequest{Batch: batch}, &writeResponse{}); err != nil {
		return fmt.Errorf("grpc typed write: %w", err)
	}
	columnsQuery, err := testQuery("grpc_typed_cpu")
	if err != nil {
		return err
	}
	var columns queryColumnsResponse
	if err := invokeGRPC(dataCtx, conn, "QueryColumns", queryRequest{Query: columnsQuery}, &columns); err != nil {
		return fmt.Errorf("grpc query columns: %w", err)
	}
	if len(columns.Columns) != 1 {
		return fmt.Errorf("grpc columns=%#v, want one", columns.Columns)
	}
	var cfg configResponse
	if err := invokeGRPC(adminCtx, conn, "GetEffectiveConfig", emptyRequest{}, &cfg); err != nil {
		return fmt.Errorf("grpc config: %w", err)
	}
	policy := testDownsamplePolicy("grpc_rollup_cpu", "grpc_typed_cpu")
	if err := invokeGRPC(adminCtx, conn, "CreateDownsamplePolicy", policy, &okResponse{}); err != nil {
		return fmt.Errorf("grpc create downsample: %w", err)
	}
	var dryRun downsampleDryRunResponse
	if err := invokeGRPC(adminCtx, conn, "DryRunDownsamplePolicy", downsamplePolicyRangeRequest{
		Name:      "grpc_rollup_cpu",
		StartUnix: 1,
		EndUnix:   int64(time.Hour),
	}, &dryRun); err != nil {
		return fmt.Errorf("grpc downsample dry-run: %w", err)
	}
	var flush maintenanceResponse
	if err := invokeGRPC(adminCtx, conn, "Flush", emptyRequest{}, &flush); err != nil {
		return fmt.Errorf("grpc flush: %w", err)
	}
	if !flush.OK {
		return fmt.Errorf("grpc flush ok=false")
	}
	var compact maintenanceResponse
	if err := invokeGRPC(adminCtx, conn, "Compact", emptyRequest{}, &compact); err != nil {
		return fmt.Errorf("grpc compact: %w", err)
	}
	if !compact.OK {
		return fmt.Errorf("grpc compact ok=false")
	}
	return nil
}

func invokeGRPC(ctx context.Context, conn *grpc.ClientConn, method string, in any, out any) error {
	return conn.Invoke(ctx, "/"+grpcServiceName+"/"+method, in, out, grpc.ForceCodec(jsonCodec{}))
}

func testPoint(measurement string, host string, timestamp int64, usage float64) mts.Point {
	return mts.Point{
		Measurement: measurement,
		Tags:        map[string]string{"host": host},
		Timestamp:   timestamp,
		Fields: map[string]mts.FieldValue{
			"usage": mts.Float64Value(usage),
		},
	}
}

func testQuery(measurement string) (mts.Query, error) {
	query, err := mts.NewQuery().From("", "", measurement).Select("usage").Build()
	if err != nil {
		return mts.Query{}, fmt.Errorf("build query: %w", err)
	}
	return query, nil
}

func testBatch(measurement string, host string, timestamp int64, usage float64) mts.TypedBatch {
	return mts.TypedBatch{
		Measurement: measurement,
		Tags:        []mts.TagColumn{{Name: "host", Values: []string{host}}},
		Timestamps:  []int64{timestamp},
		Fields: []mts.TypedFieldColumn{{
			Name:          "usage",
			Type:          mts.FieldFloat64,
			Float64Values: []float64{usage},
		}},
	}
}

func testDownsamplePolicy(name string, measurement string) mts.DownsamplePolicy {
	return mts.DownsamplePolicy{
		Name:              name,
		SourceDatabase:    "default",
		SourceRetention:   "autogen",
		SourceMeasurement: measurement,
		TargetDatabase:    "default",
		TargetRetention:   "autogen",
		TargetMeasurement: measurement + "_1h",
		Interval:          time.Hour,
		RefreshInterval:   time.Hour,
		Lookback:          time.Hour,
		BatchSize:         100,
		Functions: []mts.DownsampleFunction{{
			Function: mts.AggregateAvg,
			Field:    "usage",
			As:       "usage_avg",
		}},
		GroupByTags: []string{"host"},
		Enabled:     true,
	}
}

func assertRows(rows []mts.Row, measurement string, host string, usage float64) error {
	if len(rows) != 1 {
		return fmt.Errorf("rows len=%d, want 1 rows=%#v", len(rows), rows)
	}
	row := rows[0]
	if row.Measurement != measurement || row.Tags["host"] != host {
		return fmt.Errorf("row identity=%#v, want measurement=%s host=%s", row, measurement, host)
	}
	field, ok := row.Fields["usage"]
	if !ok || field.Float64 != usage {
		return fmt.Errorf("usage field=%#v exists=%v, want %v", field, ok, usage)
	}
	return nil
}

type jsonCodec struct{}

func (jsonCodec) Name() string { return "json" }

func (jsonCodec) Marshal(value any) ([]byte, error) { return json.Marshal(value) }

func (jsonCodec) Unmarshal(data []byte, value any) error { return json.Unmarshal(data, value) }

type emptyRequest struct{}

type writeRequest struct {
	Points  []mts.Point      `json:"points"`
	Options mts.WriteOptions `json:"options"`
}

type writeResponse struct {
	OK bool `json:"ok"`
}

type queryRowsRequest struct {
	Query mts.Query `json:"query"`
}

type queryRequest struct {
	Query mts.Query `json:"query"`
}

type queryRowsResponse struct {
	Rows []mts.Row `json:"rows"`
}

type queryColumnsResponse struct {
	Columns []mts.ColumnSeries `json:"columns"`
}

type queryExplainResponse struct {
	Result mts.QueryResult `json:"result"`
}

type typedWriteRequest struct {
	Batch   mts.TypedBatch   `json:"batch"`
	Options mts.WriteOptions `json:"options"`
}

type okResponse struct {
	OK bool `json:"ok"`
}

type userResponse struct {
	User mts.User `json:"user"`
}

type configResponse struct {
	Config struct {
		DataDir string `json:"DataDir" yaml:"data_dir"`
	} `json:"config"`
}

type apiSpecResponse struct {
	Version string `json:"version"`
}

type storageValidateResponse struct {
	OK bool `json:"ok"`
}

type databasePermissionRequest struct {
	UserName   string                 `json:"user_name"`
	Database   string                 `json:"database"`
	Permission mts.DatabasePermission `json:"permission"`
}

type downsamplePoliciesResponse struct {
	Policies []mts.DownsamplePolicy `json:"policies"`
}

type downsampleRangeRequest struct {
	StartUnix int64 `json:"start_unix"`
	EndUnix   int64 `json:"end_unix"`
}

type downsamplePolicyRangeRequest struct {
	Name      string `json:"name"`
	StartUnix int64  `json:"start_unix"`
	EndUnix   int64  `json:"end_unix"`
}

type downsampleDryRunResponse struct {
	Result mts.DownsampleDryRunResult `json:"result"`
}

type maintenanceResponse struct {
	OK     bool                 `json:"ok"`
	Result mts.CompactionResult `json:"result,omitempty"`
}
