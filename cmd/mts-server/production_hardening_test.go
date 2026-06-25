package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	mts "github.com/openmts/mts"
)

func TestProductionConfigTLSAndCLI(t *testing.T) {
	certFile, keyFile := writeSelfSignedCert(t)
	path := writeTestConfig(t, `data_dir: `+filepath.ToSlash(t.TempDir())+`
http:
  enabled: true
  addr: 127.0.0.1:0
  tls:
    enabled: true
    cert_file: `+filepath.ToSlash(certFile)+`
    key_file: `+filepath.ToSlash(keyFile)+`
grpc:
  enabled: false
auth:
  data_tokens: [data-token]
  require_user: true
limits:
  max_request_body_bytes: 1024
  max_write_points: 1
  default_query_limit: 5
  max_query_limit: 10
  request_timeout: 2s
observability:
  access_log: true
  pprof:
    enabled: true
backup:
  dir: `+filepath.ToSlash(filepath.Join(t.TempDir(), "backups"))+`
log:
  level: debug
  format: json
shutdown_timeout: 3s
`)
	cfg, err := loadConfig(path)
	if err != nil {
		t.Fatalf("loadConfig() error = %v", err)
	}
	if !cfg.HTTP.TLS.Enabled || !cfg.Auth.RequireUser || len(cfg.Auth.DataTokens) != 1 {
		t.Fatalf("config = %#v, want production fields", cfg)
	}
	if tlsCfg, err := buildTLSConfig(cfg.HTTP.TLS); err != nil || tlsCfg.MinVersion < 0x0303 {
		t.Fatalf("buildTLSConfig() = %#v %v, want tls1.2+", tlsCfg, err)
	}

	var stdout bytes.Buffer
	app := newApp(&stdout, &bytes.Buffer{})
	if err := app.RunContext(context.Background(), []string{"mts-server", "validate-config", "--config", path}); err != nil {
		t.Fatalf("validate-config error = %v", err)
	}
	if !strings.Contains(stdout.String(), "config ok") {
		t.Fatalf("validate output = %s", stdout.String())
	}

	stdout.Reset()
	if err := app.RunContext(context.Background(), []string{"mts-server", "doctor", "--config", path}); err != nil {
		t.Fatalf("doctor error = %v", err)
	}
	if !strings.Contains(stdout.String(), "doctor ok") {
		t.Fatalf("doctor output = %s", stdout.String())
	}

	outPath := filepath.Join(t.TempDir(), "generated", "mts-server.yaml")
	stdout.Reset()
	if err := app.RunContext(context.Background(), []string{"mts-server", "init-config", "--output", outPath}); err != nil {
		t.Fatalf("init-config error = %v", err)
	}
	info, err := os.Stat(outPath)
	if err != nil {
		t.Fatalf("Stat(init config) error = %v", err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("init config mode = %o, want 0600", info.Mode().Perm())
	}
	if err := app.RunContext(context.Background(), []string{"mts-server", "init-config", "--output", outPath}); !errorsIsInvalidConfig(err) {
		t.Fatalf("init-config overwrite error = %v, want errInvalidConfig", err)
	}
	if err := app.RunContext(context.Background(), []string{"mts-server", "init-config", "--output", outPath, "--force"}); err != nil {
		t.Fatalf("init-config force error = %v", err)
	}
	stdout.Reset()
	if err := app.RunContext(context.Background(), []string{"mts-server", "version"}); err != nil {
		t.Fatalf("version error = %v", err)
	}
	if !strings.Contains(stdout.String(), "mts-server") {
		t.Fatalf("version output = %s", stdout.String())
	}
}

func TestHTTPProductionAdminEndpointBranches(t *testing.T) {
	runtime := openTestRuntimeWithAdminToken(t)
	runtime.config.ConfigPath = writeRuntimeConfig(t, runtime.currentConfig())
	server := httptest.NewServer(runtime.httpHandler())
	defer server.Close()
	adminHeaders := map[string]string{"Authorization": "Bearer test-admin-token"}
	paths := []string{
		"/api/v1/admin/api-spec",
		"/api/v1/admin/error-codes",
		"/api/v1/admin/storage/export",
	}
	for _, path := range paths {
		t.Run("unauthenticated"+path, func(t *testing.T) {
			getJSONWithHeaders(t, server.URL+path, nil, http.StatusUnauthorized, &errorResponse{})
		})
	}
	postPaths := []string{
		"/api/v1/admin/config/reload",
		"/api/v1/admin/storage/validate",
		"/api/v1/admin/storage/snapshot",
	}
	for _, path := range postPaths {
		t.Run("unauthenticated"+path, func(t *testing.T) {
			postJSON(t, server.URL+path, emptyRequest{}, http.StatusUnauthorized, &errorResponse{})
		})
	}
	for _, path := range postPaths {
		t.Run("method"+path, func(t *testing.T) {
			getJSONWithHeaders(t, server.URL+path, adminHeaders, http.StatusMethodNotAllowed, &errorResponse{})
		})
	}
	resp := doHTTP(t, http.MethodPost, server.URL+"/api/v1/admin/error-codes", emptyRequest{}, adminHeaders)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("post error-codes status = %d, want 405", resp.StatusCode)
	}
	resp = doHTTP(t, http.MethodPost, server.URL+"/api/v1/users/missing/audit", emptyRequest{}, adminHeaders)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("post user audit status = %d, want 405", resp.StatusCode)
	}
}

