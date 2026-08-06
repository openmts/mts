package mts_test

import (
	"context"
	"testing"

	mts "github.com/openmts/mts"
)

func TestEngineListUserGrantPageConvertsStablePage(t *testing.T) {
	ctx := context.Background()
	engine, err := mts.Open(ctx, mts.DefaultOptions(t.TempDir()))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer closeEngine(t, engine)

	if err := engine.CreateUser(ctx, mts.User{
		Name:     "alice",
		Role:     mts.UserRoleAdmin,
		Metadata: map[string]string{"team": "platform"},
	}); err != nil {
		t.Fatalf("CreateUser(alice) error = %v", err)
	}
	if err := engine.CreateUser(ctx, mts.User{Name: "bob"}); err != nil {
		t.Fatalf("CreateUser(bob) error = %v", err)
	}
	if err := engine.GrantDatabasePermission(ctx, "alice", "metrics", mts.DatabasePermissionRead); err != nil {
		t.Fatalf("GrantDatabasePermission() error = %v", err)
	}

	page, err := engine.ListUserGrantPage(ctx, "", 1)
	if err != nil {
		t.Fatalf("ListUserGrantPage() error = %v", err)
	}
	if page.TotalUsers != 2 || page.NextCursor != "alice" || len(page.Items) != 1 {
		t.Fatalf("page = %+v, want one of two and next alice", page)
	}
	item := page.Items[0]
	if item.User.Name != "alice" || item.User.Role != mts.UserRoleAdmin {
		t.Fatalf("user = %+v, want admin alice", item.User)
	}
	if len(item.Grants) != 1 || item.Grants[0].Database != "metrics" || item.Grants[0].Permission != mts.DatabasePermissionRead {
		t.Fatalf("grants = %#v, want metrics/read", item.Grants)
	}

	page.Items[0].User.Metadata["team"] = "mutated"
	again, err := engine.ListUserGrantPage(ctx, "", 1)
	if err != nil {
		t.Fatalf("ListUserGrantPage(after mutation) error = %v", err)
	}
	if again.Items[0].User.Metadata["team"] != "platform" {
		t.Fatalf("metadata = %q, want platform", again.Items[0].User.Metadata["team"])
	}
}
