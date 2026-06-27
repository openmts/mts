package user

import (
	"encoding/binary"
	"fmt"

	"github.com/openmts/mts/internal/codec"
)

var userMetadataMagic = codec.Magic("MTSUSR3")

func encodeUserMetadata(
	users map[string]User,
	grants map[string]map[string]map[Permission]struct{},
	passwords map[string]passwordRecord,
	tokens map[string]tokenRecord,
) []byte {
	payload := binary.AppendUvarint(nil, uint64(len(users)))
	for _, name := range sortedUserNames(users) {
		payload = appendUser(payload, users[name])
		payload = appendPasswordRecord(payload, passwords[name])
		payload = appendUserGrants(payload, grants[name])
	}
	payload = appendTokenRecords(payload, tokens)
	return codec.MarshalEnvelope(nil, userMetadataMagic, 0, payload)
}

func decodeUserMetadata(data []byte) (
	map[string]User,
	map[string]map[string]map[Permission]struct{},
	map[string]passwordRecord,
	map[string]tokenRecord,
	error,
) {
	env, err := codec.UnmarshalEnvelope(data, userMetadataMagic)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	reader := userMetadataReader{rest: env.Payload}
	users, grants, passwords, tokens, err := reader.read()
	if err != nil {
		return nil, nil, nil, nil, err
	}
	if len(reader.rest) != 0 {
		return nil, nil, nil, nil, fmt.Errorf("decode user metadata: trailing bytes")
	}
	return users, grants, passwords, tokens, nil
}

func appendUser(dst []byte, user User) []byte {
	dst = codec.AppendString(dst, user.Name)
	dst = codec.AppendString(dst, user.DisplayName)
	dst = codec.AppendString(dst, string(user.Role))
	if user.Disabled {
		dst = append(dst, 1)
	} else {
		dst = append(dst, 0)
	}
	return appendStringMap(dst, user.Metadata)
}

func appendUserGrants(dst []byte, grants map[string]map[Permission]struct{}) []byte {
	databases := sortedGrantDatabases(grants)
	dst = binary.AppendUvarint(dst, uint64(len(databases)))
	for _, database := range databases {
		dst = codec.AppendString(dst, database)
		dst = append(dst, permissionMask(grants[database]))
	}
	return dst
}

func appendPasswordRecord(dst []byte, record passwordRecord) []byte {
	if len(record.Salt) == 0 || len(record.Hash) == 0 || record.Iterations <= 0 {
		return append(dst, 0)
	}
	dst = append(dst, 1)
	dst = appendBytes(dst, record.Salt)
	dst = appendBytes(dst, record.Hash)
	return binary.AppendUvarint(dst, uint64(record.Iterations))
}

func appendTokenRecords(dst []byte, tokens map[string]tokenRecord) []byte {
	digests := sortedTokenDigests(tokens)
	dst = binary.AppendUvarint(dst, uint64(len(digests)))
	for _, digest := range digests {
		token := tokens[digest]
		dst = codec.AppendString(dst, digest)
		dst = codec.AppendString(dst, token.UserName)
		dst = binary.LittleEndian.AppendUint64(dst, uint64(token.ExpiresAtUnixNano))
	}
	return dst
}

func appendStringMap(dst []byte, values map[string]string) []byte {
	keys := sortedStringMapKeys(values)
	dst = binary.AppendUvarint(dst, uint64(len(keys)))
	for _, key := range keys {
		dst = codec.AppendString(dst, key)
		dst = codec.AppendString(dst, values[key])
	}
	return dst
}

func appendBytes(dst []byte, values []byte) []byte {
	dst = binary.AppendUvarint(dst, uint64(len(values)))
	return append(dst, values...)
}

type userMetadataReader struct {
	rest []byte
}

func (r *userMetadataReader) read() (
	map[string]User,
	map[string]map[string]map[Permission]struct{},
	map[string]passwordRecord,
	map[string]tokenRecord,
	error,
) {
	count, err := r.count("user count")
	if err != nil {
		return nil, nil, nil, nil, err
	}
	users := make(map[string]User, count)
	grants := make(map[string]map[string]map[Permission]struct{}, count)
	passwords := make(map[string]passwordRecord, count)
	for range count {
		user, err := r.user()
		if err != nil {
			return nil, nil, nil, nil, err
		}
		users[user.Name] = user
		password, err := r.passwordRecord()
		if err != nil {
			return nil, nil, nil, nil, err
		}
		if len(password.Salt) > 0 && len(password.Hash) > 0 {
			passwords[user.Name] = password
		}
		got, err := r.grants()
		if err != nil {
			return nil, nil, nil, nil, err
		}
		if len(got) > 0 {
			grants[user.Name] = got
		}
	}
	tokens, err := r.tokenRecords()
	if err != nil {
		return nil, nil, nil, nil, err
	}
	return users, grants, passwords, tokens, nil
}

func (r *userMetadataReader) user() (User, error) {
	name, err := r.string("user name")
	if err != nil {
		return User{}, err
	}
	displayName, err := r.string("display name")
	if err != nil {
		return User{}, err
	}
	roleText, err := r.string("role")
	if err != nil {
		return User{}, err
	}
	disabled, err := r.bool("disabled")
	if err != nil {
		return User{}, err
	}
	metadata, err := r.stringMap("metadata")
	if err != nil {
		return User{}, err
	}
	role := normalizeRole(Role(roleText))
	if !validRole(role) {
		return User{}, fmt.Errorf("decode user metadata role: invalid role %q", roleText)
	}
	return User{Name: name, DisplayName: displayName, Role: role, Disabled: disabled, Metadata: metadata}, nil
}