func TestHTTPProductionHardening(t *testing.T) {
	runtime := openTestRuntime(t)
	runtime.config.Auth.AdminToken = "admin-token"
	runtime.config.Auth.DataTokens = []string{"data-token"}
	runtime.config.Auth.RequireUser = true
	runtime.config.Limits.MaxWritePoints = 1
	runtime.config.Limits.MaxQueryLimit = 1
	runtime.config.Limits.DefaultQueryLimit = 1
	runtime.config.Backup.Dir = filepath.Join(t.TempDir(), "backups")
	runtime.config.Observability.Pprof.Enabled = true
	runtime.config.ConfigPath = writeRuntimeConfig(t, runtime.currentConfig())
	server := httptest.NewServer(runtime.httpHandler())
	defer server.Close()

	adminHeaders := map[string]string{"Authorization": "Bearer admin-token"}
	postJSONWithHeaders(t, server.URL+"/api/v1/users", mts.User{Name: "prod"}, adminHeaders, http.StatusOK, &okResponse{})
	postJSONWithHeaders(t, server.URL+"/api/v1/users/prod/database-permissions/default/write", emptyRequest{}, adminHeaders, http.StatusOK, &okResponse{})
	postJSONWithHeaders(t, server.URL+"/api/v1/users/prod/database-permissions/default/read", emptyRequest{}, adminHeaders, http.StatusOK, &okResponse{})

	point := testPoint()
	postJSON(t, server.URL+"/api/v1/data/write", writeRequest{Points: []mts.Point{point}}, http.StatusUnauthorized, &errorResponse{})
	dataHeaders := map[string]string{"X-MTS-Data-Token": "data-token", "X-MTS-User": "prod"}
	postJSONWithHeaders(t, server.URL+"/api/v1/data/write", writeRequest{Points: []mts.Point{point, point}}, dataHeaders, http.StatusBadRequest, &errorResponse{})
	postJSONWithHeaders(t, server.URL+"/api/v1/data/write", writeRequest{Points: []mts.Point{point}}, dataHeaders, http.StatusOK, &writeResponse{})
	putJSON(t, server.URL+"/api/v1/users/prod", mts.User{Name: "prod", Disabled: true}, http.StatusUnauthorized, &errorResponse{})
	putJSONWithHeaders(t, server.URL+"/api/v1/users/prod", mts.User{Name: "prod", Disabled: true}, adminHeaders, http.StatusOK, &okResponse{})
	postJSONWithHeaders(t, server.URL+"/api/v1/data/write", writeRequest{Points: []mts.Point{point}}, dataHeaders, http.StatusForbidden, &errorResponse{})
	putJSONWithHeaders(t, server.URL+"/api/v1/users/prod", mts.User{Name: "prod"}, adminHeaders, http.StatusOK, &okResponse{})
	query := testQuery()
	query.Limit = 2
	postJSONWithHeaders(t, server.URL+"/api/v1/data/query/rows", queryRequest{Query: query}, dataHeaders, http.StatusBadRequest, &errorResponse{})

	var spec apiSpecResponse
	getJSONWithHeaders(t, server.URL+"/api/v1/admin/api-spec", adminHeaders, http.StatusOK, &spec)
	if spec.Version != "v1" || len(spec.Namespaces) == 0 {
		t.Fatalf("api spec = %#v", spec)
	}
	var codesResp errorCodesResponse
	getJSONWithHeaders(t, server.URL+"/api/v1/admin/error-codes", adminHeaders, http.StatusOK, &codesResp)
	if len(codesResp.Codes) == 0 {
		t.Fatal("error codes empty")
	}
	var audit userAuditResponse
	getJSONWithHeaders(t, server.URL+"/api/v1/users/prod/audit", adminHeaders, http.StatusOK, &audit)
	if len(audit.Events) < 3 {
		t.Fatalf("audit events = %#v, want create/grants", audit.Events)
	}
	var validate storageValidateResponse
	postJSONWithHeaders(t, server.URL+"/api/v1/admin/storage/validate", emptyRequest{}, adminHeaders, http.StatusOK, &validate)
	if !validate.OK {
		t.Fatalf("storage validate = %#v", validate)
	}
	var snapshot storageSnapshotResponse
	postJSONWithHeaders(t, server.URL+"/api/v1/admin/storage/snapshot", emptyRequest{}, adminHeaders, http.StatusOK, &snapshot)
	if snapshot.Path == "" {
		t.Fatalf("snapshot = %#v", snapshot)
	}
	info, err := os.Stat(snapshot.Path)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("snapshot stat = %#v %v, want mode 0600", info, err)
	}
	var exportResp storageExportResponse
	getJSONWithHeaders(t, server.URL+"/api/v1/admin/storage/export", adminHeaders, http.StatusOK, &exportResp)
	if len(exportResp.Export.Users) == 0 {
		t.Fatalf("export = %#v, want users", exportResp.Export)
	}
	resp, err := http.Get(server.URL + "/metrics")
	if err != nil {
		t.Fatalf("Get(metrics) error = %v", err)
	}
	body, closeBody := readResponseBody(t, resp)
	defer closeBody()
	if !strings.Contains(body, "mts_server_requests_total") {
		t.Fatalf("metrics body = %s", body)
	}
	var cfgValidate configValidateResponse
	postJSONWithHeaders(t, server.URL+"/api/v1/admin/config/validate", configValidateRequest{Config: runtime.currentConfig()}, adminHeaders, http.StatusOK, &cfgValidate)
	if !cfgValidate.OK {
		t.Fatalf("config validate = %#v", cfgValidate)
	}
	postJSONWithHeaders(t, server.URL+"/api/v1/admin/config/validate", configValidateRequest{}, adminHeaders, http.StatusBadRequest, &configValidateResponse{})
	var reload reloadConfigResponse
	postJSONWithHeaders(t, server.URL+"/api/v1/admin/config/reload", emptyRequest{}, adminHeaders, http.StatusOK, &reload)
	if !reload.OK || len(reload.Fields) == 0 {
		t.Fatalf("reload = %#v", reload)
	}
	pprofResp, err := http.Get(server.URL + "/debug/pprof/")
	if err != nil {
		t.Fatalf("Get(pprof) error = %v", err)
	}
	_ = pprofResp.Body.Close()
	if pprofResp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("pprof status = %d, want 401", pprofResp.StatusCode)
	}
	pprofReq, err := http.NewRequest(http.MethodGet, server.URL+"/debug/pprof/", nil)
	if err != nil {
		t.Fatalf("NewRequest(pprof) error = %v", err)
	}
	pprofReq.Header.Set("Authorization", "Bearer admin-token")
	pprofResp, err = http.DefaultClient.Do(pprofReq)
	if err != nil {
		t.Fatalf("Do(pprof) error = %v", err)
	}
	_ = pprofResp.Body.Close()
	if pprofResp.StatusCode != http.StatusOK {
		t.Fatalf("pprof authed status = %d, want 200", pprofResp.StatusCode)
	}
}

