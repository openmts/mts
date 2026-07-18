package sstable

import (
	"encoding/binary"
	"fmt"
	"hash/crc32"

	"github.com/openmts/mts/internal/codec"
	"github.com/openmts/mts/internal/model"
)

var (
	partMagic        = codec.Magic("MTSPRT2")
	indexMagic       = codec.Magic("MTSIDX2")
	metaIndexMagic   = codec.Magic("MTSMIX2")
	seriesIndexMagic = codec.Magic("MTSSIX2")
	envelopeCRCTable = crc32.MakeTable(crc32.Castagnoli)
)

const (
	partMetadataFlagComponents     uint16 = 1
	partMetadataFlagComponentSizes uint16 = 2
	envelopeReservedLenBytes              = binary.MaxVarintLen64
)

var defaultPartComponents = []string{
	metadataFile,
	metaindexFile,
	indexFile,
	seriesIndexFile,
	timestampsFile,
	valuesFile,
	stringsFile,
}

func appendEnvelopePrefix(dst []byte, magic codec.Magic) []byte {
	var fixed [7]byte
	copy(fixed[:], string(magic))
	dst = append(dst, fixed[:]...)
	dst = binary.LittleEndian.AppendUint16(dst, 0)
	return append(dst, make([]byte, envelopeReservedLenBytes)...)
}

func finishEnvelope(dst []byte, payloadStart int) []byte {
	payloadLen := len(dst) - payloadStart
	var length [binary.MaxVarintLen64]byte
	size := binary.PutUvarint(length[:], uint64(payloadLen))
	lengthStart := payloadStart - envelopeReservedLenBytes
	finalPayloadStart := lengthStart + size
	copy(dst[finalPayloadStart:], dst[payloadStart:])
	dst = dst[:len(dst)-(envelopeReservedLenBytes-size)]
	copy(dst[lengthStart:], length[:size])
	sum := crc32.Checksum(dst, envelopeCRCTable)
	return binary.LittleEndian.AppendUint32(dst, sum)
}

func encodeMetadata(meta metadata) ([]byte, error) {
	payload := make([]byte, 0)
	var err error
	payload, err = appendPartMeta(payload, meta.Part)
	if err != nil {
		return nil, err
	}
	payload, err = appendBlockRef(payload, meta.IndexRef)
	if err != nil {
		return nil, err
	}
	payload, err = appendBlockRef(payload, meta.MetaIndexRef)
	if err != nil {
		return nil, err
	}
	payload, err = appendBlockRef(payload, meta.SeriesIndexRef)
	if err != nil {
		return nil, err
	}
	payload = binary.AppendVarint(payload, meta.CreatedUnix)
	components := metadataComponents(meta.Components)
	payload = appendStringSlice(payload, components)
	flags := partMetadataFlagComponents
	if len(meta.ComponentSizes) > 0 {
		payload = appendComponentSizes(payload, components, meta.ComponentSizes)
		flags |= partMetadataFlagComponentSizes
	}
	return codec.MarshalEnvelope(nil, partMagic, flags, payload), nil
}

func appendComponentSizes(dst []byte, components []string, sizes map[string]int64) []byte {
	dst = binary.AppendUvarint(dst, uint64(len(components)))
	for _, name := range components {
		size := int64(0)
		if sizes != nil {
			size = sizes[name]
		}
		dst = binary.AppendVarint(dst, size)
	}
	return dst
}

func decodeMetadata(data []byte) (metadata, error) {
	env, err := codec.UnmarshalEnvelope(data, partMagic)
	if err != nil {
		return metadata{}, err
	}
	reader := newBlockReader(env.Payload)
	part, err := readPartMeta(reader)
	if err != nil {
		return metadata{}, err
	}
	indexRef, metaIndexRef, seriesIndexRef, createdUnix, components, sizes, err := readMetadataTail(reader, env.Flags)
	if err != nil {
		return metadata{}, err
	}
	if err := reader.done("part metadata"); err != nil {
		return metadata{}, err
	}
	return metadata{
		Part:           part,
		IndexRef:       indexRef,
		MetaIndexRef:   metaIndexRef,
		SeriesIndexRef: seriesIndexRef,
		Components:     metadataComponents(components),
		ComponentSizes: sizes,
		CreatedUnix:    createdUnix,
	}, nil
}

