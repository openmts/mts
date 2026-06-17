package wal

import (
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"io"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"codeberg.org/mts/mts/internal/model"
	"codeberg.org/mts/mts/internal/storagefs"
)

const (
	defaultSegmentBytes int64 = 64 << 20
	recordWriteBatch    byte  = 1
	recordTombstone     byte  = 2
)

const (
	walSegmentMagic     = "MTSWAL2"
	walSegmentFormatID  = uint16(1)
	walSegmentHeaderLen = len(walSegmentMagic) + 2 + 2 + 4
)

var castagnoliTable = crc32.MakeTable(crc32.Castagnoli)

type Options struct {
	Sync          bool
	SegmentBytes  int64
	BatchRecords  int
	BatchBytes    int64
	BatchInterval time.Duration
}

type Log struct {
	mu sync.Mutex

	dir  string
	opts Options

	file    *os.File
	segment int
	size    int64

	pendingRecords int
	pendingBytes   int64

	syncStopOnce sync.Once
	syncStop     chan struct{}
	syncWG       sync.WaitGroup
}

type Record struct {
	Points     []model.ResolvedPoint
	Tombstones []model.Tombstone
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
	log.startIntervalSync()
	return log, nil
}

func (l *Log) Append(records []model.ResolvedPoint, syncWrite bool) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	payload, err := encodeBatch(records)
	if err != nil {
		return fmt.Errorf("encode wal record: %w", err)
	}
	return l.appendFrameLocked(recordWriteBatch, payload, syncWrite)
}

func (l *Log) appendFrameLocked(recordType byte, payload []byte, syncWrite bool) error {
	frame := encodeFrame(recordType, payload)
	if err := l.rollIfNeeded(int64(len(frame))); err != nil {
		return err
	}
	if err := storagefs.WriteFull(l.file, frame); err != nil {
		return fmt.Errorf("write wal record: %w", err)
	}
	l.size += int64(len(frame))
	l.pendingRecords++
	l.pendingBytes += int64(len(frame))
	if l.shouldSync(syncWrite) {
		if err := storagefs.Sync(l.file); err != nil {
			return fmt.Errorf("sync wal: %w", err)
		}
		l.pendingRecords = 0
		l.pendingBytes = 0
	}
	return nil
}

func (l *Log) AppendTombstones(tombstones []model.Tombstone, syncWrite bool) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	payload, err := encodeTombstones(tombstones)
	if err != nil {
		return fmt.Errorf("encode wal tombstone: %w", err)
	}
	return l.appendFrameLocked(recordTombstone, payload, syncWrite)
}

func (l *Log) Replay() ([]model.ResolvedPoint, error) {
	records, err := l.ReplayRecords()
	if err != nil {
		return nil, err
	}
	points := make([]model.ResolvedPoint, 0)
	for _, record := range records {
		points = append(points, record.Points...)
	}
	return points, nil
}

func (l *Log) ReplayRecords() ([]Record, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	segments, err := listSegments(l.dir)
	if err != nil {
		return nil, err
	}
	records := make([]Record, 0)
	for index, segment := range segments {
		isLast := index == len(segments)-1
		replayed, err := replaySegment(segment.path, isLast)
		if err != nil {
			return nil, err
		}
		records = append(records, replayed...)
	}
	return records, nil
}

func (l *Log) ApproxMemoryBytes() int64 {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.pendingBytes + int64(l.pendingRecords)*16
}

func (l *Log) TruncateAll() error {
	l.mu.Lock()
	defer l.mu.Unlock()
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
		if err := storagefs.Remove(segment.path); err != nil {
			return fmt.Errorf("remove wal segment: %w", err)
		}
	}
	l.segment = 1
	l.size = 0
	return l.openSegment(1)
}

func (l *Log) Close() error {
	l.stopIntervalSync()
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.file == nil {
		return nil
	}
	if err := l.file.Close(); err != nil {
		return fmt.Errorf("close wal: %w", err)
	}
	l.file = nil
	return nil
}

func (l *Log) Checkpoint() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if err := l.syncPendingLocked(); err != nil {
		return err
	}
	oldSegment := l.segment
	if l.file != nil {
		if err := l.file.Close(); err != nil {
			return fmt.Errorf("close wal before checkpoint: %w", err)
		}
		l.file = nil
	}
	if err := l.openSegment(oldSegment + 1); err != nil {
		return err
	}
	segments, err := listSegments(l.dir)
	if err != nil {
		return err
	}
	for _, segment := range segments {
		if segment.number > oldSegment {
			continue
		}
		if err := storagefs.Remove(segment.path); err != nil {
			return fmt.Errorf("remove checkpointed wal segment: %w", err)
		}
	}
	return nil
}

func (l *Log) FlushPending() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.syncPendingLocked()
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
	file, err := storagefs.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_RDWR, storagefs.FileMode)
	if err != nil {
		return fmt.Errorf("open wal segment: %w", err)
	}
	info, err := file.Stat()
	if err != nil {
		closeErr := file.Close()
		return fmt.Errorf("stat wal segment: %w close: %v", err, closeErr)
	}
	size := info.Size()
	if size == 0 {
		if err := writeSegmentHeader(file); err != nil {
			closeErr := file.Close()
			return fmt.Errorf("write wal segment header: %w close: %v", err, closeErr)
		}
		size = int64(walSegmentHeaderLen)
	}
	l.file = file
	l.segment = number
	l.size = size
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