func TestGRPCProductionHardening(t *testing.T) {
	runtime := openTestRuntime(t)
	runtime.config.Auth.AdminToken = "admin-token"
	runtime.config.Auth.DataTokens = []string{"data-token"}
	runtime.config.Auth.RequireUser = true
	runtime.config.Backup.Dir = filepath.Join(t.TempDir(), "backups")
	runtime.config.ConfigPath = writeRuntimeConfig(t, runtime.currentConfig())
	conn := openBufconnClient(t, runtime)
	defer func() { _ = conn.Close() }()
	adminCtx := metadata.NewOutgoingContext(context.Background(), metadata.Pairs("authorization", "Bearer admin-token"))
	dataCtx := metadata.NewOutgoingContext(context.Background(), metadata.Pairs(
		"x-mts-data-token", "data-token",
		"x-mts-user", "grpc-prod",
	))
	invokeOK(t, adminCtx, conn, "CreateUser", &mts.User{Name: "grpc-prod"}, &okResponse{})
	invokeOK(t, adminCtx, conn, "GrantDatabasePermission", &databasePermissionRequest{UserName: "grpc-prod", Database: "default", Permission: mts.DatabasePermissionWrite}, &okResponse{})
	invokeOK(t, adminCtx, conn, "GrantDatabasePermission", &databasePermissionRequest{UserName: "grpc-prod", Database: "default", Permission: mts.DatabasePermissionRead}, &okResponse{})
	invokeOK(t, dataCtx, conn, "Write", &writeRequest{Points: []mts.Point{testPoint()}}, &writeResponse{})
	var spec apiSpecResponse
	invokeOK(t, adminCtx, conn, "GetAPISpec", &emptyRequest{}, &spec)
	if spec.Version != "v1" {
		t.Fatalf("grpc spec = %#v", spec)
	}
	err := invokeGRPC(context.Background(), conn, "QueryRows", &queryRequest{Query: testQuery()}, &queryRowsResponse{})
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("QueryRows without token code = %v, want unauthenticated", status.Code(err))
	}
	var codesResp errorCodesResponse
	invokeOK(t, adminCtx, conn, "GetErrorCodes", &emptyRequest{}, &codesResp)
	if len(codesResp.Codes) == 0 {
		t.Fatal("grpc error codes empty")
	}
	var cfgOK configValidateResponse
	invokeOK(t, adminCtx, conn, "ValidateConfig", &configValidateRequest{Config: runtime.currentConfig()}, &cfgOK)
	if !cfgOK.OK {
		t.Fatalf("grpc validate config = %#v", cfgOK)
	}
	invokeOK(t, adminCtx, conn, "ReloadConfig", &emptyRequest{}, &reloadConfigResponse{})
	invokeOK(t, adminCtx, conn, "StorageValidate", &emptyRequest{}, &storageValidateResponse{})
	invokeOK(t, adminCtx, conn, "StorageSnapshot", &emptyRequest{}, &storageSnapshotResponse{})
	invokeOK(t, adminCtx, conn, "StorageExport", &emptyRequest{}, &storageExportResponse{})
}

