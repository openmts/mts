package catalog

import (
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/openmts/mts/internal/codec"
	"github.com/openmts/mts/internal/collections"
	"github.com/openmts/mts/internal/model"
	"github.com/openmts/mts/internal/storagefs"
)

var downsampleMagic = codec.Magic("MTSDSP1")

func (c *Catalog) downsamplePath() string {
	return filepath.Join(c.dir, "downsample.bin")
}

func (c *Catalog) UpsertDownsamplePolicy(policy model.DownsamplePolicy) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	previous, existed := c.downsamplePolicies[policy.Name]
	c.downsamplePolicies[policy.Name] = cloneDownsamplePolicy(policy)
	if err := c.saveDownsampleMetadataLocked(); err != nil {
		if existed {
			c.downsamplePolicies[policy.Name] = previous
		} else {
			delete(c.downsamplePolicies, policy.Name)
		}
		return c.restoreDownsampleMetadataAfterErrorLocked(err)
	}
	return nil
}

func (c *Catalog) DropDownsamplePolicy(name string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	policy, policyExists := c.downsamplePolicies[name]
	watermark, watermarkExists := c.downsampleWatermarks[name]
	delete(c.downsamplePolicies, name)
	delete(c.downsampleWatermarks, name)
	if err := c.saveDownsampleMetadataLocked(); err != nil {
		if policyExists {
			c.downsamplePolicies[name] = policy
		}
		if watermarkExists {
			c.downsampleWatermarks[name] = watermark
		}
		return c.restoreDownsampleMetadataAfterErrorLocked(err)
	}
	return nil
}

func (c *Catalog) ListDownsamplePolicies() ([]model.DownsamplePolicy, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	names := sortedDownsamplePolicyNames(c.downsamplePolicies)
	out := make([]model.DownsamplePolicy, 0, len(names))
	for _, name := range names {
		out = append(out, cloneDownsamplePolicy(c.downsamplePolicies[name]))
	}
	return out, nil
}

func (c *Catalog) DownsampleWatermark(name string) (model.DownsampleWatermark, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	watermark, ok := c.downsampleWatermarks[name]
	return watermark, ok
}

func (c *Catalog) UpdateDownsampleWatermark(watermark model.DownsampleWatermark) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	previous, existed := c.downsampleWatermarks[watermark.PolicyName]
	c.downsampleWatermarks[watermark.PolicyName] = watermark
	if err := c.saveDownsampleMetadataLocked(); err != nil {
		if existed {
			c.downsampleWatermarks[watermark.PolicyName] = previous
		} else {
			delete(c.downsampleWatermarks, watermark.PolicyName)
		}
		return c.restoreDownsampleMetadataAfterErrorLocked(err)
	}
	return nil
}

func (c *Catalog) loadDownsampleMetadata() error {
	data, err := storagefs.ReadFile(c.downsamplePath())
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("read downsample metadata: %w", err)
	}
	policies, watermarks, err := decodeDownsampleMetadata(data)
	if err != nil {
		return fmt.Errorf("decode downsample metadata: %w", err)
	}
	c.downsamplePolicies = policies
	c.downsampleWatermarks = watermarks
	return nil
}

func (c *Catalog) saveDownsampleMetadataLocked() error {
	data := encodeDownsampleMetadata(c.downsamplePolicies, c.downsampleWatermarks)
	if err := storagefs.WriteFileAtomic(c.downsamplePath(), data); err != nil {
		return fmt.Errorf("write downsample metadata: %w", err)
	}
	return nil
}

func (c *Catalog) restoreDownsampleMetadataAfterErrorLocked(writeErr error) error {
	data := encodeDownsampleMetadata(c.downsamplePolicies, c.downsampleWatermarks)
	if err := storagefs.WriteFileAtomic(c.downsamplePath(), data); err != nil {
		return errors.Join(writeErr, fmt.Errorf("restore downsample metadata: %w", err))
	}
	return writeErr
}

func encodeDownsampleMetadata(
	policies map[string]model.DownsamplePolicy,
	watermarks map[string]model.DownsampleWatermark,
) []byte {
	payload := binary.AppendUvarint(nil, uint64(len(policies)))
	for _, name := range sortedDownsamplePolicyNames(policies) {
		payload = appendDownsamplePolicy(payload, policies[name])
	}
	payload = binary.AppendUvarint(payload, uint64(len(watermarks)))
	for _, name := range sortedDownsampleWatermarkNames(watermarks) {
		payload = appendDownsampleWatermark(payload, watermarks[name])
	}
	return codec.MarshalEnvelope(nil, downsampleMagic, 0, payload)
}

func decodeDownsampleMetadata(data []byte) (
	map[string]model.DownsamplePolicy,
	map[string]model.DownsampleWatermark,
	error,
) {
	env, err := codec.UnmarshalEnvelope(data, downsampleMagic)
	if err != nil {
		return nil, nil, err
	}
	reader := newPayloadReader(env.Payload)
	policies, watermarks, err := readDownsamplePayload(reader)
	if err != nil {
		return nil, nil, err
	}
	if err := reader.done("downsample metadata"); err != nil {
		return nil, nil, err
	}
	return policies, watermarks, nil
}

