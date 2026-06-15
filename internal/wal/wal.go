package wal

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"hash/crc32"
	"io"
	"os"
	"path/filepath"
	"sort"

	"codeberg.org/mts/mts/internal/model"
	"codeberg.org/mts/mts/internal/storagefs"
)

const (
	defaultSegmentBytes int64 = 64 << 20
	recordVersion       byte  = 1
	recordWriteBatch    byte  = 1
)

var castagnoliTable = crc32.MakeTable(crc32.Castagnoli)

type Options struct {
	Sync         bool
	SegmentBytes int64
	BatchRecords int
	BatchBytes   int64
}

type Log struct {
	dir  string
	opts Options

	file    *os.File
	segment int
	size    int64

	pendingRecords int
	pendingBytes   int64
}

func Open(dir string, opts Options) (*Log, error) {
	if err := storagefs.MkdirAll(dir); err != nil {
		return nil, err
	}
	if opts.SegmentBytes <= 0 {
		opts.SegmentBytes = defaultSegmentBytes
	}
	log := &Log{
		dir:     filepath.Clean(dir),
		opts:    opts,
		segment: 1,
	}
	if err := log.openLastSegment(); err != nil {
		return nil, err
	}
	return log, nil
}

func (l *Log) Append(records []model.ResolvedPoint, syncWrite bool) error {
	payload, err := json.Marshal(records)
	if err != nil {
		return fmt.Errorf("encode wal record: %w", err)
	}
	frame := encodeFrame(recordWriteBatch, payload)
	if err := l.rollIfNeeded(int64(len(frame))); err != nil {
		return err
	}
	if _, err := l.file.Write(frame); err != nil {
		return fmt.Errorf("write wal record: %w", err)
	}
	l.size += int64(len(frame))
	l.pendingRecords++
	l.pendingBytes += int64(len(frame))
	if l.shouldSync(syncWrite) {
		if err := l.file.Sync(); err != nil {
			return fmt.Errorf("sync wal: %w", err)
		}
		l.pendingRecords = 0
		l.pendingBytes = 0
	}
	return nil
}

func (l *Log) Replay() ([]model.ResolvedPoint, error) {
	segments, err := listSegments(l.dir)
	if err != nil {
		return nil, err
	}
	points := make([]model.ResolvedPoint, 0)
	for index, segment := range segments {
		isLast := index == len(segments)-1
		replayed, err := replaySegment(segment.path, isLast)
		if err != nil {
			return nil, err
		}
		points = append(points, replayed...)
	}
	return points, nil
}

func (l *Log) TruncateAll() error {
	if l.file != nil {
		if err := l.file.Close(); err != nil {
			return fmt.Errorf("close wal before truncate: %w", err)
		}
		l.file = nil
	}
	segments, err := listSegments(l.dir)
	if err != nil {
		return err
	}
	for _, segment := range segments {
		if err := os.Remove(segment.path); err != nil {
			return fmt.Errorf("remove wal segment: %w", err)
		}
	}
	l.segment = 1
	l.size = 0
	return l.openSegment(1)
}

func (l *Log) Close() error {
	if l.file == nil {
		return nil
	}
	if err := l.file.Close(); err != nil {
		return fmt.Errorf("close wal: %w", err)
	}
	l.file = nil
	return nil
}

func (l *Log) openLastSegment() error {
	segments, err := listSegments(l.dir)
	if err != nil {
		return err
	}
	if len(segments) == 0 {
		return l.openSegment(1)
	}
	last := segments[len(segments)-1]
	l.segment = last.number
	return l.openSegment(last.number)
}

func (l *Log) openSegment(number int) error {
	path := segmentPath(l.dir, number)
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_RDWR, storagefs.FileMode)
	if err != nil {
		return fmt.Errorf("open wal segment: %w", err)
	}
	info, err := file.Stat()
	if err != nil {
		closeErr := file.Close()
		return fmt.Errorf("stat wal segment: %w close: %v", err, closeErr)
	}
	l.file = file
	l.segment = number
	l.size = info.Size()
	return nil
}

func (l *Log) rollIfNeeded(frameSize int64) error {
	if l.size == 0 || l.size+frameSize <= l.opts.SegmentBytes {
		return nil
	}
	if err := l.file.Close(); err != nil {
		return fmt.Errorf("close wal segment before roll: %w", err)
	}
	l.file = nil
	return l.openSegment(l.segment + 1)
}

