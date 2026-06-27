package user

import "testing"

func TestManagerImplementsUserContracts(t *testing.T) {
	t.Parallel()
	var manager *Manager
	var _ UserStore = manager
	var _ PermissionStore = manager
	var _ CredentialStore = manager
	var _ TokenStore = manager
	var _ Authenticator = manager
	var _ Authorizer = manager
}
