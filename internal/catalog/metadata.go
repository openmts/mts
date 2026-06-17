package catalog

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"codeberg.org/mts/mts/internal/codec"
	"codeberg.org/mts/mts/internal/model"
	"codeberg.org/mts/mts/internal/storagefs"
)

var metadataMagic = codec.Magic("MTSMETA")

func (c *Catalog) metadataPath() string {
	return filepath.Join(c.dir, "metadata.bin")
}

func (c *Catalog) loadMetadata() error {
	data, err := storagefs.ReadFile(c.metadataPath())
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read catalog metadata: %w", err)
	}
	databases, policies, err := decodeMetadata(data)
	if err != nil {
		return fmt.Errorf("decode catalog metadata: %w", err)
	}
	c.databases = databases
	c.policies = policies
	return nil
}

func (c *Catalog) saveMetadataLocked() error {
	data := encodeMetadata(c.databases, c.policies)
	if err := storagefs.WriteFileAtomic(c.metadataPath(), data); err != nil {
		return fmt.Errorf("write catalog metadata: %w", err)
	}
	return nil
}

func encodeMetadata(
	databases map[string]struct{},
	policies map[string]map[string]model.RetentionPolicy,
) []byte {
	payload := binary.AppendUvarint(nil, uint64(len(databases)))
	names := sortedDatabaseNames(databases)
	for _, name := range names {
		payload = codec.AppendString(payload, name)
		payload = appendPolicies(payload, policies[name])
	}
	return codec.MarshalEnvelope(nil, metadataMagic, 0, payload)
}

func decodeMetadata(data []byte) (
	map[string]struct{},
	map[string]map[string]model.RetentionPolicy,
	error,
) {
	env, err := codec.UnmarshalEnvelope(data, metadataMagic)
	if err != nil {
		return nil, nil, err
	}
	reader := newPayloadReader(env.Payload)
	databases, policies, err := readMetadataPayload(reader)
	if err != nil {
		return nil, nil, err
	}
	if err := reader.done("catalog metadata"); err != nil {
		return nil, nil, err
	}
	return databases, policies, nil
}

func readMetadataPayload(reader *payloadReader) (
	map[string]struct{},
	map[string]map[string]model.RetentionPolicy,
	error,
) {
	count, err := reader.intCount("database count")
	if err != nil {
		return nil, nil, err
	}
	databases := make(map[string]struct{}, count)
	policies := make(map[string]map[string]model.RetentionPolicy, count)
	for range count {
		name, err := reader.string("database name")
		if err != nil {
			return nil, nil, err
		}
		databases[name] = struct{}{}
		got, err := readPolicies(reader)
		if err != nil {
			return nil, nil, err
		}
		policies[name] = got
	}
	return databases, policies, nil
}

func appendPolicies(dst []byte, policies map[string]model.RetentionPolicy) []byte {
	names := sortedPolicyNames(policies)
	dst = binary.AppendUvarint(dst, uint64(len(names)))
	for _, name := range names {
		policy := policies[name]
		dst = codec.AppendString(dst, policy.Name)
		dst = binary.AppendVarint(dst, int64(policy.Duration))
	}
	return dst
}

func readPolicies(reader *payloadReader) (map[string]model.RetentionPolicy, error) {
	count, err := reader.intCount("retention policy count")
	if err != nil {
		return nil, err
	}
	policies := make(map[string]model.RetentionPolicy, count)
	for range count {
		name, err := reader.string("retention policy name")
		if err != nil {
			return nil, err
		}
		duration, err := reader.varint("retention policy duration")
		if err != nil {
			return nil, err
		}
		policies[name] = model.RetentionPolicy{Name: name, Duration: time.Duration(duration)}
	}
	return policies, nil
}

func sortedDatabaseNames(databases map[string]struct{}) []string {
	names := make([]string, 0, len(databases))
	for name := range databases {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func sortedPolicyNames(policies map[string]model.RetentionPolicy) []string {
	names := make([]string, 0, len(policies))
	for name := range policies {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func (r *payloadReader) varint(name string) (int64, error) {
	value, size := binary.Varint(r.rest)
	if size <= 0 {
		return 0, fmt.Errorf("decode catalog %s: invalid varint", name)
	}
	r.rest = r.rest[size:]
	return value, nil
}