func readMetadataTail(reader *blockReader, flags uint16) (blockRef, blockRef, blockRef, int64, []string, map[string]int64, error) {
	indexRef, err := readBlockRef(reader)
	if err != nil {
		return blockRef{}, blockRef{}, blockRef{}, 0, nil, nil, err
	}
	metaIndexRef, err := readBlockRef(reader)
	if err != nil {
		return blockRef{}, blockRef{}, blockRef{}, 0, nil, nil, err
	}
	seriesIndexRef, err := readBlockRef(reader)
	if err != nil {
		return blockRef{}, blockRef{}, blockRef{}, 0, nil, nil, err
	}
	createdUnix, err := reader.varint("created unix")
	if err != nil {
		return blockRef{}, blockRef{}, blockRef{}, 0, nil, nil, err
	}
	if flags&partMetadataFlagComponents == 0 && len(reader.rest) == 0 {
		return indexRef, metaIndexRef, seriesIndexRef, createdUnix, nil, nil, nil
	}
	components, err := readStringSlice(reader, "part component count")
	if err != nil {
		return blockRef{}, blockRef{}, blockRef{}, 0, nil, nil, err
	}
	var sizes map[string]int64
	if flags&partMetadataFlagComponentSizes != 0 {
		sizes, err = readComponentSizes(reader, metadataComponents(components))
		if err != nil {
			return blockRef{}, blockRef{}, blockRef{}, 0, nil, nil, err
		}
	}
	return indexRef, metaIndexRef, seriesIndexRef, createdUnix, components, sizes, nil
}

func readComponentSizes(reader *blockReader, components []string) (map[string]int64, error) {
	count, err := reader.intCount("component size count")
	if err != nil {
		return nil, err
	}
	if count != len(components) {
		return nil, fmt.Errorf("component size count %d does not match components %d", count, len(components))
	}
	sizes := make(map[string]int64, count)
	for _, name := range components {
		size, err := reader.varint("component size")
		if err != nil {
			return nil, err
		}
		if size < 0 {
			return nil, fmt.Errorf("component %s size %d is negative", name, size)
		}
		sizes[name] = size
	}
	return sizes, nil
}

func encodeIndexRows(rows []indexRow) ([]byte, error) {
	return encodeIndexRowsInto(nil, rows)
}

func encodeIndexRowsInto(dst []byte, rows []indexRow) ([]byte, error) {
	payload := appendEnvelopePrefix(dst[:0], indexMagic)
	payloadStart := len(payload)
	payload = binary.AppendUvarint(payload, uint64(len(rows)))
	var err error
	for _, row := range rows {
		payload, err = appendIndexRow(payload, row)
		if err != nil {
			return nil, err
		}
	}
	return finishEnvelope(payload, payloadStart), nil
}

func decodeIndexRows(data []byte) ([]indexRow, error) {
	env, err := codec.UnmarshalEnvelope(data, indexMagic)
	if err != nil {
		return nil, err
	}
	reader := newBlockReader(env.Payload)
	count, err := reader.intCount("index row count")
	if err != nil {
		return nil, err
	}
	rows := make([]indexRow, 0, count)
	for range count {
		row, err := readIndexRow(reader)
		if err != nil {
			return nil, err
		}
		rows = append(rows, row)
	}
	return rows, reader.done("index rows")
}

func newIndexRowStream(data []byte) (*indexRowStream, error) {
	env, err := codec.UnmarshalEnvelopeView(data, indexMagic)
	if err != nil {
		return nil, err
	}
	reader := newBlockReader(env.Payload)
	count, err := reader.intCount("index row count")
	if err != nil {
		return nil, err
	}
	return &indexRowStream{reader: reader, remaining: count}, nil
}

