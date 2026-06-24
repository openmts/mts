package mts

import "sort"

func orderedDatabasePermissions() []DatabasePermission {
	return []DatabasePermission{
		DatabasePermissionRead,
		DatabasePermissionWrite,
		DatabasePermissionAdmin,
	}
}

func sortedGrantDatabases(grants map[string]map[DatabasePermission]struct{}) []string {
	databases := make([]string, 0, len(grants))
	for database := range grants {
		databases = append(databases, database)
	}
	sort.Strings(databases)
	return databases
}

func sortedStringMapKeys(values map[string]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func sortedUserNames(users map[string]User) []string {
	names := make([]string, 0, len(users))
	for name := range users {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func cloneUser(user User) User {
	user.Metadata = cloneStringMap(user.Metadata)
	return user
}

func cloneUsers(users map[string]User) map[string]User {
	out := make(map[string]User, len(users))
	for name, user := range users {
		out[name] = cloneUser(user)
	}
	return out
}

func cloneGrants(
	grants map[string]map[string]map[DatabasePermission]struct{},
) map[string]map[string]map[DatabasePermission]struct{} {
	out := make(map[string]map[string]map[DatabasePermission]struct{}, len(grants))
	for userName, userGrants := range grants {
		out[userName] = cloneUserGrants(userGrants)
	}
	return out
}

func cloneUserGrants(
	grants map[string]map[DatabasePermission]struct{},
) map[string]map[DatabasePermission]struct{} {
	out := make(map[string]map[DatabasePermission]struct{}, len(grants))
	for database, permissions := range grants {
		out[database] = clonePermissionSet(permissions)
	}
	return out
}

func clonePermissionSet(
	permissions map[DatabasePermission]struct{},
) map[DatabasePermission]struct{} {
	out := make(map[DatabasePermission]struct{}, len(permissions))
	for permission := range permissions {
		out[permission] = struct{}{}
	}
	return out
}
