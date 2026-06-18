package sstable

import (
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/openmts/mts/internal/codec"
	"github.com/openmts/mts/internal/storagefs"
)

var manifestMagic = codec.Magic("MTSMAN2")

const manifestFlagSequence uint16 = 1

func LoadManifest(dir string) (Manifest, error) {
	data, err := storagefs.ReadFile(filepath.Join(dir, manifestFile))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
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

func LoadManifestStrict(dir string, previousSequence uint64) (Manifest, error) {
	manifest, err := LoadManifest(dir)
	if err != nil {
		return Manifest{}, err
	}
	if manifest.Sequence < previousSequence {
		return Manifest{}, fmt.Errorf(
			"manifest sequence regressed: got %d previous %d",
			manifest.Sequence,
			previousSequence,
		)
	}
	if err := validateManifestPartRefs(dir, manifest.Parts); err != nil {
		return Manifest{}, err
	}
	return manifest, nil
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
	payload := binary.AppendUvarint(nil, manifest.Sequence)
	payload = binary.AppendUvarint(payload, uint64(len(manifest.Parts)))
	var err error
	for _, part := range manifest.Parts {
		payload, err = appendPartMeta(payload, part)
		if err != nil {
			return nil, err
		}
	}
	return codec.MarshalEnvelope(nil, manifestMagic, manifestFlagSequence, payload), nil
}

func decodeManifest(data []byte) (Manifest, error) {
	env, err := codec.UnmarshalEnvelope(data, manifestMagic)
	if err != nil {
		return Manifest{}, err
	}
	reader := newBlockReader(env.Payload)
	var sequence uint64
	if env.Flags&manifestFlagSequence != 0 {
		sequence, err = reader.uvarint("manifest sequence")
		if err != nil {
			return Manifest{}, err
		}
	}
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
	return Manifest{Sequence: sequence, Parts: parts}, nil
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

func validateManifestPartRefs(dir string, parts []PartMeta) error {
	for _, part := range parts {
		path := part.Path
		if path == "" {
			path = filepath.Join(dir, part.ID)
		}
		info, err := storagefs.Stat(path)
		if err != nil {
			return fmt.Errorf("manifest references missing part id=%s level=%d path=%s: %w",
				part.ID, part.Level, path, err)
		}
		if !info.IsDir() {
			return fmt.Errorf("manifest references non-directory part id=%s level=%d path=%s",
				part.ID, part.Level, path)
		}
	}
	return nil
}