func (r *userMetadataReader) grants() (
	map[string]map[Permission]struct{},
	error,
) {
	count, err := r.count("grant database count")
	if err != nil {
		return nil, err
	}
	grants := make(map[string]map[Permission]struct{}, count)
	for range count {
		database, err := r.string("grant database")
		if err != nil {
			return nil, err
		}
		mask, err := r.byte("grant permission mask")
		if err != nil {
			return nil, err
		}
		permissions, err := permissionsFromMask(mask)
		if err != nil {
			return nil, err
		}
		grants[database] = permissions
	}
	return grants, nil
}

func (r *userMetadataReader) passwordRecord() (passwordRecord, error) {
	exists, err := r.bool("password exists")
	if err != nil {
		return passwordRecord{}, err
	}
	if !exists {
		return passwordRecord{}, nil
	}
	salt, err := r.bytes("password salt")
	if err != nil {
		return passwordRecord{}, err
	}
	hash, err := r.bytes("password hash")
	if err != nil {
		return passwordRecord{}, err
	}
	iterations, err := r.uvarint("password iterations")
	if err != nil {
		return passwordRecord{}, err
	}
	return passwordRecord{Salt: salt, Hash: hash, Iterations: iterations}, nil
}

func (r *userMetadataReader) tokenRecords() (map[string]tokenRecord, error) {
	count, err := r.count("token count")
	if err != nil {
		return nil, err
	}
	tokens := make(map[string]tokenRecord, count)
	for range count {
		digest, err := r.string("token digest")
		if err != nil {
			return nil, err
		}
		userName, err := r.string("token user")
		if err != nil {
			return nil, err
		}
		expiresAt, err := r.int64("token expires at")
		if err != nil {
			return nil, err
		}
		tokens[digest] = tokenRecord{UserName: userName, ExpiresAtUnixNano: expiresAt}
	}
	return tokens, nil
}

func (r *userMetadataReader) stringMap(name string) (map[string]string, error) {
	count, err := r.count(name + " count")
	if err != nil {
		return nil, err
	}
	values := make(map[string]string, count)
	for range count {
		key, err := r.string(name + " key")
		if err != nil {
			return nil, err
		}
		value, err := r.string(name + " value")
		if err != nil {
			return nil, err
		}
		values[key] = value
	}
	return values, nil
}

func (r *userMetadataReader) string(name string) (string, error) {
	value, rest, err := codec.ReadString(r.rest)
	if err != nil {
		return "", fmt.Errorf("decode user metadata %s: %w", name, err)
	}
	r.rest = rest
	return value, nil
}

func (r *userMetadataReader) count(name string) (int, error) {
	count, err := r.uvarint(name)
	if err != nil {
		return 0, err
	}
	if count > len(r.rest) {
		return 0, fmt.Errorf("decode user metadata %s: count too large", name)
	}
	return int(count), nil
}

func (r *userMetadataReader) uvarint(name string) (int, error) {
	count, size := binary.Uvarint(r.rest)
	if size <= 0 {
		return 0, fmt.Errorf("decode user metadata %s: invalid count", name)
	}
	r.rest = r.rest[size:]
	return int(count), nil
}

func (r *userMetadataReader) bytes(name string) ([]byte, error) {
	count, err := r.count(name)
	if err != nil {
		return nil, err
	}
	if count > len(r.rest) {
		return nil, fmt.Errorf("decode user metadata %s: truncated bytes", name)
	}
	value := append([]byte(nil), r.rest[:count]...)
	r.rest = r.rest[count:]
	return value, nil
}

func (r *userMetadataReader) int64(name string) (int64, error) {
	if len(r.rest) < 8 {
		return 0, fmt.Errorf("decode user metadata %s: truncated int64", name)
	}
	value := int64(binary.LittleEndian.Uint64(r.rest[:8]))
	r.rest = r.rest[8:]
	return value, nil
}

func (r *userMetadataReader) bool(name string) (bool, error) {
	value, err := r.byte(name)
	if err != nil {
		return false, err
	}
	switch value {
	case 0:
		return false, nil
	case 1:
		return true, nil
	default:
		return false, fmt.Errorf("decode user metadata %s: invalid bool %d", name, value)
	}
}

func (r *userMetadataReader) byte(name string) (byte, error) {
	if len(r.rest) == 0 {
		return 0, fmt.Errorf("decode user metadata %s: truncated byte", name)
	}
	value := r.rest[0]
	r.rest = r.rest[1:]
	return value, nil
}

func permissionMask(permissions map[Permission]struct{}) byte {
	var mask byte
	for permission := range permissions {
		switch permission {
		case PermissionRead:
			mask |= 1
		case PermissionWrite:
			mask |= 2
		case PermissionAdmin:
			mask |= 4
		}
	}
	return mask
}

func permissionsFromMask(mask byte) (map[Permission]struct{}, error) {
	if mask&^byte(7) != 0 {
		return nil, fmt.Errorf("decode user metadata permission mask: invalid bits %d", mask)
	}
	permissions := make(map[Permission]struct{}, 3)
	if mask&1 != 0 {
		permissions[PermissionRead] = struct{}{}
	}
	if mask&2 != 0 {
		permissions[PermissionWrite] = struct{}{}
	}
	if mask&4 != 0 {
		permissions[PermissionAdmin] = struct{}{}
	}
	return permissions, nil
}
