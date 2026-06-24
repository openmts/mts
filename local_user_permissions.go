package mts

import (
	"context"
	"fmt"
	"strings"
)

func (m *LocalUserManager) GrantDatabasePermission(
	ctx context.Context,
	userName string,
	database string,
	permission DatabasePermission,
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
	if _, ok := m.users[userName]; !ok {
		return fmt.Errorf("%w: %s", ErrUserNotFound, userName)
	}
	users, grants := m.clonedStateLocked()
	ensureGrantSet(grants, userName, database)[permission] = struct{}{}
	return m.replaceStateLocked(users, grants)
}

func (m *LocalUserManager) RevokeDatabasePermission(
	ctx context.Context,
	userName string,
	database string,
	permission DatabasePermission,
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
	if _, ok := m.users[userName]; !ok {
		return fmt.Errorf("%w: %s", ErrUserNotFound, userName)
	}
	users, grants := m.clonedStateLocked()
	removeGrant(grants, userName, database, permission)
	return m.replaceStateLocked(users, grants)
}

func (m *LocalUserManager) ListDatabasePermissions(
	ctx context.Context,
	userName string,
) ([]DatabaseGrant, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	userName = strings.TrimSpace(userName)
	m.mu.RLock()
	defer m.mu.RUnlock()
	if _, ok := m.users[userName]; !ok {
		return nil, fmt.Errorf("%w: %s", ErrUserNotFound, userName)
	}
	return sortedDatabaseGrants(m.grants[userName]), nil
}

func (m *LocalUserManager) CheckDatabasePermission(
	ctx context.Context,
	userName string,
	database string,
	permission DatabasePermission,
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
	return checkDatabasePermission(m.users[userName], m.grants[userName], database, permission)
}

func validateGrantInput(
	userName string,
	database string,
	permission DatabasePermission,
) (string, string, error) {
	userName = strings.TrimSpace(userName)
	database = strings.TrimSpace(database)
	if userName == "" || database == "" || !validDatabasePermission(permission) {
		return "", "", ErrInvalidPermission
	}
	return userName, database, nil
}

func checkDatabasePermission(
	user User,
	grants map[string]map[DatabasePermission]struct{},
	database string,
	permission DatabasePermission,
) error {
	if user.Name == "" || user.Disabled {
		return ErrPermissionDenied
	}
	permissions := grants[database]
	if _, ok := permissions[permission]; ok {
		return nil
	}
	if _, ok := permissions[DatabasePermissionAdmin]; ok {
		return nil
	}
	return ErrPermissionDenied
}

func validDatabasePermission(permission DatabasePermission) bool {
	switch permission {
	case DatabasePermissionRead, DatabasePermissionWrite, DatabasePermissionAdmin:
		return true
	default:
		return false
	}
}

func ensureGrantSet(
	grants map[string]map[string]map[DatabasePermission]struct{},
	userName string,
	database string,
) map[DatabasePermission]struct{} {
	if grants[userName] == nil {
		grants[userName] = make(map[string]map[DatabasePermission]struct{})
	}
	if grants[userName][database] == nil {
		grants[userName][database] = make(map[DatabasePermission]struct{})
	}
	return grants[userName][database]
}

func removeGrant(
	grants map[string]map[string]map[DatabasePermission]struct{},
	userName string,
	database string,
	permission DatabasePermission,
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

func sortedDatabaseGrants(grants map[string]map[DatabasePermission]struct{}) []DatabaseGrant {
	databases := sortedGrantDatabases(grants)
	out := make([]DatabaseGrant, 0)
	for _, database := range databases {
		for _, permission := range orderedDatabasePermissions() {
			if _, ok := grants[database][permission]; ok {
				out = append(out, DatabaseGrant{Database: database, Permission: permission})
			}
		}
	}
	return out
}
