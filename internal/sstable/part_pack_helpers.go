package sstable

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/openmts/mts/internal/storagefs"
)

// logicalComponentSize 返回逻辑组件字节数（pack section 或独立文件）。
func PartLogicalComponentSize(path string, name string) (int64, error) {
	if name == metadataFile {
		info, err := storagefs.Stat(filepath.Join(path, metadataFile))
		if err != nil {
			return 0, err
		}
		return info.Size(), nil
	}
	packPath := filepath.Join(filepath.Clean(path), packFile)
	if _, err := storagefs.Stat(packPath); err == nil {
		file, sections, err := openPartPack(path)
		if err != nil {
			return 0, err
		}
		defer func() { _ = file.Close() }()
		section, ok := sections[name]
		if !ok {
			return 0, fmt.Errorf("pack section %s missing", name)
		}
		return section.Size, nil
	}
	info, err := storagefs.Stat(filepath.Join(path, name))
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}

// overwriteLogicalComponentAt 在逻辑组件相对 offset 写入数据（用于测试损坏场景）。
func OverwriteLogicalComponentAt(path string, name string, offset int64, data []byte) error {
	packPath := filepath.Join(filepath.Clean(path), packFile)
	if _, err := storagefs.Stat(packPath); err == nil {
		file, sections, err := openPartPack(path)
		if err != nil {
			return err
		}
		section, ok := sections[name]
		if !ok {
			_ = file.Close()
			return fmt.Errorf("pack section %s missing", name)
		}
		// open writable pack
		_ = file.Close()
		writable, err := storagefs.OpenFile(packPath, os.O_RDWR, storagefs.FileMode)
		if err != nil {
			return err
		}
		if _, err := writable.WriteAt(data, section.Offset+offset); err != nil {
			closeErr := writable.Close()
			return fmt.Errorf("write pack section: %w close: %v", err, closeErr)
		}
		return writable.Close()
	}
	writable, err := storagefs.OpenFile(filepath.Join(path, name), os.O_RDWR, storagefs.FileMode)
	if err != nil {
		return err
	}
	if _, err := writable.WriteAt(data, offset); err != nil {
		closeErr := writable.Close()
		return fmt.Errorf("write component: %w close: %v", err, closeErr)
	}
	return writable.Close()
}

// removeLogicalComponent 删除逻辑组件（pack 场景下移除 section 并重写 pack）。
func RemoveLogicalComponent(path string, name string) error {
	if name == metadataFile {
		return storagefs.Remove(filepath.Join(path, metadataFile))
	}
	packPath := filepath.Join(filepath.Clean(path), packFile)
	if _, err := storagefs.Stat(packPath); err == nil {
		file, sections, err := openPartPack(path)
		if err != nil {
			return err
		}
		defer func() { _ = file.Close() }()
		if _, ok := sections[name]; !ok {
			return fmt.Errorf("pack section %s missing", name)
		}
		ordered := make([]packSection, 0, len(packSectionOrder))
		payloads := make([][]byte, 0, len(packSectionOrder))
		for _, sectionName := range packSectionOrder {
			if sectionName == name {
				continue
			}
			section, ok := sections[sectionName]
			if !ok {
				return fmt.Errorf("pack section %s missing", sectionName)
			}
			buf := make([]byte, section.Size)
			if section.Size > 0 {
				if _, err := file.ReadAt(buf, section.Offset); err != nil {
					return err
				}
			}
			ordered = append(ordered, packSection{Name: sectionName, Size: section.Size})
			payloads = append(payloads, buf)
		}
		encoded, err := encodePartPack(ordered, payloads)
		if err != nil {
			return err
		}
		return storagefs.WriteFileAtomic(packPath, encoded)
	}
	return storagefs.Remove(filepath.Join(path, name))
}
