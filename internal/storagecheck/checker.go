package storagecheck

import (
	"encoding/binary"
	"hash/crc32"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/openmts/mts/internal/sstable"
)

type Severity string

const (
	SeverityInfo  Severity = "info"
	SeverityWarn  Severity = "warn"
	SeverityFatal Severity = "fatal"
)

type UnknownFilePolicy string

const (
	UnknownFileIgnore UnknownFilePolicy = "ignore"
	UnknownFileWarn   UnknownFilePolicy = "warn"
	UnknownFileFatal  UnknownFilePolicy = "fatal"
)

type Options struct {
	UnknownFiles UnknownFilePolicy
}

type Report struct {
	Root   string  `json:"root"`
	Issues []Issue `json:"issues"`
}

type Issue struct {
	Severity    Severity `json:"severity"`
	Path        string   `json:"path"`
	PartID      string   `json:"part_id,omitempty"`
	Level       int      `json:"level,omitempty"`
	MinTime     int64    `json:"min_time,omitempty"`
	MaxTime     int64    `json:"max_time,omitempty"`
	MinSeriesID uint64   `json:"min_series_id,omitempty"`
	MaxSeriesID uint64   `json:"max_series_id,omitempty"`
	Reason      string   `json:"reason"`
	Offset      int64    `json:"offset,omitempty"`
	BlockType   string   `json:"block_type,omitempty"`
	Error       string   `json:"error,omitempty"`
}

func Check(root string, opts Options) (Report, error) {
	clean := filepath.Clean(root)
	if opts.UnknownFiles == "" {
		opts.UnknownFiles = UnknownFileIgnore
	}
	report := Report{Root: clean}
	state := scanState{
		root:         clean,
		unknownFiles: opts.UnknownFiles,
		manifestDirs: make(map[string]sstable.Manifest),
		partDirs:     make(map[string]struct{}),
		referenced:   make(map[string]sstable.PartMeta),
	}
	if err := filepath.WalkDir(clean, state.visit); err != nil {
		return report, err
	}
	state.checkManifestRefs(&report)
	state.checkParts(&report)
	report.Issues = append(report.Issues, state.issues...)
	return report, nil
}

type scanState struct {
	root         string
	unknownFiles UnknownFilePolicy
	manifestDirs map[string]sstable.Manifest
	partDirs     map[string]struct{}
	referenced   map[string]sstable.PartMeta
	issues       []Issue
}

const (
	walSegmentMagic     = "MTSWAL2"
	walSegmentFormatID  = uint16(2)
	walSegmentHeaderLen = len(walSegmentMagic) + 2 + 2 + 4
)

var walCRCTable = crc32.MakeTable(crc32.Castagnoli)

func (s *scanState) visit(path string, entry fs.DirEntry, err error) error {
	if err != nil {
		return err
	}
	name := entry.Name()
	if entry.IsDir() {
		if isPartDir(path) {
			s.partDirs[path] = struct{}{}
		}
		if hasFile(path, "MANIFEST.bin") {
			s.loadManifest(path)
		}
		return nil
	}
	if isStorageWALSegmentName(name) {
		s.validateWALSegment(path)
		return nil
	}
	if isKnownStorageFile(name) {
		return nil
	}
	s.reportUnknownFile(path)
	return nil
}

func (s *scanState) validateWALSegment(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		s.issues = append(s.issues, Issue{Severity: SeverityFatal, Path: path, Reason: "wal segment read error", Error: err.Error()})
		return
	}
	if len(data) < walSegmentHeaderLen {
		s.issues = append(s.issues, Issue{Severity: SeverityFatal, Path: path, Reason: "wal segment format error", Error: "header truncated"})
		return
	}
	header := data[:walSegmentHeaderLen]
	if string(header[:len(walSegmentMagic)]) != walSegmentMagic {
		s.issues = append(s.issues, Issue{Severity: SeverityFatal, Path: path, Reason: "wal segment format error", Error: "magic mismatch"})
		return
	}
	formatID := binary.LittleEndian.Uint16(header[len(walSegmentMagic):])
	headerLen := binary.LittleEndian.Uint16(header[len(walSegmentMagic)+2:])
	want := binary.LittleEndian.Uint32(header[len(header)-4:])
	got := crc32.Checksum(header[:len(header)-4], walCRCTable)
	if formatID != walSegmentFormatID || headerLen != uint16(walSegmentHeaderLen) || got != want {
		s.issues = append(s.issues, Issue{Severity: SeverityFatal, Path: path, Reason: "wal segment format error", Error: "header checksum or format mismatch"})
	}
}

func (s *scanState) loadManifest(dir string) {
	manifest, err := sstable.LoadManifest(dir)
	if err != nil {
		s.issues = append(s.issues, Issue{
			Severity: SeverityFatal,
			Path:     filepath.Join(dir, "MANIFEST.bin"),
			Reason:   "manifest checksum or format error",
			Error:    err.Error(),
		})
		return
	}
	s.manifestDirs[dir] = manifest
	for _, part := range manifest.Parts {
		path := part.Path
		if path == "" {
			path = filepath.Join(dir, part.ID)
			part.Path = path
		}
		s.referenced[path] = part
	}
}

