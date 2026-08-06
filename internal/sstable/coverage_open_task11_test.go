package sstable

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/openmts/mts/internal/faultinject"
	"github.com/openmts/mts/internal/model"
	"github.com/openmts/mts/internal/storagefs"
)

func TestOpenPartReadFilesFromPackRejectsMissingSections(t *testing.T) {
	cases := []struct {
		name     string
		sections []string
	}{
		{name: "index", sections: nil},
		{name: "timestamps", sections: []string{indexFile}},
		{name: "values", sections: []string{indexFile, timestampsFile}},
	}
	for _, item := range cases {
		t.Run(item.name, func(t *testing.T) {
			dir := t.TempDir()
			sections := make([]packSection, 0, len(item.sections))
			payloads := make([][]byte, 0, len(item.sections))
			for _, name := range item.sections {
				sections = append(sections, packSection{Name: name, Size: 1})
				payloads = append(payloads, []byte{1})
			}
			encoded, err := encodePartPack(sections, payloads)
			if err != nil {
				t.Fatalf("encodePartPack() error = %v", err)
			}
			writeCoverageFile(t, filepath.Join(dir, packFile), encoded)
			if _, err := openPartReadFilesFromPack(dir); err == nil {
				t.Fatal("openPartReadFilesFromPack() error = nil")
			}
		})
	}

	corruptDir := t.TempDir()
	writeCoverageFile(t, filepath.Join(corruptDir, packFile), []byte("corrupt"))
	if _, err := openPartReadFilesFromPack(corruptDir); err == nil {
		t.Fatal("openPartReadFilesFromPack(corrupt) error = nil")
	}
}

func TestPartWriterPropagatesStorageFaults(t *testing.T) {
	createFS := faultinject.NewFS()
	createFS.FailNext(faultinject.OpCreate, os.ErrPermission)
	restore := storagefs.SetFaultController(createFS)
	_, err := NewPartWriter(t.TempDir(), 0, "create-error", WriteOptions{})
	restore()
	if err == nil || !errors.Is(err, os.ErrPermission) {
		t.Fatalf("NewPartWriter(create fault) error = %v", err)
	}

	writeWriter, err := NewPartWriter(t.TempDir(), 0, "write-error", WriteOptions{})
	if err != nil {
		t.Fatalf("NewPartWriter(write fault) error = %v", err)
	}
	writeFS := faultinject.NewFS()
	writeFS.FailNext(faultinject.OpWrite, os.ErrPermission)
	restore = storagefs.SetFaultController(writeFS)
	err = writeWriter.AddSeries([]model.ColumnData{
		columnWithField(1, 1, model.Int64Value(1)),
	})
	restore()
	if err == nil || !errors.Is(err, os.ErrPermission) {
		abortErr := writeWriter.Abort()
		t.Fatalf("AddSeries(write fault) error = %v abort = %v", err, abortErr)
	}
	if err := writeWriter.Abort(); err != nil {
		t.Fatalf("Abort(write fault) error = %v", err)
	}

	closeWriter, err := NewPartWriter(t.TempDir(), 0, "close-error", WriteOptions{})
	if err != nil {
		t.Fatalf("NewPartWriter(close fault) error = %v", err)
	}
	if err := closeWriter.AddSeries([]model.ColumnData{
		columnWithField(1, 1, model.Int64Value(1)),
	}); err != nil {
		abortErr := closeWriter.Abort()
		t.Fatalf("AddSeries(close fixture) error = %v abort = %v", err, abortErr)
	}
	closeFS := faultinject.NewFS()
	closeFS.FailNext(faultinject.OpCreate, os.ErrPermission)
	restore = storagefs.SetFaultController(closeFS)
	_, err = closeWriter.Close()
	restore()
	if err == nil || !errors.Is(err, os.ErrPermission) {
		t.Fatalf("Close(index create fault) error = %v", err)
	}
}
