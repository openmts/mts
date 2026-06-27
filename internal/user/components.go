package user

func (m *Manager) Users() UserStore {
	return m
}

func (m *Manager) Permissions() PermissionStore {
	return m
}

func (m *Manager) Credentials() CredentialStore {
	return m
}

func (m *Manager) Tokens() TokenStore {
	return m
}

func (m *Manager) Roles() RolePolicy {
	return DefaultRolePolicy{}
}
