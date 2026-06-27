package user

import "testing"

func TestLocalStateStoreReplaceAndLoad(t *testing.T) {
	t.Parallel()
	store := newLocalStateStore(t.TempDir())
	users := map[string]User{
		"alice": {Name: "alice", Role: RoleAdmin},
	}
	grants := map[string]map[string]map[Permission]struct{}{
		"alice": {"metrics": {PermissionAdmin: {}}},
	}
	passwords := map[string]passwordRecord{
		"alice": {Salt: []byte{1}, Hash: []byte{2}, Iterations: 3},
	}
	tokens := map[string]tokenRecord{
		"token": {UserName: "alice", ExpiresAtUnixNano: 4},
	}

	if err := store.replace(users, grants, passwords, tokens); err != nil {
		t.Fatalf("replace() error = %v", err)
	}

	reopened := newLocalStateStore(t.TempDir())
	reopened.path = store.path
	loadedUsers, loadedGrants, loadedPasswords, loadedTokens, err := reopened.load()
	if err != nil {
		t.Fatalf("load() error = %v", err)
	}
	if loadedUsers["alice"].Role != RoleAdmin {
		t.Fatalf("loaded users = %#v, want alice admin", loadedUsers)
	}
	if _, ok := loadedGrants["alice"]["metrics"][PermissionAdmin]; !ok {
		t.Fatalf("loaded grants = %#v, want admin grant", loadedGrants)
	}
	if loadedPasswords["alice"].Iterations != 3 {
		t.Fatalf("loaded passwords = %#v, want iterations 3", loadedPasswords)
	}
	if loadedTokens["token"].UserName != "alice" {
		t.Fatalf("loaded tokens = %#v, want alice token", loadedTokens)
	}
}
