package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	mts "github.com/openmts/mts"
)

const benchmarkAdminToken = "benchmark-admin-token"

func BenchmarkAccessGrantsHTTP(b *testing.B) {
	for _, userCount := range []int{10, 100, 1000} {
		b.Run(fmt.Sprintf("users=%d", userCount), func(b *testing.B) {
			runtime := openAccessGrantsBenchmarkRuntime(b, userCount)
			handler := runtime.httpHandler()
			request := httptest.NewRequest(http.MethodGet, routeUsersAccessGrants, nil)
			request.Header.Set(headerAdminToken, benchmarkAdminToken)

			responseBytes := measureAccessGrantsResponseBytes(b, handler, request)
			b.ReportAllocs()

			for b.Loop() {
				recorder := httptest.NewRecorder()
				handler.ServeHTTP(recorder, request)
				if recorder.Code != http.StatusOK {
					b.Fatalf("GET %s status = %d, want %d", routeUsersAccessGrants, recorder.Code, http.StatusOK)
				}
			}
			b.ReportMetric(float64(responseBytes), "response-B/op")
			b.ReportMetric(float64(min(userCount, defaultAccessGrantsPageLimit)), "users/op")
		})
	}
}

func openAccessGrantsBenchmarkRuntime(b *testing.B, userCount int) *serverRuntime {
	b.Helper()
	cfg := defaultConfig()
	cfg.DataDir = b.TempDir()
	cfg.HTTP.Addr = "127.0.0.1:0"
	cfg.GRPC.Enabled = false
	cfg.Auth.AdminToken = benchmarkAdminToken
	cfg.Observability.AccessLog = false
	runtime, err := openRuntime(context.Background(), cfg)
	if err != nil {
		b.Fatalf("openRuntime() error = %v", err)
	}
	b.Cleanup(func() {
		if shutdownErr := runtime.shutdown(context.Background()); shutdownErr != nil {
			b.Errorf("shutdown runtime error = %v", shutdownErr)
		}
	})

	for index := range userCount {
		role := mts.UserRoleUser
		if index == 0 {
			role = mts.UserRoleAdmin
		}
		userName := fmt.Sprintf("user-%04d", index)
		if createErr := runtime.engine.CreateUser(
			context.Background(),
			mts.User{Name: userName, Role: role},
		); createErr != nil {
			b.Fatalf("CreateUser(%q) error = %v", userName, createErr)
		}
		for _, permission := range []mts.DatabasePermission{
			mts.DatabasePermissionRead,
			mts.DatabasePermissionWrite,
			mts.DatabasePermissionAdmin,
		} {
			if grantErr := runtime.engine.GrantDatabasePermission(
				context.Background(),
				userName,
				"metrics",
				permission,
			); grantErr != nil {
				b.Fatalf("GrantDatabasePermission(%q, %q) error = %v", userName, permission, grantErr)
			}
		}
	}
	return runtime
}

func measureAccessGrantsResponseBytes(b *testing.B, handler http.Handler, request *http.Request) int {
	b.Helper()
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		b.Fatalf("GET %s status = %d, want %d", routeUsersAccessGrants, recorder.Code, http.StatusOK)
	}
	return recorder.Body.Len()
}