func (s *indexRowStream) nextHeader() (indexRowHeader, bool, error) {
	if s.remaining == 0 {
		return indexRowHeader{}, false, nil
	}
	seriesID, err := s.reader.uvarint("index series id")
	if err != nil {
		return indexRowHeader{}, false, err
	}
	minTime, maxTime, timeRef, err := readIndexRowCore(s.reader)
	if err != nil {
		return indexRowHeader{}, false, err
	}
	s.remaining--
	return indexRowHeader{
		seriesID: seriesID,
		minTime:  minTime,
		maxTime:  maxTime,
		timeRef:  timeRef,
	}, true, nil
}

func (s *indexRowStream) done() error {
	if s.remaining != 0 {
		return fmt.Errorf("decode index rows: %d rows remaining", s.remaining)
	}
	return s.reader.done("index rows")
}

func (s *indexRowStream) appendFilteredColumnRefs(
	dst []columnRef,
	filter map[uint32]struct{},
) ([]columnRef, error) {
	return readFilteredColumnRefsInto(s.reader, dst, filter)
}

func (s *indexRowStream) skipColumnRefs() error {
	return skipColumnRefs(s.reader)
}

func encodeMetaIndexRows(rows []metaIndexRow) ([]byte, error) {
	payload := binary.AppendUvarint(nil, uint64(len(rows)))
	var err error
	for _, row := range rows {
		payload, err = appendMetaIndexRow(payload, row)
		if err != nil {
			return nil, err
		}
	}
	return codec.MarshalEnvelope(nil, metaIndexMagic, 0, payload), nil
}

func decodeMetaIndexRows(data []byte) ([]metaIndexRow, error) {
	env, err := codec.UnmarshalEnvelope(data, metaIndexMagic)
	if err != nil {
		return nil, err
	}
	reader := newBlockReader(env.Payload)
	count, err := reader.intCount("metaindex row count")
	if err != nil {
		return nil, err
	}
	rows := make([]metaIndexRow, 0, count)
	for range count {
		row, err := readMetaIndexRow(reader)
		if err != nil {
			return nil, err
		}
		rows = append(rows, row)
	}
	return rows, reader.done("metaindex rows")
}

func encodeSeriesIndexRows(rows []seriesIndexRow) ([]byte, error) {
	payload := binary.AppendUvarint(nil, uint64(len(rows)))
	var err error
	for _, row := range rows {
		payload, err = appendSeriesIndexRow(payload, row)
		if err != nil {
			return nil, err
		}
	}
	return codec.MarshalEnvelope(nil, seriesIndexMagic, 0, payload), nil
}

func decodeSeriesIndexRows(data []byte) ([]seriesIndexRow, error) {
	env, err := codec.UnmarshalEnvelope(data, seriesIndexMagic)
	if err != nil {
		return nil, err
	}
	reader := newBlockReader(env.Payload)
	count, err := reader.intCount("series index row count")
	if err != nil {
		return nil, err
	}
	rows := make([]seriesIndexRow, 0, count)
	for range count {
		row, err := readSeriesIndexRow(reader)
		if err != nil {
			return nil, err
		}
		rows = append(rows, row)
	}
	return rows, reader.done("series index rows")
}

func appendPartMeta(dst []byte, meta PartMeta) ([]byte, error) {
	dst = codec.AppendString(dst, meta.ID)
	dst = binary.AppendVarint(dst, int64(meta.Level))
	dst = binary.AppendVarint(dst, meta.MinTime)
	dst = binary.AppendVarint(dst, meta.MaxTime)
	dst = binary.AppendUvarint(dst, meta.MinSeriesID)
	dst = binary.AppendUvarint(dst, meta.MaxSeriesID)
	return appendPartMetaCounts(dst, meta)
}

