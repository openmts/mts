package user

import (
	"bytes"
	"encoding/binary"
	"testing"
	"time"

	"github.com/openmts/mts/internal/codec"
)

func TestUserMetadataEncodingRoundTripIncludesAuthState(t *testing.T) {
	passwords := map[string]passwordRecord{
		"alice": {Salt: []byte("salt"), Hash: []byte("hash"), Iterations: 7},
	}
	tokens := map[string]tokenRecord{
		"abc": {UserName: "alice", ExpiresAtUnixNano: time.Unix(1, 2).UnixNano()},
	}
	data := encodeUserMetadata(
		map[string]User{"alice": {Name: "alice", Role: RoleAdmin, Metadata: map[string]string{"team": "storage"}}},
		map[string]map[string]map[Permission]struct{}{
			"alice": {"metrics": {PermissionRead: {}, PermissionAdmin: {}}},
		},
		passwords,
		tokens,
	)
	users, grants, gotPasswords, gotTokens, err := decodeUserMetadata(data)
	if err != nil {
		t.Fatalf("decodeUserMetadata() error = %v", err)
	}
	if users["alice"].Metadata["team"] != "storage" {
		t.Fatalf("users = %#v, want metadata", users)
	}
	if users["alice"].Role != RoleAdmin {
		t.Fatalf("role = %q, want admin", users["alice"].Role)
	}
	if _, ok := grants["alice"]["metrics"][PermissionAdmin]; !ok {
		t.Fatalf("grants = %#v, want admin", grants)
	}
	if !bytes.Equal(gotPasswords["alice"].Salt, []byte("salt")) || gotPasswords["alice"].Iterations != 7 {
		t.Fatalf("password = %#v, want salt and iterations", gotPasswords["alice"])
	}
	if gotTokens["abc"].UserName != "alice" {
		t.Fatalf("tokens = %#v, want alice token", gotTokens)
	}
}

func TestUserMetadataDecodeRejectsInvalidPayloads(t *testing.T) {
	if _, _, _, _, err := decodeUserMetadata([]byte("bad")); err == nil {
		t.Fatal("decodeUserMetadata(bad envelope) error = nil, want error")
	}
	payload := []byte{1}
	data := codec.MarshalEnvelope(nil, userMetadataMagic, 0, payload)
	if _, _, _, _, err := decodeUserMetadata(data); err == nil {
		t.Fatal("decodeUserMetadata(truncated user) error = nil, want error")
	}
	if _, err := permissionsFromMask(8); err == nil {
		t.Fatal("permissionsFromMask(invalid) error = nil, want error")
	}
}

func TestUserMetadataDecodeRejectsInvalidNestedFields(t *testing.T) {
	tests := []struct {
		name    string
		payload []byte
	}{
		{name: "invalid disabled bool", payload: appendUserPrefix(2)},
		{name: "truncated password bytes", payload: appendPasswordPrefix([]byte{1, 4, 's'})},
		{name: "truncated token int64", payload: appendTokenPrefix([]byte{1, 1, 'd', 1, 'u', 1})},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := codec.MarshalEnvelope(nil, userMetadataMagic, 0, tt.payload)
			if _, _, _, _, err := decodeUserMetadata(data); err == nil {
				t.Fatal("decodeUserMetadata() error = nil, want error")
			}
		})
	}
}

func appendUserPrefix(disabled byte) []byte {
	payload := binary.AppendUvarint(nil, 1)
	payload = codec.AppendString(payload, "alice")
	payload = codec.AppendString(payload, "Alice")
	payload = codec.AppendString(payload, string(RoleUser))
	payload = append(payload, disabled)
	return payload
}

func appendPasswordPrefix(suffix []byte) []byte {
	payload := binary.AppendUvarint(nil, 1)
	payload = appendUser(payload, User{Name: "alice"})
	return append(payload, suffix...)
}

func appendTokenPrefix(suffix []byte) []byte {
	payload := binary.AppendUvarint(nil, 0)
	return append(payload, suffix...)
}
