package main

import (
	"errors"
	"fmt"
	"testing"

	mts "github.com/openmts/mts"
)

func TestClassifyAPIErrorResourceExhausted(t *testing.T) {
	cases := []struct {
		name string
		err  error
	}{
		{name: "resource exhausted", err: mts.ErrResourceExhausted},
		{name: "cardinality", err: mts.ErrCardinalityLimit},
		{name: "memory", err: mts.ErrStorageMemoryLimitExceeded},
		{name: "read budget", err: mts.ErrReadBudgetExceeded},
		{name: "engine busy", err: mts.ErrEngineBusy},
		{name: "joined", err: errors.Join(mts.ErrCardinalityLimit, mts.ErrResourceExhausted)},
		{name: "wrapped", err: fmt.Errorf("wrap: %w", mts.ErrEngineBusy)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := classifyAPIError(tc.err)
			if got.Code != errorCodeResourceExhausted {
				t.Fatalf("classifyAPIError() code = %q, want %q message=%q", got.Code, errorCodeResourceExhausted, got.Message)
			}
		})
	}
}

func TestClassifyAPIErrorStableSentinels(t *testing.T) {
	cases := []struct {
		name string
		err  error
		code errorCode
	}{
		{name: "not found", err: mts.ErrNotFound, code: errorCodeNotFound},
		{name: "unsupported", err: mts.ErrUnsupported, code: errorCodeBadRequest},
		{name: "invalid options", err: mts.ErrInvalidOptions, code: errorCodeBadRequest},
		{name: "permission", err: mts.ErrPermissionDenied, code: errorCodePermissionDenied},
		{name: "unauthenticated", err: mts.ErrInvalidCredentials, code: errorCodeUnauthenticated},
		{name: "already exists", err: mts.ErrUserAlreadyExists, code: errorCodeAlreadyExists},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := classifyAPIError(tc.err)
			if got.Code != tc.code {
				t.Fatalf("classifyAPIError() code = %q, want %q", got.Code, tc.code)
			}
		})
	}
}