func appendPartMetaCounts(dst []byte, meta PartMeta) ([]byte, error) {
	if meta.RowsCount < 0 || meta.SeriesCount < 0 || meta.BlockCount < 0 {
		return nil, fmt.Errorf("part metadata contains negative count")
	}
	dst = binary.AppendUvarint(dst, uint64(meta.RowsCount))
	dst = binary.AppendUvarint(dst, uint64(meta.SeriesCount))
	dst = binary.AppendUvarint(dst, uint64(meta.BlockCount))
	dst = binary.AppendUvarint(dst, meta.MaxWriteSeq)
	dst = codec.AppendString(dst, meta.Path)
	return dst, nil
}

func readPartMeta(reader *blockReader) (PartMeta, error) {
	id, err := reader.string("part id")
	if err != nil {
		return PartMeta{}, err
	}
	level, err := reader.varint("part level")
	if err != nil {
		return PartMeta{}, err
	}
	times, err := readPartTimes(reader)
	if err != nil {
		return PartMeta{}, err
	}
	series, err := readPartSeries(reader)
	if err != nil {
		return PartMeta{}, err
	}
	counts, err := readPartCounts(reader)
	if err != nil {
		return PartMeta{}, err
	}
	path, err := reader.string("part path")
	if err != nil {
		return PartMeta{}, err
	}
	return buildPartMeta(id, int(level), times, series, counts, path), nil
}

type partTimes struct {
	min int64
	max int64
}

type partSeries struct {
	min uint64
	max uint64
}

type partCounts struct {
	rows     int
	series   int
	blocks   int
	writeSeq uint64
}

type indexRowHeader struct {
	seriesID uint64
	minTime  int64
	maxTime  int64
	timeRef  blockRef
}

type indexRowStream struct {
	reader    *blockReader
	remaining int
}

func readPartTimes(reader *blockReader) (partTimes, error) {
	minTime, err := reader.varint("part min time")
	if err != nil {
		return partTimes{}, err
	}
	maxTime, err := reader.varint("part max time")
	return partTimes{min: minTime, max: maxTime}, err
}

func readPartSeries(reader *blockReader) (partSeries, error) {
	minSeries, err := reader.uvarint("part min series")
	if err != nil {
		return partSeries{}, err
	}
	maxSeries, err := reader.uvarint("part max series")
	return partSeries{min: minSeries, max: maxSeries}, err
}

func readPartCounts(reader *blockReader) (partCounts, error) {
	rows, err := reader.intCount("part rows count")
	if err != nil {
		return partCounts{}, err
	}
	series, err := reader.intCount("part series count")
	if err != nil {
		return partCounts{}, err
	}
	blocks, err := reader.intCount("part block count")
	if err != nil {
		return partCounts{}, err
	}
	writeSeq, err := reader.uvarint("part max write seq")
	return partCounts{rows: rows, series: series, blocks: blocks, writeSeq: writeSeq}, err
}

func buildPartMeta(
	id string,
	level int,
	times partTimes,
	series partSeries,
	counts partCounts,
	path string,
) PartMeta {
	return PartMeta{
		ID: id, Level: level, MinTime: times.min, MaxTime: times.max,
		MinSeriesID: series.min, MaxSeriesID: series.max,
		RowsCount: counts.rows, SeriesCount: counts.series,
		BlockCount: counts.blocks, MaxWriteSeq: counts.writeSeq, Path: path,
	}
}

func appendIndexRow(dst []byte, row indexRow) ([]byte, error) {
	dst = binary.AppendUvarint(dst, row.SeriesID)
	dst = binary.AppendVarint(dst, row.MinTime)
	dst = binary.AppendVarint(dst, row.MaxTime)
	var err error
	dst, err = appendBlockRef(dst, row.TimeRef)
	if err != nil {
		return nil, err
	}
	dst = binary.AppendUvarint(dst, uint64(len(row.Columns)))
	for _, column := range row.Columns {
		dst, err = appendColumnRef(dst, column)
		if err != nil {
			return nil, err
		}
	}
	return dst, nil
}

