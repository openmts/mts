package user

import "testing"

func TestManagerExposesLocalComponents(t *testing.T) {
	t.Parallel()
	manager := openTestUserManager(t)
	t.Cleanup(func() {
		closeUserManager(t, manager)
	})

	if manager.Users() == nil {
		t.Fatalf("Users() = nil")
	}
	if manager.Permissions() == nil {
		t.Fatalf("Permissions() = nil")
	}
	if manager.Credentials() == nil {
		t.Fatalf("Credentials() = nil")
	}
	if manager.Tokens() == nil {
		t.Fatalf("Tokens() = nil")
	}
	if manager.Roles() == nil {
		t.Fatalf("Roles() = nil")
	}
}
