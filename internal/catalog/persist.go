package catalog

import (
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"codeberg.org/mts/mts/internal/codec"
	"codeberg.org/mts/mts/internal/model"
	"codeberg.org/mts/mts/internal/storagefs"
)

const (
	catalogRecordSeries = 1
	catalogRecordField  = 2

	snapshotCheckpointRecords = 4096
)

var catalogMagic = codec.Magic("MTSCAT2")

func (c *Catalog) snapshotPath() string {
	return filepath.Join(c.dir, "snapshot.bin")
}

func (c *Catalog) walPath() string {
	return filepath.Join(c.dir, "catalog.wal")
}

func (c *Catalog) loadSnapshot() error {
	data, err := storagefs.ReadFile(c.snapshotPath())
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("read catalog snapshot: %w", err)
	}
	snap, err := decodeSnapshot(data)
	if err != nil {
		return fmt.Errorf("decode catalog snapshot: %w", err)
	}
	c.nextSeriesID = max(snap.NextSeriesID, 1)
	c.nextFieldID = max(snap.NextFieldID, 1)
	for _, series := range snap.Series {
		c.applySeries(series)
	}
	for _, field := range snap.Fields {
		c.applyField(field)
	}
	return nil
}

func (c *Catalog) saveSnapshotLocked() error {
	data := encodeSnapshot(c.snapshotLocked())
	if err := storagefs.WriteFileAtomic(c.snapshotPath(), data); err != nil {
		return fmt.Errorf("write catalog snapshot: %w", err)
	}
	return nil
}

func (c *Catalog) checkpointSnapshotLocked(force bool) error {
	if c.snapshotDirtyRecords == 0 {
		return nil
	}
	if !force && c.snapshotDirtyRecords < snapshotCheckpointRecords {
		return nil
	}
	if err := c.saveSnapshotLocked(); err != nil {
		return err
	}
	if c.wal == nil {
		c.snapshotDirtyRecords = 0
		return nil
	}
	if err := c.wal.Truncate(0); err != nil {
		return fmt.Errorf("truncate catalog wal after snapshot: %w", err)
	}
	if _, err := c.wal.Seek(0, 0); err != nil {
		return fmt.Errorf("seek catalog wal after snapshot: %w", err)
	}
	if err := storagefs.Sync(c.wal); err != nil {
		return fmt.Errorf("sync truncated catalog wal: %w", err)
	}
	c.snapshotDirtyRecords = 0
	return nil
}

func (c *Catalog) replayWAL() error {
	data, err := storagefs.ReadFile(c.walPath())
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("read catalog wal for replay: %w", err)
	}
	replayed := 0
	for len(data) > 0 {
		entry, rest, err := decodeWALRecord(data)
		if err != nil {
			return err
		}
		c.applyEntry(entry)
		replayed++
		data = rest
	}
	c.snapshotDirtyRecords += replayed
	return nil
}

func (c *Catalog) appendEntryLocked(entry walEntry) error {
	payload, err := encodeWALEntry(entry)
	if err != nil {
		return fmt.Errorf("encode catalog wal entry: %w", err)
	}
	frame := codec.MarshalEnvelope(nil, catalogMagic, 0, payload)
	encoded := binary.AppendUvarint(nil, uint64(len(frame)))
	encoded = append(encoded, frame...)
	if err := storagefs.WriteFull(c.wal, encoded); err != nil {
		return fmt.Errorf("write catalog wal: %w", err)
	}
	if err := storagefs.Sync(c.wal); err != nil {
		return fmt.Errorf("sync catalog wal: %w", err)
	}
	return nil
}

func decodeLine(line []byte) (walEntry, error) {
	return decodeWALFrame(line)
}

func (c *Catalog) applyEntry(entry walEntry) {
	switch entry.Type {
	case "series":
		if entry.Series != nil {
			c.applySeries(*entry.Series)
		}
	case "field":
		if entry.Field != nil {
			c.applyField(*entry.Field)
		}
	}
}

func (c *Catalog) snapshotLocked() snapshot {
	snap := snapshot{NextSeriesID: c.nextSeriesID, NextFieldID: c.nextFieldID}
	snap.Series = make([]Series, 0, len(c.series))
	for _, series := range c.series {
		series.Tags = cloneTags(series.Tags)
		snap.Series = append(snap.Series, series)
	}
	snap.Fields = make([]Field, 0, len(c.fields))
	for _, field := range c.fields {
		snap.Fields = append(snap.Fields, field)
	}
	sort.Slice(snap.Series, func(i int, j int) bool { return snap.Series[i].ID < snap.Series[j].ID })
	sort.Slice(snap.Fields, func(i int, j int) bool { return snap.Fields[i].ID < snap.Fields[j].ID })
	return snap
}