func readDownsamplePayload(reader *payloadReader) (
	map[string]model.DownsamplePolicy,
	map[string]model.DownsampleWatermark,
	error,
) {
	policyCount, err := reader.intCount("downsample policy count")
	if err != nil {
		return nil, nil, err
	}
	policies := make(map[string]model.DownsamplePolicy, policyCount)
	for range policyCount {
		policy, err := readDownsamplePolicy(reader)
		if err != nil {
			return nil, nil, err
		}
		policies[policy.Name] = policy
	}
	watermarkCount, err := reader.intCount("downsample watermark count")
	if err != nil {
		return nil, nil, err
	}
	watermarks := make(map[string]model.DownsampleWatermark, watermarkCount)
	for range watermarkCount {
		watermark, err := readDownsampleWatermark(reader)
		if err != nil {
			return nil, nil, err
		}
		watermarks[watermark.PolicyName] = watermark
	}
	return policies, watermarks, nil
}

func appendDownsamplePolicy(dst []byte, policy model.DownsamplePolicy) []byte {
	dst = codec.AppendString(dst, policy.Name)
	dst = codec.AppendString(dst, policy.SourceDatabase)
	dst = codec.AppendString(dst, policy.SourceRetention)
	dst = codec.AppendString(dst, policy.SourceMeasurement)
	dst = codec.AppendString(dst, policy.TargetDatabase)
	dst = codec.AppendString(dst, policy.TargetRetention)
	dst = codec.AppendString(dst, policy.TargetMeasurement)
	dst = binary.AppendVarint(dst, int64(policy.Interval))
	dst = appendDownsampleFunctions(dst, policy.Functions)
	dst = appendStringList(dst, policy.GroupByTags)
	dst = binary.AppendVarint(dst, int64(policy.Delay))
	dst = binary.AppendVarint(dst, int64(policy.RefreshInterval))
	dst = binary.AppendVarint(dst, int64(policy.Lookback))
	dst = binary.AppendVarint(dst, policy.InitialStartTime)
	dst = binary.AppendVarint(dst, int64(policy.RunTimeout))
	dst = binary.AppendVarint(dst, int64(policy.BatchSize))
	dst = binary.AppendVarint(dst, int64(policy.CheckpointInterval))
	dst = codec.AppendString(dst, policy.PolicyTagName)
	if policy.Enabled {
		dst = append(dst, 1)
	} else {
		dst = append(dst, 0)
	}
	return dst
}

func readDownsamplePolicy(reader *payloadReader) (model.DownsamplePolicy, error) {
	var policy model.DownsamplePolicy
	var err error
	if policy.Name, err = reader.string("downsample policy name"); err != nil {
		return model.DownsamplePolicy{}, err
	}
	if policy.SourceDatabase, err = reader.string("downsample source database"); err != nil {
		return model.DownsamplePolicy{}, err
	}
	if policy.SourceRetention, err = reader.string("downsample source retention"); err != nil {
		return model.DownsamplePolicy{}, err
	}
	if policy.SourceMeasurement, err = reader.string("downsample source measurement"); err != nil {
		return model.DownsamplePolicy{}, err
	}
	if policy.TargetDatabase, err = reader.string("downsample target database"); err != nil {
		return model.DownsamplePolicy{}, err
	}
	if policy.TargetRetention, err = reader.string("downsample target retention"); err != nil {
		return model.DownsamplePolicy{}, err
	}
	if policy.TargetMeasurement, err = reader.string("downsample target measurement"); err != nil {
		return model.DownsamplePolicy{}, err
	}
	interval, err := reader.varint("downsample interval")
	if err != nil {
		return model.DownsamplePolicy{}, err
	}
	policy.Interval = time.Duration(interval)
	if policy.Functions, err = readDownsampleFunctions(reader); err != nil {
		return model.DownsamplePolicy{}, err
	}
	if policy.GroupByTags, err = readStringList(reader, "downsample group tag"); err != nil {
		return model.DownsamplePolicy{}, err
	}
	delay, err := reader.varint("downsample delay")
	if err != nil {
		return model.DownsamplePolicy{}, err
	}
	refresh, err := reader.varint("downsample refresh interval")
	if err != nil {
		return model.DownsamplePolicy{}, err
	}
	lookback, err := reader.varint("downsample lookback")
	if err != nil {
		return model.DownsamplePolicy{}, err
	}
	initialStart, err := reader.varint("downsample initial start time")
	if err != nil {
		return model.DownsamplePolicy{}, err
	}
	runTimeout, err := reader.varint("downsample run timeout")
	if err != nil {
		return model.DownsamplePolicy{}, err
	}
	batchSize, err := reader.varint("downsample batch size")
	if err != nil {
		return model.DownsamplePolicy{}, err
	}
	checkpointInterval, err := reader.varint("downsample checkpoint interval")
	if err != nil {
		return model.DownsamplePolicy{}, err
	}
	if policy.PolicyTagName, err = reader.string("downsample policy tag name"); err != nil {
		return model.DownsamplePolicy{}, err
	}
	enabled, err := reader.byte("downsample enabled")
	if err != nil {
		return model.DownsamplePolicy{}, err
	}
	policy.Delay = time.Duration(delay)
	policy.RefreshInterval = time.Duration(refresh)
	policy.Lookback = time.Duration(lookback)
	policy.InitialStartTime = initialStart
	policy.RunTimeout = time.Duration(runTimeout)
	policy.BatchSize = int(batchSize)
	policy.CheckpointInterval = int(checkpointInterval)
	policy.Enabled = enabled != 0
	return policy, nil
}

