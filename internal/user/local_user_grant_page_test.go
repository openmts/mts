package user

import (
	"context"
	"errors"
	"slices"
	"sync"
	"testing"
)

func TestManagerListUserGrantPagePaginatesByUserName(t *testing.T) {
	manager := grantPageTestManager()

	first, err := manager.ListUserGrantPage(context.Background(), "", 2)
	if err != nil {
		t.Fatalf("ListUserGrantPage(first) error = %v", err)
	}
	if first.TotalUsers != 3 || first.NextCursor != "bob" {
		t.Fatalf("first page meta = %+v, want total=3 next=bob", first)
	}
	if got := grantPageUserNames(first.Items); !slices.Equal(got, []string{"alice", "bob"}) {
		t.Fatalf("first page users = %v, want [alice bob]", got)
	}
	if len(first.Items[1].Grants) != 0 {
		t.Fatalf("bob grants = %#v, want empty", first.Items[1].Grants)
	}

	second, err := manager.ListUserGrantPage(context.Background(), first.NextCursor, 2)
	if err != nil {
		t.Fatalf("ListUserGrantPage(second) error = %v", err)
	}
	if second.TotalUsers != 3 || second.NextCursor != "" {
		t.Fatalf("second page meta = %+v, want total=3 no next", second)
	}
	if got := grantPageUserNames(second.Items); !slices.Equal(got, []string{"carol"}) {
		t.Fatalf("second page users = %v, want [carol]", got)
	}
}

func TestManagerListUserGrantPageUsesStrictSuccessorForDeletedCursor(t *testing.T) {
	manager := grantPageTestManager()

	page, err := manager.ListUserGrantPage(context.Background(), "beatrice", 2)
	if err != nil {
		t.Fatalf("ListUserGrantPage() error = %v", err)
	}
	if got := grantPageUserNames(page.Items); !slices.Equal(got, []string{"bob", "carol"}) {
		t.Fatalf("users = %v, want [bob carol]", got)
	}
}

func TestManagerListUserGrantPageReturnsClones(t *testing.T) {
	manager := grantPageTestManager()

	page, err := manager.ListUserGrantPage(context.Background(), "", 1)
	if err != nil {
		t.Fatalf("ListUserGrantPage() error = %v", err)
	}
	page.Items[0].User.Metadata["team"] = "mutated"
	page.Items[0].Grants[0].Database = "mutated"

	got, err := manager.ListUserGrantPage(context.Background(), "", 1)
	if err != nil {
		t.Fatalf("ListUserGrantPage(after mutation) error = %v", err)
	}
	if got.Items[0].User.Metadata["team"] != "platform" {
		t.Fatalf("metadata = %q, want platform", got.Items[0].User.Metadata["team"])
	}
	if got.Items[0].Grants[0].Database != "metrics" {
		t.Fatalf("database = %q, want metrics", got.Items[0].Grants[0].Database)
	}
}

func TestManagerListUserGrantPageRejectsCanceledContextAndInvalidLimit(t *testing.T) {
	manager := grantPageTestManager()
	canceled, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := manager.ListUserGrantPage(canceled, "", 1); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled error = %v, want context.Canceled", err)
	}
	if _, err := manager.ListUserGrantPage(context.Background(), "", 0); !errors.Is(err, ErrInvalidPageLimit) {
		t.Fatalf("zero limit error = %v, want ErrInvalidPageLimit", err)
	}
	if _, err := manager.ListUserGrantPage(context.Background(), "", MaxGrantPageLimit+1); !errors.Is(err, ErrInvalidPageLimit) {
		t.Fatalf("oversized limit error = %v, want ErrInvalidPageLimit", err)
	}
}

func TestManagerListUserGrantPageConcurrentPermissionChanges(t *testing.T) {
	ctx := context.Background()
	manager := openTestUserManager(t)
	if err := manager.CreateUser(ctx, User{Name: "alice"}); err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}

	const iterations = 50
	var waitGroup sync.WaitGroup
	waitGroup.Add(2)
	go func() {
		defer waitGroup.Done()
		for range iterations {
			if err := manager.GrantPermission(ctx, "alice", "metrics", PermissionRead); err != nil {
				t.Errorf("GrantPermission() error = %v", err)
				return
			}
			if err := manager.RevokePermission(ctx, "alice", "metrics", PermissionRead); err != nil {
				t.Errorf("RevokePermission() error = %v", err)
				return
			}
		}
	}()
	go func() {
		defer waitGroup.Done()
		for range iterations * 2 {
			page, err := manager.ListUserGrantPage(ctx, "", 1)
			if err != nil {
				t.Errorf("ListUserGrantPage() error = %v", err)
				return
			}
			if len(page.Items) != 1 || page.Items[0].User.Name != "alice" {
				t.Errorf("page = %+v, want alice", page)
				return
			}
			if len(page.Items[0].Grants) > 1 {
				t.Errorf("grants = %#v, want zero or one", page.Items[0].Grants)
				return
			}
		}
	}()
	waitGroup.Wait()
}

func grantPageTestManager() *Manager {
	store := newLocalStateStore("")
	store.users = map[string]User{
		"alice": {Name: "alice", Role: RoleAdmin, Metadata: map[string]string{"team": "platform"}},
		"bob":   {Name: "bob", Role: RoleUser},
		"carol": {Name: "carol", Role: RoleUser},
	}
	store.grants = map[string]map[string]map[Permission]struct{}{
		"alice": {
			"metrics": {PermissionRead: {}},
		},
		"carol": {
			"archive": {PermissionAdmin: {}},
		},
	}
	return &Manager{store: store, opts: DefaultOptions()}
}

func grantPageUserNames(items []UserGrantBundle) []string {
	names := make([]string, len(items))
	for index, item := range items {
		names[index] = item.User.Name
	}
	return names
}
