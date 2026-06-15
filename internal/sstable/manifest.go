package sstable

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"codeberg.org/mts/mts/internal/storagefs"
)

func LoadManifest(dir string) (Manifest, error) {
	data, err := os.ReadFile(filepath.Join(dir, manifestFile))
	if err != nil {
		if os.IsNotExist(err) {
			return Manifest{Parts: []PartMeta{}}, nil
		}
		return Manifest{}, fmt.Errorf("read manifest: %w", err)
	}
	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return Manifest{}, fmt.Errorf("decode manifest: %w", err)
	}
	sort.Slice(manifest.Parts, func(i, j int) bool {
		if manifest.Parts[i].Level != manifest.Parts[j].Level {
			return manifest.Parts[i].Level < manifest.Parts[j].Level
		}
		return manifest.Parts[i].ID < manifest.Parts[j].ID
	})
	if manifest.Parts == nil {
		manifest.Parts = []PartMeta{}
	}
	return manifest, nil
}

func WriteManifest(dir string, manifest Manifest) error {
	if manifest.Parts == nil {
		manifest.Parts = []PartMeta{}
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("encode manifest: %w", err)
	}
	if err := storagefs.WriteFileAtomic(filepath.Join(dir, manifestFile), data); err != nil {
		return fmt.Errorf("write manifest: %w", err)
	}
	return nil
}