func encodeSnapshot(snap snapshot) []byte {
	payload := binary.AppendUvarint(nil, snap.NextSeriesID)
	payload = binary.AppendUvarint(payload, uint64(snap.NextFieldID))
	payload = binary.AppendUvarint(payload, uint64(len(snap.Series)))
	for _, series := range snap.Series {
		payload = appendSeries(payload, series)
	}
	payload = binary.AppendUvarint(payload, uint64(len(snap.Fields)))
	for _, field := range snap.Fields {
		payload = appendField(payload, field)
	}
	return codec.MarshalEnvelope(nil, catalogMagic, 0, payload)
}

func decodeSnapshot(data []byte) (snapshot, error) {
	env, err := codec.UnmarshalEnvelope(data, catalogMagic)
	if err != nil {
		return snapshot{}, err
	}
	reader := newPayloadReader(env.Payload)
	snap, err := readSnapshotPayload(reader)
	if err != nil {
		return snapshot{}, err
	}
	if err := reader.done("catalog snapshot"); err != nil {
		return snapshot{}, err
	}
	return snap, nil
}

func readSnapshotPayload(reader *payloadReader) (snapshot, error) {
	nextSeriesID, err := reader.uvarint("next series id")
	if err != nil {
		return snapshot{}, err
	}
	nextFieldID, err := reader.uvarint("next field id")
	if err != nil {
		return snapshot{}, err
	}
	nextField, err := uint32Value("next field id", nextFieldID)
	if err != nil {
		return snapshot{}, err
	}
	series, err := readSeriesList(reader)
	if err != nil {
		return snapshot{}, err
	}
	fields, err := readFieldList(reader)
	if err != nil {
		return snapshot{}, err
	}
	return snapshot{NextSeriesID: nextSeriesID, NextFieldID: nextField, Series: series, Fields: fields}, nil
}

func readSeriesList(reader *payloadReader) ([]Series, error) {
	count, err := reader.intCount("series count")
	if err != nil {
		return nil, err
	}
	series := make([]Series, 0, count)
	for range count {
		item, err := readSeries(reader)
		if err != nil {
			return nil, err
		}
		series = append(series, item)
	}
	return series, nil
}

func readFieldList(reader *payloadReader) ([]Field, error) {
	count, err := reader.intCount("field count")
	if err != nil {
		return nil, err
	}
	fields := make([]Field, 0, count)
	for range count {
		field, err := readField(reader)
		if err != nil {
			return nil, err
		}
		fields = append(fields, field)
	}
	return fields, nil
}

func encodeWALEntry(entry walEntry) ([]byte, error) {
	switch entry.Type {
	case "series":
		if entry.Series == nil {
			return nil, fmt.Errorf("series wal entry is missing payload")
		}
		return appendSeries([]byte{catalogRecordSeries}, *entry.Series), nil
	case "field":
		if entry.Field == nil {
			return nil, fmt.Errorf("field wal entry is missing payload")
		}
		return appendField([]byte{catalogRecordField}, *entry.Field), nil
	default:
		return nil, fmt.Errorf("unsupported catalog wal entry type %q", entry.Type)
	}
}

func decodeWALRecord(data []byte) (walEntry, []byte, error) {
	length, size := binary.Uvarint(data)
	if size <= 0 {
		return walEntry{}, nil, fmt.Errorf("decode catalog wal record: invalid length")
	}
	start := size
	if length > uint64(len(data)-start) {
		return walEntry{}, nil, fmt.Errorf("decode catalog wal record: truncated frame")
	}
	end := start + int(length)
	entry, err := decodeWALFrame(data[start:end])
	if err != nil {
		return walEntry{}, nil, err
	}
	return entry, data[end:], nil
}

func decodeWALFrame(frame []byte) (walEntry, error) {
	env, err := codec.UnmarshalEnvelope(frame, catalogMagic)
	if err != nil {
		return walEntry{}, fmt.Errorf("decode catalog wal frame: %w", err)
	}
	reader := newPayloadReader(env.Payload)
	entry, err := readWALEntry(reader)
	if err != nil {
		return walEntry{}, err
	}
	if err := reader.done("catalog wal record"); err != nil {
		return walEntry{}, err
	}
	return entry, nil
}

func readWALEntry(reader *payloadReader) (walEntry, error) {
	recordType, err := reader.byte("record type")
	if err != nil {
		return walEntry{}, err
	}
	switch recordType {
	case catalogRecordSeries:
		series, err := readSeries(reader)
		return walEntry{Type: "series", Series: &series}, err
	case catalogRecordField:
		field, err := readField(reader)
		return walEntry{Type: "field", Field: &field}, err
	default:
		return walEntry{}, fmt.Errorf("decode catalog wal record: unsupported type %d", recordType)
	}
}

