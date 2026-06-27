package mts

import (
	"reflect"
	"testing"
)

func TestLocalUserManagerStoresBackendInterface(t *testing.T) {
	field, ok := reflect.TypeOf(localUserManager{}).FieldByName("inner")
	if !ok {
		t.Fatal("localUserManager.inner field is missing")
	}
	if field.Type.Kind() != reflect.Interface {
		t.Fatalf("localUserManager.inner kind = %s, want interface", field.Type.Kind())
	}
}