func TestGRPCProductionAdminUnauthenticatedBranches(t *testing.T) {
	runtime := openTestRuntimeWithAdminToken(t)
	runtime.config.ConfigPath = writeRuntimeConfig(t, runtime.currentConfig())
	conn := openBufconnClient(t, runtime)
	defer func() { _ = conn.Close() }()
	ctx := context.Background()
	cases := []struct {
		name string
		req  any
	}{
		{name: "GetAPISpec", req: &emptyRequest{}},
		{name: "GetErrorCodes", req: &emptyRequest{}},
		{name: "ValidateConfig", req: &configValidateRequest{}},
		{name: "ReloadConfig", req: &emptyRequest{}},
		{name: "StorageValidate", req: &emptyRequest{}},
		{name: "StorageSnapshot", req: &emptyRequest{}},
		{name: "StorageExport", req: &emptyRequest{}},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			err := invokeGRPC(ctx, conn, tt.name, tt.req, &okResponse{})
			if status.Code(err) != codes.Unauthenticated {
				t.Fatalf("%s code = %v, want unauthenticated", tt.name, status.Code(err))
			}
		})
	}
}

func TestProductionHelperBranches(t *testing.T) {
	runtime := openTestRuntime(t)
	var marker grpcServiceServer = &grpcService{}
	marker.mtsServer()
	if requestIDFromContext(context.Background()) != "" {
		t.Fatal("request id from empty context should be empty")
	}
	if httpRoute("") != "/" {
		t.Fatal("empty route should be slash")
	}
	if grpcStatusCodeText(nil) != "0" {
		t.Fatal("nil grpc status text should be 0")
	}
	if statusText := grpcStatusCodeText(status.Error(codes.Internal, "x")); statusText != codes.Internal.String() {
		t.Fatalf("grpc status text = %s, want internal", statusText)
	}
	if build, err := buildTLSConfig(tlsConfig{Enabled: true, CertFile: "missing", KeyFile: "missing"}); err == nil || build != nil {
		t.Fatalf("build bad tls = %#v %v, want error", build, err)
	}
	caFile := filepath.Join(t.TempDir(), "ca.pem")
	if err := os.WriteFile(caFile, []byte("bad"), 0600); err != nil {
		t.Fatalf("WriteFile(ca) error = %v", err)
	}
	if _, err := loadClientCAPool(caFile); err == nil {
		t.Fatal("load bad ca error = nil")
	}
	certFile, keyFile := writeSelfSignedCert(t)
	if tlsCfg, err := buildTLSConfig(tlsConfig{Enabled: true, CertFile: certFile, KeyFile: keyFile, ClientCAFile: certFile, ClientAuth: true}); err != nil || tlsCfg.ClientAuth == 0 {
		t.Fatalf("build mtls = %#v %v, want client auth", tlsCfg, err)
	}
	invalidConfigs := []config{
		{DataDir: "x", HTTP: httpConfig{Enabled: true, Addr: "x"}, GRPC: grpcConfig{}, Shutdown: durationText(time.Second), Limits: limitsConfig{DefaultQueryLimit: 2, MaxQueryLimit: 1}, Engine: defaultConfig().Engine, Log: defaultConfig().Log},
		{DataDir: "x", HTTP: httpConfig{Enabled: true, Addr: "x", TLS: tlsConfig{Enabled: true}}, Shutdown: durationText(time.Second), Engine: defaultConfig().Engine, Log: defaultConfig().Log},
		{DataDir: "x", HTTP: httpConfig{Enabled: true, Addr: "x"}, Shutdown: durationText(time.Second), Engine: defaultConfig().Engine, Log: logConfig{Level: "info", Format: "xml"}},
	}
	for _, cfg := range invalidConfigs {
		if err := cfg.validate(); err == nil {
			t.Fatalf("validate(%#v) error = nil", cfg)
		}
	}
	logger := newLogger(&bytes.Buffer{}, logConfig{Level: "warn", Format: "json"})
	runtime.setLogger(logger)
	if runtime.currentLogger() == nil {
		t.Fatal("logger nil")
	}
	runtime.setLogger(nil)
	_ = newLogger(&bytes.Buffer{}, logConfig{Level: "error", Format: "text"})
	_ = newLogger(&bytes.Buffer{}, logConfig{Level: "unknown", Format: "text"})
	runtime.applyLimitState(config{})
	if runtime.httpSem != nil || runtime.grpcSem != nil {
		t.Fatalf("zero limits semaphores = %#v %#v, want nil", runtime.httpSem, runtime.grpcSem)
	}
	runtime.applyLimitState(config{Limits: limitsConfig{MaxConcurrentHTTP: 1, MaxConcurrentGRPC: 1}})
	recorder := httptest.NewRecorder()
	httpSem := runtime.httpSem
	if !runtime.acquireHTTP(recorder, httpSem) {
		t.Fatal("first acquire http failed")
	}
	if runtime.acquireHTTP(recorder, httpSem) {
		t.Fatal("second acquire http succeeded, want reject")
	}
	releaseHTTP(httpSem)
	grpcSem := runtime.grpcSem
	if !acquireGRPC(grpcSem) {
		t.Fatal("first acquire grpc failed")
	}
	if acquireGRPC(grpcSem) {
		t.Fatal("second acquire grpc succeeded, want reject")
	}
	releaseGRPC(grpcSem)
	audit := newAuditLog(1)
	audit.record(auditEvent{UserName: "a", Action: "one"})
	audit.record(auditEvent{UserName: "a", Action: "two"})
	if events := audit.list("a"); len(events) != 1 || events[0].Action != "two" {
		t.Fatalf("audit events = %#v, want only latest", events)
	}
	defaultAudit := newAuditLog(0)
	defaultAudit.record(auditEvent{UserName: "b", Action: "created"})
	if events := defaultAudit.list("b"); len(events) != 1 {
		t.Fatalf("default audit events = %#v, want one", events)
	}
	if events := defaultAudit.list("missing"); len(events) != 0 {
		t.Fatalf("missing audit events = %#v, want empty", events)
	}
	badRuntime := openTestRuntime(t)
	badRuntime.config.Backup.Dir = filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(badRuntime.config.Backup.Dir, []byte("x"), 0600); err != nil {
		t.Fatalf("WriteFile(backup file) error = %v", err)
	}
	if _, err := badRuntime.storageSnapshot(context.Background()); err == nil {
		t.Fatal("storageSnapshot with file backup dir error = nil")
	}
	if _, err := badRuntime.storageSnapshot(canceledContext()); err == nil {
		t.Fatal("storageSnapshot canceled error = nil")
	}
	badGRPC := defaultConfig()
	badGRPC.DataDir = t.TempDir()
	badGRPC.HTTP.Enabled = false
	badGRPC.GRPC.Enabled = true
	badGRPC.GRPC.TLS.Enabled = true
	badGRPC.GRPC.TLS.CertFile = "missing"
	badGRPC.GRPC.TLS.KeyFile = "missing"
	if runtime, err := openRuntime(context.Background(), badGRPC); err == nil {
		_ = runtime.shutdown(context.Background())
		t.Fatal("openRuntime with bad grpc tls error = nil")
	}
}