func appendSeries(dst []byte, series Series) []byte {
	dst = binary.AppendUvarint(dst, series.ID)
	dst = codec.AppendString(dst, series.Measurement)
	return appendTags(dst, series.Tags)
}

func readSeries(reader *payloadReader) (Series, error) {
	id, err := reader.uvarint("series id")
	if err != nil {
		return Series{}, err
	}
	measurement, err := reader.string("series measurement")
	if err != nil {
		return Series{}, err
	}
	tags, err := readTags(reader)
	return Series{ID: id, Measurement: measurement, Tags: tags}, err
}

func appendField(dst []byte, field Field) []byte {
	dst = binary.AppendUvarint(dst, uint64(field.ID))
	dst = codec.AppendString(dst, field.Measurement)
	dst = codec.AppendString(dst, field.Name)
	return append(dst, byte(field.Type))
}

func readField(reader *payloadReader) (Field, error) {
	id, err := reader.uvarint("field id")
	if err != nil {
		return Field{}, err
	}
	fieldID, err := uint32Value("field id", id)
	if err != nil {
		return Field{}, err
	}
	measurement, name, err := readFieldNames(reader)
	if err != nil {
		return Field{}, err
	}
	fieldType, err := reader.byte("field type")
	if err != nil {
		return Field{}, err
	}
	if !validFieldType(model.FieldType(fieldType)) {
		return Field{}, fmt.Errorf("decode catalog field: unsupported type %d", fieldType)
	}
	return Field{ID: fieldID, Measurement: measurement, Name: name, Type: model.FieldType(fieldType)}, nil
}

func readFieldNames(reader *payloadReader) (string, string, error) {
	measurement, err := reader.string("field measurement")
	if err != nil {
		return "", "", err
	}
	name, err := reader.string("field name")
	if err != nil {
		return "", "", err
	}
	return measurement, name, nil
}

func appendTags(dst []byte, tags map[string]string) []byte {
	keys := make([]string, 0, len(tags))
	for key := range tags {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	dst = binary.AppendUvarint(dst, uint64(len(keys)))
	for _, key := range keys {
		dst = codec.AppendString(dst, key)
		dst = codec.AppendString(dst, tags[key])
	}
	return dst
}

func readTags(reader *payloadReader) (map[string]string, error) {
	count, err := reader.intCount("tag count")
	if err != nil {
		return nil, err
	}
	tags := make(map[string]string, count)
	for range count {
		key, value, err := readTagPair(reader)
		if err != nil {
			return nil, err
		}
		tags[key] = value
	}
	return tags, nil
}

func readTagPair(reader *payloadReader) (string, string, error) {
	key, err := reader.string("tag key")
	if err != nil {
		return "", "", err
	}
	value, err := reader.string("tag value")
	if err != nil {
		return "", "", err
	}
	return key, value, nil
}

func validFieldType(fieldType model.FieldType) bool {
	switch fieldType {
	case model.FieldFloat64, model.FieldInt64, model.FieldString, model.FieldBool:
		return true
	default:
		return false
	}
}

func uint32Value(name string, value uint64) (uint32, error) {
	if value > uint64(^uint32(0)) {
		return 0, fmt.Errorf("decode catalog %s: value %d overflows uint32", name, value)
	}
	return uint32(value), nil
}

type payloadReader struct {
	rest []byte
}

func newPayloadReader(data []byte) *payloadReader {
	return &payloadReader{rest: data}
}

func (r *payloadReader) uvarint(name string) (uint64, error) {
	value, size := binary.Uvarint(r.rest)
	if size <= 0 {
		return 0, fmt.Errorf("decode catalog %s: invalid uvarint", name)
	}
	r.rest = r.rest[size:]
	return value, nil
}

func (r *payloadReader) intCount(name string) (int, error) {
	value, err := r.uvarint(name)
	if err != nil {
		return 0, err
	}
	maxInt := uint64(int(^uint(0) >> 1))
	if value > maxInt {
		return 0, fmt.Errorf("decode catalog %s: count %d overflows int", name, value)
	}
	return int(value), nil
}

func (r *payloadReader) string(name string) (string, error) {
	value, rest, err := codec.ReadString(r.rest)
	if err != nil {
		return "", fmt.Errorf("decode catalog %s: %w", name, err)
	}
	r.rest = rest
	return value, nil
}

func (r *payloadReader) byte(name string) (byte, error) {
	if len(r.rest) == 0 {
		return 0, fmt.Errorf("decode catalog %s: missing byte", name)
	}
	value := r.rest[0]
	r.rest = r.rest[1:]
	return value, nil
}

func (r *payloadReader) done(name string) error {
	if len(r.rest) != 0 {
		return fmt.Errorf("decode %s: %d trailing bytes", name, len(r.rest))
	}
	return nil
}
