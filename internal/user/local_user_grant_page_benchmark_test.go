package user

import (
	"context"
	"fmt"
	"testing"
)

var benchmarkGrantCount int

func BenchmarkUserGrantRead(b *testing.B) {
	for _, userCount := range []int{10, 100, 1000} {
		b.Run(fmt.Sprintf("legacy-users=%d", userCount), func(b *testing.B) {
			manager := benchmarkUserManager(userCount)
			ctx := context.Background()
			b.ReportAllocs()

			for b.Loop() {
				users, err := manager.ListUsers(ctx)
				if err != nil {
					b.Fatalf("ListUsers() error = %v", err)
				}
				grantCount := 0
				for _, currentUser := range users {
					grants, listErr := manager.ListPermissions(ctx, currentUser.Name)
					if listErr != nil {
						b.Fatalf("ListPermissions(%q) error = %v", currentUser.Name, listErr)
					}
					grantCount += len(grants)
				}
				benchmarkGrantCount = grantCount
			}
			b.ReportMetric(float64(userCount+1), "requests/op")
		})
		b.Run(fmt.Sprintf("page-users=%d", userCount), func(b *testing.B) {
			manager := benchmarkUserManager(userCount)
			ctx := context.Background()
			b.ReportAllocs()

			for b.Loop() {
				page, err := manager.ListUserGrantPage(ctx, "", 100)
				if err != nil {
					b.Fatalf("ListUserGrantPage() error = %v", err)
				}
				benchmarkGrantCount = len(page.Items)
			}
			b.ReportMetric(1, "requests/op")
			b.ReportMetric(float64(min(userCount, 100)), "users/op")
		})
	}
}

func benchmarkUserManager(userCount int) *Manager {
	store := newLocalStateStore("")
	for index := range userCount {
		name := fmt.Sprintf("user-%04d", index)
		store.users[name] = User{Name: name, Role: RoleUser}
		store.grants[name] = map[string]map[Permission]struct{}{
			"metrics": {
				PermissionRead:  {},
				PermissionWrite: {},
				PermissionAdmin: {},
			},
		}
	}
	return &Manager{store: store, opts: DefaultOptions()}
}
