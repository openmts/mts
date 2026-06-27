package user

import (
	"context"
	"fmt"
	"strings"
)

func (m *Manager) GrantPermission(
	ctx context.Context,
	userName string,
	database string,
	permission Permission,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	userName, database, err := validateGrantInput(userName, database, permission)
	if err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.store.users[userName]; !ok {
		return fmt.Errorf("%w: %s", ErrUserNotFound, userName)
	}
	users, grants, passwords, tokens := m.clonedStateLocked()
	ensureGrantSet(grants, userName, database)[permission] = struct{}{}
	return m.replaceStateLocked(users, grants, passwords, tokens)
}

func (m *Manager) RevokePermission(
	ctx context.Context,
	userName string,
	database string,
	permission Permission,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	userName, database, err := validateGrantInput(userName, database, permission)
	if err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.store.users[userName]; !ok {
		return fmt.Errorf("%w: %s", ErrUserNotFound, userName)
	}
	users, grants, passwords, tokens := m.clonedStateLocked()
	removeGrant(grants, userName, database, permission)
	return m.replaceStateLocked(users, grants, passwords, tokens)
}

func (m *Manager) ListPermissions(
	ctx context.Context,
	userName string,
) ([]Grant, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	userName = strings.TrimSpace(userName)
	m.mu.RLock()
	defer m.mu.RUnlock()
	if _, ok := m.store.users[userName]; !ok {
		return nil, fmt.Errorf("%w: %s", ErrUserNotFound, userName)
	}
	return sortedGrants(m.store.grants[userName]), nil
}

func (m *Manager) CheckPermission(
	ctx context.Context,
	userName string,
	database string,
	permission Permission,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	userName, database, err := validateGrantInput(userName, database, permission)
	if err != nil {
		return err
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return checkPermission(m.store.users[userName], m.store.grants[userName], database, permission)
}

func validateGrantInput(
	userName string,
	database string,
	permission Permission,
) (string, string, error) {
	userName = strings.TrimSpace(userName)
	database = strings.TrimSpace(database)
	if userName == "" || database == "" || !validPermission(permission) {
		return "", "", ErrInvalidPermission
	}
	return userName, database, nil
}

func checkPermission(
	user User,
	grants map[string]map[Permission]struct{},
	database string,
	permission Permission,
) error {
	if user.Name == "" || user.Disabled {
		return ErrPermissionDenied
	}
	permissions := grants[database]
	if _, ok := permissions[permission]; ok {
		return nil
	}
	if _, ok := permissions[PermissionAdmin]; ok {
		return nil
	}
	return ErrPermissionDenied
}

func validPermission(permission Permission) bool {
	switch permission {
	case PermissionRead, PermissionWrite, PermissionAdmin:
		return true
	default:
		return false
	}
}

func ensureGrantSet(
	grants map[string]map[string]map[Permission]struct{},
	userName string,
	database string,
) map[Permission]struct{} {
	if grants[userName] == nil {
		grants[userName] = make(map[string]map[Permission]struct{})
	}
	if grants[userName][database] == nil {
		grants[userName][database] = make(map[Permission]struct{})
	}
	return grants[userName][database]
}

func removeGrant(
	grants map[string]map[string]map[Permission]struct{},
	userName string,
	database string,
	permission Permission,
) {
	permissions := grants[userName][database]
	delete(permissions, permission)
	if len(permissions) == 0 {
		delete(grants[userName], database)
	}
	if len(grants[userName]) == 0 {
		delete(grants, userName)
	}
}

func sortedGrants(grants map[string]map[Permission]struct{}) []Grant {
	databases := sortedGrantDatabases(grants)
	out := make([]Grant, 0)
	for _, database := range databases {
		for _, permission := range orderedPermissions() {
			if _, ok := grants[database][permission]; ok {
				out = append(out, Grant{Database: database, Permission: permission})
			}
		}
	}
	return out
}