func (s *scanState) checkManifestRefs(report *Report) {
	for path, meta := range s.referenced {
		info, err := os.Stat(path)
		if err != nil {
			report.Issues = append(report.Issues, issueFromPart(meta, SeverityFatal, path, "manifest references missing part", err))
			continue
		}
		if !info.IsDir() {
			report.Issues = append(report.Issues, issueFromPart(meta, SeverityFatal, path, "manifest references non-directory part", nil))
		}
	}
}

func (s *scanState) checkParts(report *Report) {
	for path := range s.partDirs {
		meta, referenced := s.referenced[path]
		if !referenced {
			meta = s.readPartMeta(path)
			report.Issues = append(report.Issues, issueFromPart(meta, SeverityWarn, path, "orphan part", nil))
		}
		part, err := sstable.OpenPart(path)
		if err != nil {
			report.Issues = append(report.Issues, issueFromPart(meta, SeverityFatal, path, "open part failed", err))
			continue
		}
		closeErr := part.Close()
		if closeErr != nil {
			report.Issues = append(report.Issues, issueFromPart(part.Meta(), SeverityWarn, path, "close part failed", closeErr))
		}
	}
}

func (s *scanState) readPartMeta(path string) sstable.PartMeta {
	part, err := sstable.OpenPart(path)
	if err != nil {
		return sstable.PartMeta{Path: path}
	}
	meta := part.Meta()
	if closeErr := part.Close(); closeErr != nil {
		s.issues = append(s.issues, issueFromPart(meta, SeverityWarn, path, "close part failed", closeErr))
	}
	return meta
}

func (s *scanState) reportUnknownFile(path string) {
	switch s.unknownFiles {
	case UnknownFileIgnore:
		return
	case UnknownFileFatal:
		s.issues = append(s.issues, Issue{Severity: SeverityFatal, Path: path, Reason: "unknown file"})
	default:
		s.issues = append(s.issues, Issue{Severity: SeverityWarn, Path: path, Reason: "unknown file"})
	}
}

func issueFromPart(meta sstable.PartMeta, severity Severity, path string, reason string, err error) Issue {
	issue := Issue{
		Severity:    severity,
		Path:        path,
		PartID:      meta.ID,
		Level:       meta.Level,
		MinTime:     meta.MinTime,
		MaxTime:     meta.MaxTime,
		MinSeriesID: meta.MinSeriesID,
		MaxSeriesID: meta.MaxSeriesID,
		Reason:      reason,
	}
	if err != nil {
		issue.Error = err.Error()
		issue.Offset, issue.BlockType = extractBlockLocation(err.Error())
	}
	return issue
}

func extractBlockLocation(message string) (int64, string) {
	if offset, ok := parseOffset(message); ok {
		return offset, "block"
	}
	if strings.Contains(message, "value page") {
		return 0, "value_page"
	}
	if strings.Contains(message, "value index") {
		return 0, "value_index"
	}
	if strings.Contains(message, "time block") {
		return 0, "time_block"
	}
	if strings.Contains(message, "index") {
		return 0, "index"
	}
	return 0, ""
}

func parseOffset(message string) (int64, bool) {
	const marker = "offset="
	start := strings.Index(message, marker)
	if start < 0 {
		return 0, false
	}
	start += len(marker)
	end := start
	for end < len(message) && message[end] >= '0' && message[end] <= '9' {
		end++
	}
	if end == start {
		return 0, false
	}
	offset, err := strconv.ParseInt(message[start:end], 10, 64)
	return offset, err == nil
}

func hasFile(dir string, name string) bool {
	info, err := os.Stat(filepath.Join(dir, name))
	return err == nil && !info.IsDir()
}

func isPartDir(dir string) bool {
	if !hasFile(dir, "metadata.bin") {
		return false
	}
	if hasFile(dir, "pack.bin") {
		return true
	}
	for _, name := range []string{
		"metaindex.bin",
		"index.bin",
		"series_index.bin",
		"timestamps.bin",
		"values.bin",
		"strings.bin",
	} {
		if hasFile(dir, name) {
			return true
		}
	}
	return false
}

func isStorageWALSegmentName(name string) bool {
	if len(name) != len("000001.wal") || !strings.HasSuffix(name, ".wal") {
		return false
	}
	for _, digit := range name[:6] {
		if digit < '0' || digit > '9' {
			return false
		}
	}
	return true
}

func isKnownStorageFile(name string) bool {
	switch name {
	case "MANIFEST.bin",
		"metadata.bin",
		"pack.bin",
		"metaindex.bin",
		"index.bin",
		"series_index.bin",
		"timestamps.bin",
		"values.bin",
		"strings.bin":
		return true
	default:
		return strings.HasSuffix(name, ".wal")
	}
}