func (l *Log) shouldSync(syncWrite bool) bool {
	if syncWrite || l.opts.Sync {
		return true
	}
	if l.opts.BatchRecords > 0 && l.pendingRecords >= l.opts.BatchRecords {
		return true
	}
	return l.opts.BatchBytes > 0 && l.pendingBytes >= l.opts.BatchBytes
}

func encodeFrame(recordType byte, payload []byte) []byte {
	bodyLen := 1 + 1 + len(payload) + 4
	frame := make([]byte, 4+bodyLen)
	binary.BigEndian.PutUint32(frame[:4], uint32(bodyLen))
	frame[4] = recordVersion
	frame[5] = recordType
	copy(frame[6:], payload)
	checksum := crc32.Checksum(frame[4:4+bodyLen-4], castagnoliTable)
	binary.BigEndian.PutUint32(frame[4+bodyLen-4:], checksum)
	return frame
}

type segmentInfo struct {
	number int
	path   string
}

func listSegments(dir string) ([]segmentInfo, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read wal directory: %w", err)
	}
	segments := make([]segmentInfo, 0)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		var number int
		if _, err := fmt.Sscanf(entry.Name(), "%06d.wal", &number); err != nil {
			continue
		}
		segments = append(segments, segmentInfo{
			number: number,
			path:   filepath.Join(dir, entry.Name()),
		})
	}
	sort.Slice(segments, func(i, j int) bool {
		return segments[i].number < segments[j].number
	})
	return segments, nil
}

func segmentPath(dir string, number int) string {
	return filepath.Join(dir, fmt.Sprintf("%06d.wal", number))
}

func replaySegment(path string, isLast bool) ([]model.ResolvedPoint, error) {
	file, err := os.OpenFile(path, os.O_RDWR, storagefs.FileMode)
	if err != nil {
		return nil, fmt.Errorf("open wal segment for replay: %w", err)
	}
	points, replayErr := replayOpenSegment(file, isLast)
	closeErr := file.Close()
	if replayErr != nil {
		return nil, replayErr
	}
	if closeErr != nil {
		return nil, fmt.Errorf("close wal segment after replay: %w", closeErr)
	}
	return points, nil
}

func replayOpenSegment(file *os.File, isLast bool) ([]model.ResolvedPoint, error) {
	points := make([]model.ResolvedPoint, 0)
	for {
		offset, err := file.Seek(0, io.SeekCurrent)
		if err != nil {
			return nil, fmt.Errorf("seek wal segment: %w", err)
		}
		record, err := readFrame(file)
		if err == nil {
			points = append(points, record...)
			continue
		}
		if err == io.EOF {
			return points, nil
		}
		if isLast && isPartial(err) {
			if truncateErr := file.Truncate(offset); truncateErr != nil {
				return nil, fmt.Errorf("truncate partial wal record: %w", truncateErr)
			}
			return points, nil
		}
		return nil, err
	}
}

func readFrame(reader io.Reader) ([]model.ResolvedPoint, error) {
	header := make([]byte, 4)
	if _, err := io.ReadFull(reader, header); err != nil {
		return nil, normalizeReadError(err)
	}
	length := binary.BigEndian.Uint32(header)
	if length < 6 {
		return nil, fmt.Errorf("wal record length is too small")
	}
	body := make([]byte, length)
	if _, err := io.ReadFull(reader, body); err != nil {
		return nil, normalizeReadError(err)
	}
	if body[0] != recordVersion {
		return nil, fmt.Errorf("unsupported wal record version")
	}
	if body[1] != recordWriteBatch {
		return nil, fmt.Errorf("unsupported wal record type")
	}
	want := binary.BigEndian.Uint32(body[len(body)-4:])
	got := crc32.Checksum(body[:len(body)-4], castagnoliTable)
	if got != want {
		return nil, fmt.Errorf("wal record crc mismatch")
	}
	var records []model.ResolvedPoint
	if err := json.Unmarshal(body[2:len(body)-4], &records); err != nil {
		return nil, fmt.Errorf("decode wal payload: %w", err)
	}
	return records, nil
}

func normalizeReadError(err error) error {
	if err == io.EOF {
		return io.EOF
	}
	if err == io.ErrUnexpectedEOF {
		return err
	}
	return fmt.Errorf("read wal record: %w", err)
}

func isPartial(err error) bool {
	return err == io.ErrUnexpectedEOF
}
