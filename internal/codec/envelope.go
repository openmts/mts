package codec

import (
	"encoding/binary"
	"fmt"
	"hash/crc32"
)

const (
	envelopeMagicLen = 7
	envelopeHeadLen  = envelopeMagicLen + 2
	envelopeCRCLen   = 4
)

var crcTable = crc32.MakeTable(crc32.Castagnoli)

type Magic string

type Envelope struct {
	Magic   Magic
	Flags   uint16
	Payload []byte
}

func MarshalEnvelope(dst []byte, magic Magic, flags uint16, payload []byte) []byte {
	start := len(dst)
	dst = appendMagic(dst, magic)
	dst = binary.LittleEndian.AppendUint16(dst, flags)
	dst = binary.AppendUvarint(dst, uint64(len(payload)))
	dst = append(dst, payload...)
	sum := crc32.Checksum(dst[start:], crcTable)
	return binary.LittleEndian.AppendUint32(dst, sum)
}

func UnmarshalEnvelope(data []byte, want Magic) (Envelope, error) {
	env, err := unmarshalEnvelope(data, want)
	if err != nil {
		return Envelope{}, err
	}
	env.Payload = append([]byte(nil), env.Payload...)
	return env, nil
}

func UnmarshalEnvelopeView(data []byte, want Magic) (Envelope, error) {
	return unmarshalEnvelope(data, want)
}

func unmarshalEnvelope(data []byte, want Magic) (Envelope, error) {
	if len(data) < envelopeHeadLen+envelopeCRCLen {
		return Envelope{}, fmt.Errorf("decode envelope: data too short: %d", len(data))
	}
	if got := Magic(string(data[:envelopeMagicLen])); got != want {
		return Envelope{}, fmt.Errorf("decode envelope: magic %q, want %q", got, want)
	}
	flags := binary.LittleEndian.Uint16(data[envelopeMagicLen:])
	payload, err := envelopePayload(data)
	if err != nil {
		return Envelope{}, err
	}
	return Envelope{Magic: want, Flags: flags, Payload: payload}, nil
}

func appendMagic(dst []byte, magic Magic) []byte {
	var fixed [envelopeMagicLen]byte
	copy(fixed[:], string(magic))
	return append(dst, fixed[:]...)
}

func envelopePayload(data []byte) ([]byte, error) {
	wantCRC := binary.LittleEndian.Uint32(data[len(data)-envelopeCRCLen:])
	gotCRC := crc32.Checksum(data[:len(data)-envelopeCRCLen], crcTable)
	if gotCRC != wantCRC {
		return nil, fmt.Errorf("decode envelope: crc mismatch")
	}
	payloadLen, size := binary.Uvarint(data[envelopeHeadLen:])
	if size <= 0 {
		return nil, fmt.Errorf("decode envelope: invalid payload length")
	}
	payloadStart := envelopeHeadLen + size
	payloadLimit := len(data) - envelopeCRCLen
	if payloadStart > payloadLimit || payloadLen > uint64(payloadLimit-payloadStart) {
		return nil, fmt.Errorf("decode envelope: payload truncated")
	}
	payloadEnd := payloadStart + int(payloadLen)
	if payloadEnd != len(data)-envelopeCRCLen {
		return nil, fmt.Errorf("decode envelope: trailing bytes before crc")
	}
	return data[payloadStart:payloadEnd], nil
}
