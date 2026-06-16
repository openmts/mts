package codec

import (
	"bytes"
	"testing"

	"codeberg.org/mts/mts/internal/model"
)

func TestEnvelopeRoundTripAndCorruption(t *testing.T) {
	payload := []byte{1, 2, 3, 4}
	frame := MarshalEnvelope(nil, Magic("MTSTST2"), 0, payload)
	got, err := UnmarshalEnvelope(frame, Magic("MTSTST2"))
	if err != nil {
		t.Fatalf("UnmarshalEnvelope() error = %v", err)
	}
	if !bytes.Equal(got.Payload, payload) {
		t.Fatalf("payload = %v, want %v", got.Payload, payload)
	}
	frame[len(frame)-1] ^= 0xff
	if _, err := UnmarshalEnvelope(frame, Magic("MTSTST2")); err == nil {
		t.Fatal("UnmarshalEnvelope(corrupt) error = nil, want error")
	}
}

func TestEnvelopeViewSharesPayloadAndCopyDoesNot(t *testing.T) {
	payload := []byte{1, 2, 3, 4}
	frame := MarshalEnvelope(nil, Magic("MTSTST2"), 7, payload)
	copied, err := UnmarshalEnvelope(frame, Magic("MTSTST2"))
	if err != nil {
		t.Fatalf("UnmarshalEnvelope() error = %v", err)
	}
	view, err := UnmarshalEnvelopeView(frame, Magic("MTSTST2"))
	if err != nil {
		t.Fatalf("UnmarshalEnvelopeView() error = %v", err)
	}
	if view.Flags != 7 {
		t.Fatalf("view flags = %d, want 7", view.Flags)
	}
	view.Payload[0] = 99
	if copied.Payload[0] == 99 {
		t.Fatal("UnmarshalEnvelope() returned shared payload, want copy")
	}
	if frame[envelopeHeadLen+1] != 99 {
		t.Fatalf("view payload did not share frame backing array, frame byte = %d", frame[envelopeHeadLen+1])
	}
}

func TestFieldValueRoundTrip(t *testing.T) {
	values := []model.FieldValue{
		model.Float64Value(1.5),
		model.Int64Value(-2),
		model.StringValue("ok"),
		model.BoolValue(true),
	}
	buf := make([]byte, 0)
	for _, value := range values {
		buf = AppendFieldValue(buf, value)
	}
	rest := buf
	for _, want := range values {
		got, next, err := ReadFieldValue(rest)
		if err != nil {
			t.Fatalf("ReadFieldValue() error = %v", err)
		}
		if got != want {
			t.Fatalf("value = %#v, want %#v", got, want)
		}
		rest = next
	}
	if len(rest) != 0 {
		t.Fatalf("remaining bytes = %d, want 0", len(rest))
	}
}

func TestStringRoundTripAndTruncatedInput(t *testing.T) {
	buf := AppendString(nil, "hello")
	got, rest, err := ReadString(buf)
	if err != nil {
		t.Fatalf("ReadString() error = %v", err)
	}
	if got != "hello" || len(rest) != 0 {
		t.Fatalf("ReadString() = %q with %d rest bytes, want hello with 0", got, len(rest))
	}
	if _, _, err := ReadString(buf[:len(buf)-1]); err == nil {
		t.Fatal("ReadString(truncated) error = nil, want error")
	}
}

func TestBoolBitsRoundTrip(t *testing.T) {
	values := []bool{true, false, true, true, false, false, false, true, true}
	buf := AppendBoolBits(nil, values)
	got, rest, err := ReadBoolBits(buf, len(values))
	if err != nil {
		t.Fatalf("ReadBoolBits() error = %v", err)
	}
	if len(rest) != 0 {
		t.Fatalf("rest bytes = %d, want 0", len(rest))
	}
	for index, want := range values {
		if got[index] != want {
			t.Fatalf("value[%d] = %v, want %v", index, got[index], want)
		}
	}
	if _, _, err := ReadBoolBits(buf[:len(buf)-1], len(values)); err == nil {
		t.Fatal("ReadBoolBits(truncated) error = nil, want error")
	}
}

func TestFieldValueRejectsInvalidPayloads(t *testing.T) {
	tests := [][]byte{
		nil,
		{byte(model.FieldFloat64), 1},
		{byte(model.FieldInt64), 0xff},
		{byte(model.FieldString), 2, 'a'},
		{byte(model.FieldBool)},
		{byte(model.FieldBool), 2},
		{99},
	}
	for _, data := range tests {
		if _, _, err := ReadFieldValue(data); err == nil {
			t.Fatalf("ReadFieldValue(%v) error = nil, want error", data)
		}
	}
}

func TestAppendFieldValueUnsupportedTypeWritesTypeOnly(t *testing.T) {
	got := AppendFieldValue(nil, model.FieldValue{Type: model.FieldType(99)})
	if len(got) != 1 || got[0] != 99 {
		t.Fatalf("AppendFieldValue(unsupported) = %v, want type byte only", got)
	}
}

func TestEnvelopeRejectsBadMagicLengthAndTrailingBytes(t *testing.T) {
	if _, err := UnmarshalEnvelope([]byte{1}, Magic("MTSTST2")); err == nil {
		t.Fatal("UnmarshalEnvelope(short) error = nil, want error")
	}
	frame := MarshalEnvelope(nil, Magic("MTSTST2"), 0, []byte{1})
	if _, err := UnmarshalEnvelope(frame, Magic("MTSBAD2")); err == nil {
		t.Fatal("UnmarshalEnvelope(magic) error = nil, want error")
	}
	frame = append(frame[:len(frame)-4], append([]byte{0}, frame[len(frame)-4:]...)...)
	if _, err := UnmarshalEnvelope(frame, Magic("MTSTST2")); err == nil {
		t.Fatal("UnmarshalEnvelope(trailing) error = nil, want error")
	}
}
