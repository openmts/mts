package user

import "sort"

func orderedPermissions() []Permission {
	return []Permission{
		PermissionRead,
		PermissionWrite,
		PermissionAdmin,
	}
}

func sortedGrantDatabases(grants map[string]map[Permission]struct{}) []string {
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

func sortedTokenDigests(tokens map[string]tokenRecord) []string {
	digests := make([]string, 0, len(tokens))
	for digest := range tokens {
		digests = append(digests, digest)
	}
	sort.Strings(digests)
	return digests
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
	if values == nil {
		return nil
	}
	out := make(map[string]string, len(values))
	for key, value := range values {
		out[key] = value
	}
	return out
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
	out := make(map[string]tokenRecord, len(records))
	for digest, record := range records {
		out[digest] = record
	}
	return out
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