func readIndexRow(reader *blockReader) (indexRow, error) {
	seriesID, err := reader.uvarint("index series id")
	if err != nil {
		return indexRow{}, err
	}
	minTime, maxTime, timeRef, err := readIndexRowCore(reader)
	if err != nil {
		return indexRow{}, err
	}
	columns, err := readColumnRefs(reader)
	if err != nil {
		return indexRow{}, err
	}
	return indexRow{SeriesID: seriesID, MinTime: minTime, MaxTime: maxTime, TimeRef: timeRef, Columns: columns}, nil
}

func (h indexRowHeader) indexRow(columns []columnRef) indexRow {
	return indexRow{
		SeriesID: h.seriesID,
		MinTime:  h.minTime,
		MaxTime:  h.maxTime,
		TimeRef:  h.timeRef,
		Columns:  columns,
	}
}

func readIndexRowCore(reader *blockReader) (int64, int64, blockRef, error) {
	minTime, err := reader.varint("index min time")
	if err != nil {
		return 0, 0, blockRef{}, err
	}
	maxTime, err := reader.varint("index max time")
	if err != nil {
		return 0, 0, blockRef{}, err
	}
	timeRef, err := readBlockRef(reader)
	return minTime, maxTime, timeRef, err
}

func appendColumnRef(dst []byte, column columnRef) ([]byte, error) {
	dst = binary.AppendUvarint(dst, uint64(column.FieldID))
	dst = append(dst, byte(column.FieldType))
	return appendBlockRef(dst, column.ValueRef)
}

func readColumnRefs(reader *blockReader) ([]columnRef, error) {
	return readFilteredColumnRefsInto(reader, nil, nil)
}

func readFilteredColumnRefsInto(
	reader *blockReader,
	dst []columnRef,
	filter map[uint32]struct{},
) ([]columnRef, error) {
	count, err := reader.intCount("column ref count")
	if err != nil {
		return nil, err
	}
	capacity := count
	if len(filter) > 0 && len(filter) < capacity {
		capacity = len(filter)
	}
	columns := dst[:0]
	if cap(columns) < capacity {
		columns = make([]columnRef, 0, capacity)
	}
	for range count {
		column, err := readColumnRef(reader)
		if err != nil {
			return nil, err
		}
		if containsField(filter, column.FieldID) {
			columns = append(columns, column)
		}
	}
	return columns, nil
}

func skipColumnRefs(reader *blockReader) error {
	count, err := reader.intCount("column ref count")
	if err != nil {
		return err
	}
	for range count {
		if _, err := readColumnRef(reader); err != nil {
			return err
		}
	}
	return nil
}

func readColumnRef(reader *blockReader) (columnRef, error) {
	fieldID, err := reader.uint32("column field id")
	if err != nil {
		return columnRef{}, err
	}
	fieldType, err := reader.byte("column field type")
	if err != nil {
		return columnRef{}, err
	}
	valueRef, err := readBlockRef(reader)
	return columnRef{FieldID: fieldID, FieldType: model.FieldType(fieldType), ValueRef: valueRef}, err
}

func appendMetaIndexRow(dst []byte, row metaIndexRow) ([]byte, error) {
	dst = binary.AppendUvarint(dst, row.MinSeriesID)
	dst = binary.AppendUvarint(dst, row.MaxSeriesID)
	dst = binary.AppendVarint(dst, row.MinTime)
	dst = binary.AppendVarint(dst, row.MaxTime)
	dst = appendUint32Slice(dst, row.FieldIDs)
	return appendBlockRef(dst, row.IndexRef)
}

func readMetaIndexRow(reader *blockReader) (metaIndexRow, error) {
	minSeries, maxSeries, minTime, maxTime, err := readMetaIndexBounds(reader)
	if err != nil {
		return metaIndexRow{}, err
	}
	fieldIDs, err := readUint32Slice(reader, "metaindex field id count")
	if err != nil {
		return metaIndexRow{}, err
	}
	indexRef, err := readBlockRef(reader)
	return metaIndexRow{
		MinSeriesID: minSeries, MaxSeriesID: maxSeries,
		MinTime: minTime, MaxTime: maxTime, FieldIDs: fieldIDs, IndexRef: indexRef,
	}, err
}

