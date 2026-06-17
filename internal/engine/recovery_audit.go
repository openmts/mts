package engine

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"codeberg.org/mts/mts/internal/sstable"
)

var ErrRecoveryFatal = errors.New("recovery fatal")

type RecoveryIssueKind string

const (
	RecoveryIssueMissingPart        RecoveryIssueKind = "missing_part"
	RecoveryIssuePartOpenFailed     RecoveryIssueKind = "part_open_failed"
	RecoveryIssueMetadataMismatch   RecoveryIssueKind = "metadata_mismatch"
	RecoveryIssueOrphanPartRemoved  RecoveryIssueKind = "orphan_part_removed"
	RecoveryIssueOrphanRemoveFailed RecoveryIssueKind = "orphan_remove_failed"
	RecoveryIssueTempRemoved        RecoveryIssueKind = "temp_removed"
	RecoveryIssueTempRemoveFailed   RecoveryIssueKind = "temp_remove_failed"
	RecoveryIssueCleanupScanFailed  RecoveryIssueKind = "cleanup_scan_failed"
)

type RecoveryIssue struct {
	Kind    RecoveryIssueKind
	Path    string
	Message string
	Err     error
	Fatal   bool
}

func (i *RecoveryIssue) Error() string {
	if i == nil {
		return "<nil>"
	}
	message := i.Message
	if message == "" {
		message = string(i.Kind)
	}
	if i.Path == "" {
		return message
	}
	return fmt.Sprintf("%s: %s", message, i.Path)
}

func (i *RecoveryIssue) Unwrap() []error {
	if i == nil {
		return nil
	}
	if i.Fatal && i.Err != nil {
		return []error{ErrRecoveryFatal, i.Err}
	}
	if i.Fatal {
		return []error{ErrRecoveryFatal}
	}
	if i.Err != nil {
		return []error{i.Err}
	}
	return nil
}

func (i *RecoveryIssue) Is(target error) bool {
	return target == ErrRecoveryFatal && i != nil && i.Fatal
}

type RecoveryReport struct {
	Issues []RecoveryIssue
}

func (r *RecoveryReport) Add(issue RecoveryIssue) {
	r.Issues = append(r.Issues, issue)
}

func (r *RecoveryReport) Merge(other RecoveryReport) {
	r.Issues = append(r.Issues, other.Issues...)
}

func (r RecoveryReport) FatalError() error {
	return r.joinIssues(true)
}

func (r RecoveryReport) MaintenanceError() error {
	return r.joinIssues(false)
}

func (r RecoveryReport) Clone() RecoveryReport {
	return RecoveryReport{Issues: append([]RecoveryIssue{}, r.Issues...)}
}

func (r RecoveryReport) joinIssues(fatal bool) error {
	var joined error
	for index := range r.Issues {
		if r.Issues[index].Fatal != fatal {
			continue
		}
		joined = errors.Join(joined, &r.Issues[index])
	}
	return joined
}

func partOpenRecoveryIssue(meta sstable.PartMeta, err error) RecoveryIssue {
	kind := RecoveryIssuePartOpenFailed
	message := "open manifest part failed"
	if errors.Is(err, os.ErrNotExist) {
		kind = RecoveryIssueMissingPart
		message = "manifest references missing part"
	}
	return RecoveryIssue{Kind: kind, Path: meta.Path, Message: message, Err: err, Fatal: true}
}

func partMetadataMismatchIssue(expected sstable.PartMeta, actual sstable.PartMeta) (RecoveryIssue, bool) {
	mismatches := partMetaMismatches(expected, actual)
	if len(mismatches) == 0 {
		return RecoveryIssue{}, false
	}
	return RecoveryIssue{
		Kind:    RecoveryIssueMetadataMismatch,
		Path:    expected.Path,
		Message: "part metadata mismatch: " + strings.Join(mismatches, ","),
		Fatal:   true,
	}, true
}

func partMetaMismatches(expected sstable.PartMeta, actual sstable.PartMeta) []string {
	mismatches := make([]string, 0, 10)
	compareString(&mismatches, "id", expected.ID, actual.ID)
	compareInt(&mismatches, "level", expected.Level, actual.Level)
	compareInt64(&mismatches, "min_time", expected.MinTime, actual.MinTime)
	compareInt64(&mismatches, "max_time", expected.MaxTime, actual.MaxTime)
	compareUint64(&mismatches, "min_series_id", expected.MinSeriesID, actual.MinSeriesID)
	compareUint64(&mismatches, "max_series_id", expected.MaxSeriesID, actual.MaxSeriesID)
	compareInt(&mismatches, "rows_count", expected.RowsCount, actual.RowsCount)
	compareInt(&mismatches, "series_count", expected.SeriesCount, actual.SeriesCount)
	compareInt(&mismatches, "block_count", expected.BlockCount, actual.BlockCount)
	compareUint64(&mismatches, "max_write_seq", expected.MaxWriteSeq, actual.MaxWriteSeq)
	return mismatches
}

func compareString(out *[]string, name string, expected string, actual string) {
	if expected != actual {
		*out = append(*out, name)
	}
}

func compareInt(out *[]string, name string, expected int, actual int) {
	if expected != actual {
		*out = append(*out, name)
	}
}

func compareInt64(out *[]string, name string, expected int64, actual int64) {
	if expected != actual {
		*out = append(*out, name)
	}
}

func compareUint64(out *[]string, name string, expected uint64, actual uint64) {
	if expected != actual {
		*out = append(*out, name)
	}
}