func TestProductionCLIErrorBranches(t *testing.T) {
	var stdout bytes.Buffer
	app := newApp(&stdout, &bytes.Buffer{})
	missingPath := filepath.Join(t.TempDir(), "missing.yaml")
	if err := app.RunContext(context.Background(), []string{"mts-server", "validate-config", "--config", missingPath}); err == nil {
		t.Fatal("validate-config missing file error = nil")
	}
	if err := app.RunContext(context.Background(), []string{"mts-server", "doctor", "--config", missingPath}); err == nil {
		t.Fatal("doctor missing file error = nil")
	}
	outFile := filepath.Join(t.TempDir(), "as-file")
	if err := os.WriteFile(outFile, []byte("x"), 0600); err != nil {
		t.Fatalf("WriteFile(output parent) error = %v", err)
	}
	if err := app.RunContext(context.Background(), []string{"mts-server", "init-config", "--output", filepath.Join(outFile, "mts-server.yaml"), "--force"}); err == nil {
		t.Fatal("init-config with file parent error = nil")
	}
	stdout.Reset()
	if err := app.RunContext(context.Background(), []string{"mts-server", "serve", "--print-config"}); err != nil {
		t.Fatalf("serve --print-config error = %v", err)
	}
	if !strings.Contains(stdout.String(), "data_dir") {
		t.Fatalf("print-config output = %s", stdout.String())
	}
}

