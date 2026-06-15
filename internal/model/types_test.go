package model

import "testing"

func TestFieldValueConstructors(t *testing.T) {
	tests := []struct {
		name string
		got  FieldValue
		want FieldType
	}{
		{name: "float", got: Float64Value(1), want: FieldFloat64},
		{name: "int", got: Int64Value(2), want: FieldInt64},
		{name: "string", got: StringValue("x"), want: FieldString},
		{name: "bool", got: BoolValue(true), want: FieldBool},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got.Type != tt.want {
				t.Fatalf("Type = %v, want %v", tt.got.Type, tt.want)
			}
		})
	}
}
