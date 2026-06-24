package mts

import (
	"encoding/binary"
	"fmt"

	"github.com/openmts/mts/internal/codec"
)

var userMetadataMagic = codec.Magic("MTSUSR1")

func encodeUserMetadata(
	users map[string]User,
	grants map[string]map[string]map[DatabasePermission]struct{},
) []byte {
	payload := binary.AppendUvarint(nil, uint64(len(users)))
	for _, name := range sortedUserNames(users) {
		payload = appendUser(payload, users[name])
		payload = appendUserGrants(payload, grants[name])
	}
	return codec.MarshalEnvelope(nil, userMetadataMagic, 0, payload)
}

func decodeUserMetadata(data []byte) (
	map[string]User,
	map[string]map[string]map[DatabasePermission]struct{},
	error,
) {
	env, err := codec.UnmarshalEnvelope(data, userMetadataMagic)
	if err != nil {
		return nil, nil, err
	}
	reader := userMetadataReader{rest: env.Payload}
	users, grants, err := reader.read()
	if err != nil {
		return nil, nil, err
	}
	if len(reader.rest) != 0 {
		return nil, nil, fmt.Errorf("decode user metadata: trailing bytes")
	}
	return users, grants, nil
}

func appendUser(dst []byte, user User) []byte {
	dst = codec.AppendString(dst, user.Name)
	dst = codec.AppendString(dst, user.DisplayName)
	if user.Disabled {
		dst = append(dst, 1)
	} else {
		dst = append(dst, 0)
	}
	return appendStringMap(dst, user.Metadata)
}

func appendUserGrants(dst []byte, grants map[string]map[DatabasePermission]struct{}) []byte {
	databases := sortedGrantDatabases(grants)
	dst = binary.AppendUvarint(dst, uint64(len(databases)))
	for _, database := range databases {
		dst = codec.AppendString(dst, database)
		dst = append(dst, permissionMask(grants[database]))
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

type userMetadataReader struct {
	rest []byte
}

func (r *userMetadataReader) read() (
	map[string]User,
	map[string]map[string]map[DatabasePermission]struct{},
	error,
) {
	count, err := r.count("user count")
	if err != nil {
		return nil, nil, err
	}
	users := make(map[string]User, count)
	grants := make(map[string]map[string]map[DatabasePermission]struct{}, count)
	for range count {
		user, err := r.user()
		if err != nil {
			return nil, nil, err
		}
		users[user.Name] = user
		got, err := r.grants()
		if err != nil {
			return nil, nil, err
		}
		if len(got) > 0 {
			grants[user.Name] = got
		}
	}
	return users, grants, nil
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
	disabled, err := r.bool("disabled")
	if err != nil {
		return User{}, err
	}
	metadata, err := r.stringMap("metadata")
	if err != nil {
		return User{}, err
	}
	return User{Name: name, DisplayName: displayName, Disabled: disabled, Metadata: metadata}, nil
}

func (r *userMetadataReader) grants() (
	map[string]map[DatabasePermission]struct{},
	error,
) {
	count, err := r.count("grant database count")
	if err != nil {
		return nil, err
	}
	grants := make(map[string]map[DatabasePermission]struct{}, count)
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
	count, size := binary.Uvarint(r.rest)
	if size <= 0 {
		return 0, fmt.Errorf("decode user metadata %s: invalid count", name)
	}
	r.rest = r.rest[size:]
	if count > uint64(len(r.rest)) {
		return 0, fmt.Errorf("decode user metadata %s: count too large", name)
	}
	return int(count), nil
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

func permissionMask(permissions map[DatabasePermission]struct{}) byte {
	var mask byte
	for permission := range permissions {
		switch permission {
		case DatabasePermissionRead:
			mask |= 1
		case DatabasePermissionWrite:
			mask |= 2
		case DatabasePermissionAdmin:
			mask |= 4
		}
	}
	return mask
}

func permissionsFromMask(mask byte) (map[DatabasePermission]struct{}, error) {
	if mask&^byte(7) != 0 {
		return nil, fmt.Errorf("decode user metadata permission mask: invalid bits %d", mask)
	}
	permissions := make(map[DatabasePermission]struct{}, 3)
	if mask&1 != 0 {
		permissions[DatabasePermissionRead] = struct{}{}
	}
	if mask&2 != 0 {
		permissions[DatabasePermissionWrite] = struct{}{}
	}
	if mask&4 != 0 {
		permissions[DatabasePermissionAdmin] = struct{}{}
	}
	return permissions, nil
}
