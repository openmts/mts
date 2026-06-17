package sstable

import (
	"encoding/binary"
	"fmt"
	"sync"

	"github.com/klauspost/compress/snappy"
	"github.com/klauspost/compress/zstd"
	"github.com/pierrec/lz4/v4"
)

const (
	payloadCompressionNone byte = iota
	payloadCompressionSnappy
	payloadCompressionLZ4
	payloadCompressionZSTD
)

var (
	lz4Compressors = sync.Pool{}
	zstdEncoders   = sync.Pool{}
	zstdDecoders   = sync.Pool{}
)

func appendCodecPayloadWithCompression(
	dst []byte,
	codec byte,
	payload []byte,
	algorithm string,
) ([]byte, error) {
	algorithmID, err := payloadCompressionAlgorithmID(algorithm)
	if err != nil {
		return nil, err
	}
	storedAlgorithmID, stored, err := compressPayload(algorithmID, payload)
	if err != nil {
		return nil, err
	}
	dst = append(dst, codec, storedAlgorithmID)
	dst = binary.AppendUvarint(dst, uint64(len(payload)))
	dst = binary.AppendUvarint(dst, uint64(len(stored)))
	return append(dst, stored...), nil
}

func payloadCompressionAlgorithmID(algorithm string) (byte, error) {
	switch algorithm {
	case "", "none":
		return payloadCompressionNone, nil
	case "snappy":
		return payloadCompressionSnappy, nil
	case "lz4":
		return payloadCompressionLZ4, nil
	case "zstd":
		return payloadCompressionZSTD, nil
	default:
		return 0, fmt.Errorf("unknown payload compression algorithm %q", algorithm)
	}
}

func compressPayload(algorithmID byte, payload []byte) (byte, []byte, error) {
	switch algorithmID {
	case payloadCompressionNone:
		return payloadCompressionNone, payload, nil
	case payloadCompressionSnappy:
		return payloadCompressionSnappy, snappy.Encode(nil, payload), nil
	case payloadCompressionLZ4:
		return encodeLZ4(payload)
	case payloadCompressionZSTD:
		encoded, err := encodeZSTD(payload)
		return payloadCompressionZSTD, encoded, err
	default:
		return 0, nil, fmt.Errorf("unknown payload compression id %d", algorithmID)
	}
}

func decompressPayload(algorithmID byte, payload []byte, rawSize int) ([]byte, error) {
	var (
		decoded []byte
		err     error
	)
	switch algorithmID {
	case payloadCompressionNone:
		decoded = payload
	case payloadCompressionSnappy:
		decoded, err = snappy.Decode(make([]byte, 0, rawSize), payload)
	case payloadCompressionLZ4:
		decoded, err = decodeLZ4(payload, rawSize)
	case payloadCompressionZSTD:
		decoded, err = decodeZSTD(payload, rawSize)
	default:
		return nil, fmt.Errorf("unknown payload compression id %d", algorithmID)
	}
	if err != nil {
		return nil, err
	}
	if len(decoded) != rawSize {
		return nil, fmt.Errorf("decoded size %d does not match raw size %d", len(decoded), rawSize)
	}
	return decoded, nil
}

func readPayloadSize(reader *blockReader, name string, kind string) (int, error) {
	sizeValue, sizeBytes := binary.Uvarint(reader.rest)
	if sizeBytes <= 0 {
		return 0, fmt.Errorf("decode sstable %s %s size: invalid uvarint", name, kind)
	}
	reader.rest = reader.rest[sizeBytes:]
	maxInt := uint64(int(^uint(0) >> 1))
	if sizeValue > maxInt {
		return 0, fmt.Errorf("decode sstable %s %s size: count %d overflows int", name, kind, sizeValue)
	}
	return int(sizeValue), nil
}

func encodeLZ4(payload []byte) (byte, []byte, error) {
	compressor := getLZ4Compressor()
	defer lz4Compressors.Put(compressor)
	dst := make([]byte, lz4.CompressBlockBound(len(payload)))
	size, err := compressor.CompressBlock(payload, dst)
	if err != nil {
		return 0, nil, err
	}
	if size == 0 {
		return payloadCompressionNone, payload, nil
	}
	return payloadCompressionLZ4, dst[:size], nil
}

func decodeLZ4(payload []byte, rawSize int) ([]byte, error) {
	dst := make([]byte, rawSize)
	size, err := lz4.UncompressBlock(payload, dst)
	if err != nil {
		return nil, err
	}
	return dst[:size], nil
}

func getLZ4Compressor() *lz4.Compressor {
	if value := lz4Compressors.Get(); value != nil {
		return value.(*lz4.Compressor)
	}
	return &lz4.Compressor{}
}

func encodeZSTD(payload []byte) ([]byte, error) {
	encoder, err := getZSTDEncoder()
	if err != nil {
		return nil, err
	}
	defer zstdEncoders.Put(encoder)
	return encoder.EncodeAll(payload, nil), nil
}

func decodeZSTD(payload []byte, rawSize int) ([]byte, error) {
	decoder, err := getZSTDDecoder()
	if err != nil {
		return nil, err
	}
	defer zstdDecoders.Put(decoder)
	return decoder.DecodeAll(payload, make([]byte, 0, rawSize))
}

func getZSTDEncoder() (*zstd.Encoder, error) {
	if value := zstdEncoders.Get(); value != nil {
		return value.(*zstd.Encoder), nil
	}
	return zstd.NewWriter(
		nil,
		zstd.WithEncoderConcurrency(1),
		zstd.WithEncoderLevel(zstd.SpeedFastest),
		zstd.WithLowerEncoderMem(true),
	)
}

func getZSTDDecoder() (*zstd.Decoder, error) {
	if value := zstdDecoders.Get(); value != nil {
		return value.(*zstd.Decoder), nil
	}
	return zstd.NewReader(nil, zstd.WithDecoderConcurrency(1), zstd.WithDecoderLowmem(true))
}
