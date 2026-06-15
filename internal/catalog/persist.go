package catalog

import (
	"bufio"
	"encoding/json"
	"fmt"
	"hash/crc32"
	"os"
	"path/filepath"

	"codeberg.org/mts/mts/internal/storagefs"
)

func (c *Catalog) snapshotPath() string {
	return filepath.Join(c.dir, "snapshot.json")
}

func (c *Catalog) walPath() string {
	return filepath.Join(c.dir, "catalog.wal")
}

func (c *Catalog) loadSnapshot() error {
	data, err := os.ReadFile(c.snapshotPath())
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read catalog snapshot: %w", err)
	}
	var snap snapshot
	if err := json.Unmarshal(data, &snap); err != nil {
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
	snap := snapshot{
		NextSeriesID: c.nextSeriesID,
		NextFieldID:  c.nextFieldID,
		Series:       make([]Series, 0, len(c.series)),
		Fields:       make([]Field, 0, len(c.fields)),
	}
	for _, series := range c.series {
		series.Tags = cloneTags(series.Tags)
		snap.Series = append(snap.Series, series)
	}
	for _, field := range c.fields {
		snap.Fields = append(snap.Fields, field)
	}
	data, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return fmt.Errorf("encode catalog snapshot: %w", err)
	}
	if err := storagefs.WriteFileAtomic(c.snapshotPath(), data); err != nil {
		return fmt.Errorf("write catalog snapshot: %w", err)
	}
	return nil
}

func (c *Catalog) replayWAL() error {
	file, err := os.Open(c.walPath())
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("open catalog wal for replay: %w", err)
	}
	defer func() {
		_ = file.Close()
	}()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		entry, err := decodeLine(scanner.Bytes())
		if err != nil {
			return err
		}
		c.applyEntry(entry)
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scan catalog wal: %w", err)
	}
	return nil
}

func (c *Catalog) appendEntryLocked(entry walEntry) error {
	payload, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("encode catalog wal entry: %w", err)
	}
	line := walLine{
		CRC:     crc32.Checksum(payload, crc32.MakeTable(crc32.Castagnoli)),
		Payload: payload,
	}
	encoded, err := json.Marshal(line)
	if err != nil {
		return fmt.Errorf("encode catalog wal line: %w", err)
	}
	encoded = append(encoded, '\n')
	if _, err := c.wal.Write(encoded); err != nil {
		return fmt.Errorf("write catalog wal: %w", err)
	}
	if err := c.wal.Sync(); err != nil {
		return fmt.Errorf("sync catalog wal: %w", err)
	}
	return nil
}

func decodeLine(line []byte) (walEntry, error) {
	var record walLine
	if err := json.Unmarshal(line, &record); err != nil {
		return walEntry{}, fmt.Errorf("decode catalog wal line: %w", err)
	}
	checksum := crc32.Checksum(record.Payload, crc32.MakeTable(crc32.Castagnoli))
	if checksum != record.CRC {
		return walEntry{}, fmt.Errorf("catalog wal crc mismatch")
	}
	var entry walEntry
	if err := json.Unmarshal(record.Payload, &entry); err != nil {
		return walEntry{}, fmt.Errorf("decode catalog wal payload: %w", err)
	}
	return entry, nil
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
