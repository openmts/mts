package user

import "github.com/openmts/mts/internal/collections"

func orderedPermissions() []Permission {
	return []Permission{
		PermissionRead,
		PermissionWrite,
		PermissionAdmin,
	}
}

func sortedGrantDatabases(grants map[string]map[Permission]struct{}) []string {
	return collections.SortedKeys(grants)
}

func sortedStringMapKeys(values map[string]string) []string {
	return collections.SortedKeys(values)
}

func sortedUserNames(users map[string]User) []string {
	return collections.SortedKeys(users)
}

func sortedTokenDigests(tokens map[string]tokenRecord) []string {
	return collections.SortedKeys(tokens)
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
	grants map[string]map[string]map[Permission]struct{},
) map[string]map[string]map[Permission]struct{} {
	out := make(map[string]map[string]map[Permission]struct{}, len(grants))
	for userName, userGrants := range grants {
		out[userName] = cloneUserGrants(userGrants)
	}
	return out
}

func cloneUserGrants(
	grants map[string]map[Permission]struct{},
) map[string]map[Permission]struct{} {
	out := make(map[string]map[Permission]struct{}, len(grants))
	for database, permissions := range grants {
		out[database] = clonePermissionSet(permissions)
	}
	return out
}

func clonePermissionSet(
	permissions map[Permission]struct{},
) map[Permission]struct{} {
	out := make(map[Permission]struct{}, len(permissions))
	for permission := range permissions {
		out[permission] = struct{}{}
	}
	return out
}

func cloneStringMap(values map[string]string) map[string]string {
	return collections.CloneMap(values)
}

func clonePasswordRecords(records map[string]passwordRecord) map[string]passwordRecord {
	out := make(map[string]passwordRecord, len(records))
	for userName, record := range records {
		out[userName] = clonePasswordRecord(record)
	}
	return out
}

func clonePasswordRecord(record passwordRecord) passwordRecord {
	record.Salt = append([]byte(nil), record.Salt...)
	record.Hash = append([]byte(nil), record.Hash...)
	return record
}

func cloneTokenRecords(records map[string]tokenRecord) map[string]tokenRecord {
	return collections.CloneMap(records)
}

func removeUserTokens(tokens map[string]tokenRecord, userName string) map[string]tokenRecord {
	out := cloneTokenRecords(tokens)
	for digest, token := range out {
		if token.UserName == userName {
			delete(out, digest)
		}
	}
	return out
}