func appendDownsampleFunctions(
	dst []byte,
	functions []model.DownsampleFunction,
) []byte {
	dst = binary.AppendUvarint(dst, uint64(len(functions)))
	for _, function := range functions {
		dst = codec.AppendString(dst, function.Function)
		dst = codec.AppendString(dst, function.Field)
		dst = codec.AppendString(dst, function.As)
	}
	return dst
}

func readDownsampleFunctions(reader *payloadReader) ([]model.DownsampleFunction, error) {
	count, err := reader.intCount("downsample function count")
	if err != nil {
		return nil, err
	}
	functions := make([]model.DownsampleFunction, 0, count)
	for range count {
		function, err := reader.string("downsample function")
		if err != nil {
			return nil, err
		}
		field, err := reader.string("downsample function field")
		if err != nil {
			return nil, err
		}
		as, err := reader.string("downsample function as")
		if err != nil {
			return nil, err
		}
		functions = append(functions, model.DownsampleFunction{
			Function: function,
			Field:    field,
			As:       as,
		})
	}
	return functions, nil
}

func appendDownsampleWatermark(dst []byte, watermark model.DownsampleWatermark) []byte {
	dst = codec.AppendString(dst, watermark.PolicyName)
	dst = binary.AppendVarint(dst, watermark.CompletedUntilUnix)
	dst = binary.AppendVarint(dst, watermark.LastRunUnix)
	dst = binary.AppendVarint(dst, watermark.LastSuccessUnix)
	dst = codec.AppendString(dst, watermark.LastError)
	if watermark.AllowPolicyReplace {
		dst = append(dst, 1)
	} else {
		dst = append(dst, 0)
	}
	return dst
}

func readDownsampleWatermark(reader *payloadReader) (model.DownsampleWatermark, error) {
	name, err := reader.string("downsample watermark policy")
	if err != nil {
		return model.DownsampleWatermark{}, err
	}
	completed, err := reader.varint("downsample watermark completed")
	if err != nil {
		return model.DownsampleWatermark{}, err
	}
	lastRun, err := reader.varint("downsample watermark last run")
	if err != nil {
		return model.DownsampleWatermark{}, err
	}
	lastSuccess, err := reader.varint("downsample watermark last success")
	if err != nil {
		return model.DownsampleWatermark{}, err
	}
	lastError, err := reader.string("downsample watermark last error")
	if err != nil {
		return model.DownsampleWatermark{}, err
	}
	allowReplace, err := reader.byte("downsample watermark allow replace")
	if err != nil {
		return model.DownsampleWatermark{}, err
	}
	return model.DownsampleWatermark{
		PolicyName:         name,
		CompletedUntilUnix: completed,
		LastRunUnix:        lastRun,
		LastSuccessUnix:    lastSuccess,
		LastError:          lastError,
		AllowPolicyReplace: allowReplace != 0,
	}, nil
}

func appendStringList(dst []byte, values []string) []byte {
	dst = binary.AppendUvarint(dst, uint64(len(values)))
	for _, value := range values {
		dst = codec.AppendString(dst, value)
	}
	return dst
}

func readStringList(reader *payloadReader, name string) ([]string, error) {
	count, err := reader.intCount(name + " count")
	if err != nil {
		return nil, err
	}
	values := make([]string, 0, count)
	for range count {
		value, err := reader.string(name)
		if err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, nil
}

func sortedDownsamplePolicyNames(policies map[string]model.DownsamplePolicy) []string {
	return collections.SortedKeys(policies)
}

func sortedDownsampleWatermarkNames(
	watermarks map[string]model.DownsampleWatermark,
) []string {
	return collections.SortedKeys(watermarks)
}

func cloneDownsamplePolicy(policy model.DownsamplePolicy) model.DownsamplePolicy {
	policy.Functions = collections.CloneSliceNilIfEmpty(policy.Functions)
	policy.GroupByTags = collections.CloneSliceNilIfEmpty(policy.GroupByTags)
	return policy
}
