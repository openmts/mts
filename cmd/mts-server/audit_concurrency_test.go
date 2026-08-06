package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"google.golang.org/grpc/metadata"

	mts "github.com/openmts/mts"
	"github.com/openmts/mts/internal/faultinject"
	"github.com/openmts/mts/internal/storagefs"
)

func TestAuditRecordAndCloseConcurrent(t *testing.T) {
	for range 20 {
		audit := newAuditLog(64)
		start := make(chan struct{})
		var wait sync.WaitGroup
		for worker := range 16 {
			wait.Add(1)
			go func() {
				defer wait.Done()
				<-start
				for event := range 100 {
					audit.record(auditEvent{UserName: "user", Action: "concurrent", Detail: auditDetail(worker, event)})
				}
			}()
		}
		close(start)
		go func() { _ = audit.close(context.Background()) }()
		wait.Wait()
		_ = audit.close(context.Background())
	}
}

func TestAuditCloseDrainsAcceptedEvents(t *testing.T) {
	runtime := openTestRuntime(t)
	audit := newAuditLog(256)
	audit.engine = runtime.engine
	for index := range 200 {
		audit.record(auditEvent{
			Time:     time.Unix(0, int64(index+1)).UTC(),
			UserName: "drain-user",
			Action:   "drain",
		})
	}
	if err := audit.close(context.Background()); err != nil {
		t.Fatalf("audit.close() error = %v", err)
	}

	rows, err := runtime.engine.QueryRows(context.Background(), mts.Query{
		Database:    "_internal",
		Measurement: "audit_log",
		Tags:        map[string]string{"user_name": "drain-user"},
		StartTime:   0,
		EndTime:     1000,
		Limit:       1000,
	})
	if err != nil {
		t.Fatalf("QueryRows(audit) error = %v", err)
	}
	if len(rows) != 200 {
		t.Fatalf("persisted audit rows = %d, want 200", len(rows))
	}
}

func auditDetail(worker int, event int) string {
	return strconv.Itoa(worker*100 + event)
}

func TestAuditCloseReportsDropsAndPersistErrors(t *testing.T) {
	persistCtx, cancel := context.WithCancel(context.Background())
	audit := &auditLog{
		limit:      1,
		ch:         make(chan auditEvent, 1),
		done:       make(chan struct{}),
		persistCtx: persistCtx,
		cancel:     cancel,
	}
	audit.record(auditEvent{Action: "accepted"})
	audit.record(auditEvent{Action: "dropped"})
	go audit.persistLoop()
	err := audit.close(context.Background())
	if err == nil || !strings.Contains(err.Error(), "dropped=1") {
		t.Fatalf("audit.close() error = %v, want dropped count", err)
	}

	runtime := openTestRuntime(t)
	failing := newAuditLog(1)
	failing.engine = runtime.engine
	fs := faultinject.NewFS()
	fs.Fail(faultinject.OpWrite, errors.New("persist failure"))
	restore := storagefs.SetFaultController(fs)
	failing.record(auditEvent{Action: "persist_failure"})
	err = failing.close(context.Background())
	restore()
	if err == nil || !strings.Contains(err.Error(), "persist_errors=1") {
		t.Fatalf("failing audit.close() error = %v, want persist error count", err)
	}
}

func TestGRPCAuditMatchesHTTPForLoginAndAdminOperation(t *testing.T) {
	runtime := openTestRuntimeWithAdminToken(t)
	seedUserWithPassword(t, runtime, mts.User{Name: "audit-protocol-user"}, "secret12")
	server := httptest.NewServer(runtime.httpHandler())
	t.Cleanup(server.Close)
	postJSONWithHeaders(t, server.URL+routeAuthLogin, loginRequest{
		UserName: "audit-protocol-user", Password: "secret12",
	}, nil, http.StatusOK, &authTokenResponse{})
	postJSONWithHeaders(t, server.URL+routeAdminDatabases, databaseRequest{Name: "http-audit-db"},
		map[string]string{headerAdminToken: "test-admin-token"}, http.StatusOK, &okResponse{})

	conn := openBufconnClient(t, runtime)
	t.Cleanup(func() {
		if err := conn.Close(); err != nil {
			t.Fatalf("Close(conn) error = %v", err)
		}
	})
	ctx := context.Background()
	invokeOK(t, ctx, conn, "Login", &loginRequest{
		UserName: "audit-protocol-user", Password: "secret12",
	}, &authTokenResponse{})
	adminCtx := metadata.NewOutgoingContext(ctx, metadata.Pairs(metadataAdminToken, "test-admin-token"))
	invokeOK(t, adminCtx, conn, "CreateDatabase", &databaseRequest{Name: "grpc-audit-db"}, &okResponse{})

	events := runtime.audit.listFiltered(auditListRequest{Limit: 100})
	if countAuditAction(events, "login") != 2 {
		t.Fatalf("login audit count = %d, want 2", countAuditAction(events, "login"))
	}
	if !hasAuditDatabase(events, "create_database", "http-audit-db") ||
		!hasAuditDatabase(events, "create_database", "grpc-audit-db") {
		t.Fatalf("create_database audit events = %#v", events)
	}
}

func countAuditAction(events []auditEvent, action string) int {
	count := 0
	for _, event := range events {
		if event.Action == action {
			count++
		}
	}
	return count
}

func hasAuditDatabase(events []auditEvent, action string, database string) bool {
	for _, event := range events {
		if event.Action == action && event.Database == database {
			return true
		}
	}
	return false
}
