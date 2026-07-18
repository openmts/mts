package sstable

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/openmts/mts/internal/storagefs"
)

const (
	packMagic     = "MTSPAK1"
	packMagicSize = 7
)

// packSectionOrder 定义 pack.bin 内逻辑组件顺序。
var packSectionOrder = []string{
	metaindexFile,
	indexFile,
	seriesIndexFile,
	timestampsFile,
	valuesFile,
	stringsFile,
}

type packSection struct {
	Name   string
	Offset int64
	Size   int64
}

// writePartPack 将 part 目录中的逻辑组件合并为 pack.bin，并返回逻辑组件尺寸。
func writePartPack(path string, sync bool) (map[string]int64, error) {
	clean := filepath.Clean(path)
	sections := make([]packSection, 0, len(packSectionOrder))
	payloads := make([][]byte, 0, len(packSectionOrder))
	sizes := make(map[string]int64, len(packSectionOrder)+1)
	sizes[metadataFile] = 0

	for _, name := range packSectionOrder {
		data, err := storagefs.ReadFile(filepath.Join(clean, name))
		if err != nil {
			return nil, fmt.Errorf("read pack section %s: %w", name, err)
		}
		sections = append(sections, packSection{Name: name, Size: int64(len(data))})
		payloads = append(payloads, data)
		sizes[name] = int64(len(data))
	}

	encoded, err := encodePartPack(sections, payloads)
	if err != nil {
		return nil, err
	}
	packPath := filepath.Join(clean, packFile)
	if err := storagefs.WriteFileAtomic(packPath, encoded); err != nil {
		return nil, fmt.Errorf("write pack.bin: %w", err)
	}
	if sync {
		file, err := storagefs.Open(packPath)
		if err != nil {
			return nil, fmt.Errorf("open pack.bin for sync: %w", err)
		}
		syncErr := storagefs.Sync(file)
		closeErr := file.Close()
		if syncErr != nil || closeErr != nil {
			return nil, errors.Join(fmt.Errorf("sync pack.bin: %w", syncErr), closeErr)
		}
	}
	// 中间逻辑文件仅写路径临时产物；pack 提交后用 os.Remove 清理，避免占用 durable
	// 故障注入配额（compact 清理旧 part 的 RemoveAll 才应记 maintenance）。
	for _, name := range packSectionOrder {
		_ = os.Remove(filepath.Join(clean, name))
	}
	return sizes, nil
}

func encodePartPack(sections []packSection, payloads [][]byte) ([]byte, error) {
	if len(sections) != len(payloads) {
		return nil, fmt.Errorf("pack section count mismatch")
	}
	// magic | section_count uvarint | (name_len uvarint | name | size uvarint)* | payloads
	header := make([]byte, 0, 64+len(sections)*24)
	header = append(header, packMagic...)
	header = binary.AppendUvarint(header, uint64(len(sections)))
	for _, section := range sections {
		header = binary.AppendUvarint(header, uint64(len(section.Name)))
		header = append(header, section.Name...)
		header = binary.AppendUvarint(header, uint64(section.Size))
	}
	total := len(header)
	for _, payload := range payloads {
		total += len(payload)
	}
	out := make([]byte, 0, total)
	out = append(out, header...)
	offset := int64(len(header))
	for i, payload := range payloads {
		sections[i].Offset = offset
		out = append(out, payload...)
		offset += int64(len(payload))
	}
	return out, nil
}

func openPartPack(path string) (*os.File, map[string]packSection, error) {
	file, err := storagefs.Open(filepath.Join(filepath.Clean(path), packFile))
	if err != nil {
		return nil, nil, fmt.Errorf("open pack.bin: %w", err)
	}
	sections, err := readPartPackSections(file)
	if err != nil {
		closeErr := file.Close()
		return nil, nil, errors.Join(err, closeErr)
	}
	return file, sections, nil
}