func (l *Log) syncPendingLocked() error {
	if l.file == nil || l.pendingRecords == 0 {
		return nil
	}
	if err := storagefs.Sync(l.file); err != nil {
		return fmt.Errorf("sync wal: %w", err)
	}
	l.pendingRecords = 0
	l.pendingBytes = 0
	return nil
}

func (l *Log) startIntervalSync() {
	if l.opts.BatchInterval <= 0 {
		return
	}
	l.syncStop = make(chan struct{})
	l.syncWG.Add(1)
	go l.intervalSyncLoop(l.opts.BatchInterval)
}

func (l *Log) stopIntervalSync() {
	if l.syncStop == nil {
		return
	}
	l.syncStopOnce.Do(func() {
		close(l.syncStop)
	})
	l.syncWG.Wait()
}

func (l *Log) intervalSyncLoop(interval time.Duration) {
	defer l.syncWG.Done()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			_ = l.FlushPending()
		case <-l.syncStop:
			return
		}
	}
}

func encodeFrame(recordType byte, payload []byte) []byte {
	bodyLen := 1 + len(payload) + 4
	frame := make([]byte, 4+bodyLen)
	binary.BigEndian.PutUint32(frame[:4], uint32(bodyLen))
	frame[4] = recordType
	copy(frame[5:], payload)
	checksum := crc32.Checksum(frame[4:4+bodyLen-4], castagnoliTable)
	binary.BigEndian.PutUint32(frame[4+bodyLen-4:], checksum)
	return frame
}

type segmentInfo struct {
	number int
	path   string
}

func listSegments(dir string) ([]segmentInfo, error) {
	entries, err := storagefs.ReadDir(dir)
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

func replaySegment(path string, isLast bool) ([]Record, error) {
	file, err := storagefs.OpenFile(path, os.O_RDWR, storagefs.FileMode)
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

func replayOpenSegment(file *os.File, isLast bool) ([]Record, error) {
	records := make([]Record, 0)
	if err := readSegmentHeader(file); err != nil {
		return nil, err
	}
	for {
		offset, err := file.Seek(0, io.SeekCurrent)
		if err != nil {
			return nil, fmt.Errorf("seek wal segment: %w", err)
		}
		record, err := readFrame(file)
		if err == nil {
			records = append(records, record)
			continue
		}
		if err == io.EOF {
			return records, nil
		}
		if isLast && isPartial(err) {
			if truncateErr := file.Truncate(offset); truncateErr != nil {
				return nil, fmt.Errorf("truncate partial wal record: %w", truncateErr)
			}
			return records, nil
		}
		return nil, err
	}
}

func readFrame(reader io.Reader) (Record, error) {
	header := make([]byte, 4)
	if _, err := io.ReadFull(reader, header); err != nil {
		return Record{}, normalizeReadError(err)
	}
	length := binary.BigEndian.Uint32(header)
	if length < 5 {
		return Record{}, fmt.Errorf("wal record length is too small")
	}
	body := make([]byte, length)
	if _, err := io.ReadFull(reader, body); err != nil {
		return Record{}, normalizeReadError(err)
	}
	want := binary.BigEndian.Uint32(body[len(body)-4:])
	got := crc32.Checksum(body[:len(body)-4], castagnoliTable)
	if got != want {
		return Record{}, fmt.Errorf("wal record crc mismatch")
	}
	record, err := decodeFramePayload(body[0], body[1:len(body)-4])
	if err != nil {
		return Record{}, fmt.Errorf("decode wal payload: %w", err)
	}
	return record, nil
}

func appendSegmentHeader(dst []byte) []byte {
	start := len(dst)
	dst = append(dst, walSegmentMagic...)
	dst = binary.LittleEndian.AppendUint16(dst, walSegmentFormatID)
	dst = binary.LittleEndian.AppendUint16(dst, uint16(walSegmentHeaderLen))
	sum := crc32.Checksum(dst[start:], castagnoliTable)
	return binary.LittleEndian.AppendUint32(dst, sum)
}

func writeSegmentHeader(file *os.File) error {
	return storagefs.WriteFull(file, appendSegmentHeader(nil))
}

func readSegmentHeader(reader io.Reader) error {
	header := make([]byte, walSegmentHeaderLen)
	if _, err := io.ReadFull(reader, header); err != nil {
		return normalizeReadError(err)
	}
	if string(header[:len(walSegmentMagic)]) != walSegmentMagic {
		return fmt.Errorf("wal segment magic mismatch")
	}
	formatID := binary.LittleEndian.Uint16(header[len(walSegmentMagic):])
	if formatID != walSegmentFormatID {
		return fmt.Errorf("wal segment format id %d is unsupported", formatID)
	}
	headerLenOffset := len(walSegmentMagic) + 2
	headerLen := binary.LittleEndian.Uint16(header[headerLenOffset:])
	if headerLen != uint16(walSegmentHeaderLen) {
		return fmt.Errorf("wal segment header length %d is invalid", headerLen)
	}
	want := binary.LittleEndian.Uint32(header[len(header)-4:])
	got := crc32.Checksum(header[:len(header)-4], castagnoliTable)
	if got != want {
		return fmt.Errorf("wal segment header crc mismatch")
	}
	return nil
}

func decodeFramePayload(recordType byte, payload []byte) (Record, error) {
	switch recordType {
	case recordWriteBatch:
		points, err := decodeBatch(payload)
		return Record{Points: points}, err
	case recordTombstone:
		tombstones, err := decodeTombstones(payload)
		return Record{Tombstones: tombstones}, err
	default:
		return Record{}, fmt.Errorf("unsupported wal record type")
	}
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
