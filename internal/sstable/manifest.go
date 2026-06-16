package sstable

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"codeberg.org/mts/mts/internal/codec"
	"codeberg.org/mts/mts/internal/storagefs"
)

var manifestMagic = codec.Magic("MTSMAN2")

func LoadManifest(dir string) (Manifest, error) {
	data, err := os.ReadFile(filepath.Join(dir, manifestFile))
	if err != nil {
		if os.IsNotExist(err) {
			return loadMissingManifest(dir)
		}
		return Manifest{}, fmt.Errorf("read manifest: %w", err)
	}
	manifest, err := decodeManifest(data)
	if err != nil {
		return Manifest{}, fmt.Errorf("decode manifest: %w", err)
	}
	return normalizeManifest(manifest), nil
}

func WriteManifest(dir string, manifest Manifest) error {
	if manifest.Parts == nil {
		manifest.Parts = []PartMeta{}
	}
	data, err := encodeManifest(manifest)
	if err != nil {
		return fmt.Errorf("encode manifest: %w", err)
	}
	if err := storagefs.WriteFileAtomic(filepath.Join(dir, manifestFile), data); err != nil {
		return fmt.Errorf("write manifest: %w", err)
	}
	return nil
}

func loadMissingManifest(_ string) (Manifest, error) {
	return Manifest{Parts: []PartMeta{}}, nil
}

func encodeManifest(manifest Manifest) ([]byte, error) {
	payload := binary.AppendUvarint(nil, uint64(len(manifest.Parts)))
	var err error
	for _, part := range manifest.Parts {
		payload, err = appendPartMeta(payload, part)
		if err != nil {
			return nil, err
		}
	}
	return codec.MarshalEnvelope(nil, manifestMagic, 0, payload), nil
}

func decodeManifest(data []byte) (Manifest, error) {
	env, err := codec.UnmarshalEnvelope(data, manifestMagic)
	if err != nil {
		return Manifest{}, err
	}
	reader := newBlockReader(env.Payload)
	count, err := reader.intCount("manifest part count")
	if err != nil {
		return Manifest{}, err
	}
	parts := make([]PartMeta, 0, count)
	for range count {
		part, err := readPartMeta(reader)
		if err != nil {
			return Manifest{}, err
		}
		parts = append(parts, part)
	}
	if err := reader.done("manifest"); err != nil {
		return Manifest{}, err
	}
	return Manifest{Parts: parts}, nil
}

func normalizeManifest(manifest Manifest) Manifest {
	sort.Slice(manifest.Parts, func(i int, j int) bool {
		if manifest.Parts[i].Level != manifest.Parts[j].Level {
			return manifest.Parts[i].Level < manifest.Parts[j].Level
		}
		return manifest.Parts[i].ID < manifest.Parts[j].ID
	})
	if manifest.Parts == nil {
		manifest.Parts = []PartMeta{}
	}
	return manifest
}
