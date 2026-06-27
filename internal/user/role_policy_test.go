package user

import "testing"

func TestDefaultRolePolicyAllowsAdminUserManagement(t *testing.T) {
	t.Parallel()
	policy := DefaultRolePolicy{}
	if !policy.CanManageUser(User{Role: RoleAdmin}, User{Role: RoleUser}) {
		t.Fatalf("admin should manage normal user")
	}
	if !policy.CanSetPassword(User{Role: RoleAdmin}, User{Role: RoleUser}) {
		t.Fatalf("admin should set normal user password")
	}
	if !policy.CanGrantDatabase(User{Role: RoleAdmin}, User{Role: RoleUser}) {
		t.Fatalf("admin should grant normal user database permission")
	}
}

func TestDefaultRolePolicyRejectsNormalUserAdminActions(t *testing.T) {
	t.Parallel()
	policy := DefaultRolePolicy{}
	if policy.CanManageUser(User{Role: RoleUser}, User{Role: RoleUser}) {
		t.Fatalf("normal user must not manage users")
	}
	if policy.CanSetPassword(User{Role: RoleUser}, User{Role: RoleAdmin}) {
		t.Fatalf("normal user must not set admin password")
	}
	if policy.CanGrantDatabase(User{Role: RoleUser}, User{Role: RoleUser}) {
		t.Fatalf("normal user must not grant database permission")
	}
}

func TestDefaultRolePolicyAllowsSelfPasswordChange(t *testing.T) {
	t.Parallel()
	policy := DefaultRolePolicy{}
	self := User{Name: "alice", Role: RoleUser}
	if !policy.CanChangeOwnPassword(self, "alice") {
		t.Fatalf("user should change own password")
	}
	if policy.CanChangeOwnPassword(self, "bob") {
		t.Fatalf("user must not change another user's own-password path")
	}
}
