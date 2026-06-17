package sstable

import (
	"encoding/binary"
	"fmt"
)

const (
	lz4MinMatch     = 4
	lz4HashLog      = 16
	lz4HashSize     = 1 << lz4HashLog
	lz4HashShift    = 32 - lz4HashLog
	lz4MaxOffset    = 1 << 16
	lz4LastLiterals = 5
)

func encodeLZ4Block(dst []byte, src []byte) []byte {
	if len(src) < lz4MinMatch+lz4LastLiterals {
		return appendLZ4Literals(dst, src)
	}
	table := make([]int, lz4HashSize)
	anchor := 0
	index := 0
	limit := len(src) - lz4MinMatch
	for index <= limit {
		sequence := binary.LittleEndian.Uint32(src[index:])
		slot := lz4Hash(sequence)
		ref := table[slot] - 1
		table[slot] = index + 1
		if !validLZ4Match(src, index, ref) {
			index++
			continue
		}
		matchLen := lz4MinMatch + countLZ4Match(src, ref+lz4MinMatch, index+lz4MinMatch)
		dst = appendLZ4Sequence(dst, src[anchor:index], index-ref, matchLen)
		index += matchLen
		anchor = index
		if index > limit {
			break
		}
		table[lz4Hash(binary.LittleEndian.Uint32(src[index-2:]))] = index - 1
	}
	return appendLZ4Literals(dst, src[anchor:])
}

func decodeLZ4Block(dst []byte, src []byte, rawSize int) ([]byte, error) {
	reader := src
	for len(reader) > 0 {
		token := reader[0]
		reader = reader[1:]
		literalLen, next, err := readLZ4Length(reader, int(token>>4))
		if err != nil {
			return nil, fmt.Errorf("read literal length: %w", err)
		}
		reader = next
		if literalLen > len(reader) {
			return nil, fmt.Errorf("literal length %d exceeds remaining %d", literalLen, len(reader))
		}
		dst = append(dst, reader[:literalLen]...)
		reader = reader[literalLen:]
		if len(reader) == 0 {
			break
		}
		if len(reader) < 2 {
			return nil, fmt.Errorf("truncated match offset")
		}
		offset := int(binary.LittleEndian.Uint16(reader))
		reader = reader[2:]
		if offset == 0 || offset > len(dst) {
			return nil, fmt.Errorf("invalid match offset %d", offset)
		}
		matchLen, rest, err := readLZ4Length(reader, int(token&0x0f))
		if err != nil {
			return nil, fmt.Errorf("read match length: %w", err)
		}
		reader = rest
		if err := appendLZ4Match(&dst, offset, matchLen+lz4MinMatch, rawSize); err != nil {
			return nil, err
		}
	}
	return dst, nil
}

func validLZ4Match(src []byte, index int, ref int) bool {
	if ref < 0 || index-ref > lz4MaxOffset {
		return false
	}
	return binary.LittleEndian.Uint32(src[ref:]) == binary.LittleEndian.Uint32(src[index:])
}

func lz4Hash(sequence uint32) int {
	return int((sequence * 2654435761) >> lz4HashShift)
}

func countLZ4Match(src []byte, ref int, index int) int {
	count := 0
	for index+count < len(src) && src[ref+count] == src[index+count] {
		count++
	}
	return count
}

func appendLZ4Sequence(dst []byte, literals []byte, offset int, matchLen int) []byte {
	literalLen := len(literals)
	matchCode := matchLen - lz4MinMatch
	tokenIndex := len(dst)
	dst = append(dst, lz4Token(literalLen, matchCode))
	dst = appendLZ4Length(dst, literalLen)
	dst = append(dst, literals...)
	dst = binary.LittleEndian.AppendUint16(dst, uint16(offset))
	dst = appendLZ4Length(dst, matchCode)
	dst[tokenIndex] = lz4Token(literalLen, matchCode)
	return dst
}

func appendLZ4Literals(dst []byte, literals []byte) []byte {
	literalLen := len(literals)
	dst = append(dst, lz4Token(literalLen, 0))
	dst = appendLZ4Length(dst, literalLen)
	return append(dst, literals...)
}

func lz4Token(literalLen int, matchCode int) byte {
	token := byte(min(literalLen, 15) << 4)
	return token | byte(min(matchCode, 15))
}

func appendLZ4Length(dst []byte, length int) []byte {
	if length < 15 {
		return dst
	}
	length -= 15
	for length >= 255 {
		dst = append(dst, 255)
		length -= 255
	}
	return append(dst, byte(length))
}

func readLZ4Length(src []byte, nibble int) (int, []byte, error) {
	if nibble < 15 {
		return nibble, src, nil
	}
	length := 15
	for {
		if len(src) == 0 {
			return 0, nil, fmt.Errorf("truncated extended length")
		}
		next := int(src[0])
		src = src[1:]
		length += next
		if next != 255 {
			return length, src, nil
		}
	}
}

func appendLZ4Match(dst *[]byte, offset int, count int, rawSize int) error {
	out := *dst
	if len(out)+count > rawSize {
		return fmt.Errorf("match length exceeds raw size")
	}
	start := len(out) - offset
	for range count {
		out = append(out, out[start])
		start++
	}
	*dst = out
	return nil
}