func readPartPackSections(file *os.File) (map[string]packSection, error) {
	info, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat pack.bin: %w", err)
	}
	if info.Size() < packMagicSize {
		return nil, fmt.Errorf("pack.bin too small")
	}
	// 先读完整文件头；section payload 可能很大，按需只解析 header。
	// header 上限：magic + count + 每 section 名称与 size，POC 组件少，读前 64KiB 足够。
	const maxHeaderProbe = 64 << 10
	probe := maxHeaderProbe
	if info.Size() < int64(probe) {
		probe = int(info.Size())
	}
	buf := make([]byte, probe)
	if _, err := file.ReadAt(buf, 0); err != nil && err != io.EOF {
		return nil, fmt.Errorf("read pack header: %w", err)
	}
	sections, headerSize, err := decodePartPackHeader(buf)
	if err != nil {
		return nil, err
	}
	offset := int64(headerSize)
	out := make(map[string]packSection, len(sections))
	for _, section := range sections {
		if section.Size < 0 {
			return nil, fmt.Errorf("pack section %s has negative size", section.Name)
		}
		if offset+section.Size > info.Size() {
			return nil, fmt.Errorf("pack section %s exceeds file size", section.Name)
		}
		section.Offset = offset
		out[section.Name] = section
		offset += section.Size
	}
	if offset != info.Size() {
		return nil, fmt.Errorf("pack payload size mismatch: got %d want %d", offset, info.Size())
	}
	return out, nil
}

func decodePartPackHeader(data []byte) ([]packSection, int, error) {
	if len(data) < packMagicSize {
		return nil, 0, fmt.Errorf("pack header too short")
	}
	if string(data[:packMagicSize]) != packMagic {
		return nil, 0, fmt.Errorf("invalid pack magic")
	}
	rest := data[packMagicSize:]
	count, n := binary.Uvarint(rest)
	if n <= 0 {
		return nil, 0, fmt.Errorf("invalid pack section count")
	}
	rest = rest[n:]
	consumed := packMagicSize + n
	sections := make([]packSection, 0, count)
	for range count {
		nameLen, n := binary.Uvarint(rest)
		if n <= 0 {
			return nil, 0, fmt.Errorf("invalid pack section name length")
		}
		rest = rest[n:]
		consumed += n
		if uint64(len(rest)) < nameLen {
			return nil, 0, fmt.Errorf("pack section name truncated")
		}
		name := string(rest[:nameLen])
		rest = rest[nameLen:]
		consumed += int(nameLen)
		size, n := binary.Uvarint(rest)
		if n <= 0 {
			return nil, 0, fmt.Errorf("invalid pack section size")
		}
		rest = rest[n:]
		consumed += n
		if size > uint64(^uint64(0)>>1) {
			return nil, 0, fmt.Errorf("pack section size overflows int64")
		}
		sections = append(sections, packSection{Name: name, Size: int64(size)})
	}
	return sections, consumed, nil
}

func packSectionFile(file *os.File, sections map[string]packSection, name string) (*sectionReader, error) {
	section, ok := sections[name]
	if !ok {
		return nil, fmt.Errorf("pack section %s missing", name)
	}
	return &sectionReader{file: file, base: section.Offset, size: section.Size}, nil
}

// sectionReader 将 pack 内逻辑组件暴露为基于相对 offset 的 ReadAt 视图。
type sectionReader struct {
	file *os.File
	base int64
	size int64
}

func (r *sectionReader) ReadAt(p []byte, off int64) (int, error) {
	if r == nil || r.file == nil {
		return 0, fmt.Errorf("nil section reader")
	}
	if off < 0 {
		return 0, fmt.Errorf("negative section offset")
	}
	if off >= r.size {
		return 0, io.EOF
	}
	max := r.size - off
	if int64(len(p)) > max {
		p = p[:max]
	}
	return r.file.ReadAt(p, r.base+off)
}

func (r *sectionReader) Size() int64 {
	if r == nil {
		return 0
	}
	return r.size
}