func readMetaIndexBounds(reader *blockReader) (uint64, uint64, int64, int64, error) {
	minSeries, err := reader.uvarint("metaindex min series")
	if err != nil {
		return 0, 0, 0, 0, err
	}
	maxSeries, err := reader.uvarint("metaindex max series")
	if err != nil {
		return 0, 0, 0, 0, err
	}
	minTime, err := reader.varint("metaindex min time")
	if err != nil {
		return 0, 0, 0, 0, err
	}
	maxTime, err := reader.varint("metaindex max time")
	return minSeries, maxSeries, minTime, maxTime, err
}

func appendSeriesIndexRow(dst []byte, row seriesIndexRow) ([]byte, error) {
	dst = binary.AppendUvarint(dst, row.SeriesID)
	dst = binary.AppendVarint(dst, row.MinTime)
	dst = binary.AppendVarint(dst, row.MaxTime)
	dst = appendUint32Slice(dst, row.FieldIDs)
	return appendBlockRef(dst, row.IndexRef)
}

func readSeriesIndexRow(reader *blockReader) (seriesIndexRow, error) {
	seriesID, err := reader.uvarint("series index series id")
	if err != nil {
		return seriesIndexRow{}, err
	}
	minTime, err := reader.varint("series index min time")
	if err != nil {
		return seriesIndexRow{}, err
	}
	maxTime, err := reader.varint("series index max time")
	if err != nil {
		return seriesIndexRow{}, err
	}
	fieldIDs, err := readUint32Slice(reader, "series index field id count")
	if err != nil {
		return seriesIndexRow{}, err
	}
	indexRef, err := readBlockRef(reader)
	return seriesIndexRow{
		SeriesID: seriesID,
		MinTime:  minTime,
		MaxTime:  maxTime,
		FieldIDs: fieldIDs,
		IndexRef: indexRef,
	}, err
}

func appendUint32Slice(dst []byte, values []uint32) []byte {
	dst = binary.AppendUvarint(dst, uint64(len(values)))
	for _, value := range values {
		dst = binary.AppendUvarint(dst, uint64(value))
	}
	return dst
}

func appendStringSlice(dst []byte, values []string) []byte {
	dst = binary.AppendUvarint(dst, uint64(len(values)))
	for _, value := range values {
		dst = codec.AppendString(dst, value)
	}
	return dst
}

func readStringSlice(reader *blockReader, name string) ([]string, error) {
	count, err := reader.intCount(name)
	if err != nil {
		return nil, err
	}
	values := make([]string, 0, count)
	for range count {
		value, err := reader.string("string slice value")
		if err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, nil
}

func metadataComponents(values []string) []string {
	if len(values) == 0 {
		return append([]string(nil), defaultPartComponents...)
	}
	return append([]string(nil), values...)
}

func readUint32Slice(reader *blockReader, name string) ([]uint32, error) {
	count, err := reader.intCount(name)
	if err != nil {
		return nil, err
	}
	values := make([]uint32, 0, count)
	for range count {
		value, err := reader.uint32("uint32 slice value")
		if err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, nil
}

func appendBlockRef(dst []byte, ref blockRef) ([]byte, error) {
	if ref.Offset < 0 || ref.Size < 0 {
		return nil, fmt.Errorf("block ref contains negative values")
	}
	dst = binary.AppendUvarint(dst, uint64(ref.Offset))
	dst = binary.AppendUvarint(dst, uint64(ref.Size))
	return dst, nil
}

func readBlockRef(reader *blockReader) (blockRef, error) {
	offset, err := reader.uvarint("block offset")
	if err != nil {
		return blockRef{}, err
	}
	size, err := reader.uvarint("block size")
	if err != nil {
		return blockRef{}, err
	}
	if offset > maxInt64() || size > maxInt64() {
		return blockRef{}, fmt.Errorf("block ref overflows int64")
	}
	return blockRef{Offset: int64(offset), Size: int64(size)}, nil
}

func maxInt64() uint64 {
	return uint64(^uint64(0) >> 1)
}