func writeSelfSignedCert(t *testing.T) (string, string) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("GenerateKey error = %v", err)
	}
	tmpl := x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "localhost"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), DNSNames: []string{"localhost"}, IPAddresses: []net.IP{net.ParseIP("127.0.0.1")}}
	der, err := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("CreateCertificate error = %v", err)
	}
	dir := t.TempDir()
	certFile := filepath.Join(dir, "cert.pem")
	keyFile := filepath.Join(dir, "key.pem")
	certOut := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyOut := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	if err := os.WriteFile(certFile, certOut, 0600); err != nil {
		t.Fatalf("WriteFile(cert) error = %v", err)
	}
	if err := os.WriteFile(keyFile, keyOut, 0600); err != nil {
		t.Fatalf("WriteFile(key) error = %v", err)
	}
	return certFile, keyFile
}

func writeRuntimeConfig(t *testing.T, cfg config) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "mts-server.yaml")
	body := `data_dir: ` + filepath.ToSlash(cfg.DataDir) + `
http:
  enabled: true
  addr: 127.0.0.1:0
grpc:
  enabled: true
  addr: 127.0.0.1:0
auth:
  admin_token: ` + cfg.Auth.AdminToken + `
  data_tokens: [` + strings.Join(cfg.Auth.DataTokens, ",") + `]
  require_user: ` + fmt.Sprintf("%t", cfg.Auth.RequireUser) + `
limits:
  max_request_body_bytes: 16777216
  max_write_points: 10000
  default_query_limit: 10000
  max_query_limit: 100000
  request_timeout: 30s
  max_concurrent_http: 1024
  max_concurrent_grpc: 1024
observability:
  access_log: false
  pprof:
    enabled: true
backup:
  dir: ` + filepath.ToSlash(cfg.Backup.Dir) + `
log:
  level: info
  format: text
engine:
  default_database: default
  default_retention_policy: autogen
  shard_duration: 1h
  retention: 0s
  memtable_max_samples: 10000
  compaction:
    enabled: true
    background_interval: 0s
    level0_part_limit: 4
    max_cascade_steps: 8
shutdown_timeout: 15s
`
	if err := os.WriteFile(path, []byte(body), 0600); err != nil {
		t.Fatalf("WriteFile(runtime config) error = %v", err)
	}
	return path
}

func errorsIsInvalidConfig(err error) bool {
	return err != nil && strings.Contains(err.Error(), errInvalidConfig.Error())
}

func putJSONWithHeaders(t *testing.T, url string, req any, headers map[string]string, wantStatus int, out any) {
	t.Helper()
	body, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Marshal(put) error = %v", err)
	}
	request, err := http.NewRequest(http.MethodPut, url, bytes.NewReader(body))
	if err != nil {
		t.Fatalf("NewRequest(put) error = %v", err)
	}
	request.Header.Set("Content-Type", "application/json")
	for key, value := range headers {
		request.Header.Set(key, value)
	}
	resp, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatalf("Put(%s) error = %v", url, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != wantStatus {
		t.Fatalf("Put(%s) status = %d, want %d", url, resp.StatusCode, wantStatus)
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		t.Fatalf("Decode(put response) error = %v", err)
	}
}
