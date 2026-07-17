package mts

import (
	"errors"
	"fmt"
	"testing"

	"github.com/openmts/mts/internal/catalog"
	storageengine "github.com/openmts/mts/internal/engine"
	"github.com/openmts/mts/internal/queryexec"
)

func TestPublicErrorMapsStableSentinels(t *testing.T) {
	cases := []struct {
		name   string
		input  error
		checks []error
	}{
		{
			name:   "cardinality",
			input:  fmt.Errorf("wrap: %w", catalog.ErrCardinalityLimit),
			checks: []error{ErrCardinalityLimit, ErrResourceExhausted},
		},
		{
			name:   "memory",
			input:  fmt.Errorf("wrap: %w", storageengine.ErrStorageMemoryLimitExceeded),
			checks: []error{ErrStorageMemoryLimitExceeded, ErrResourceExhausted},
		},
		{
			name:   "read budget",
			input:  fmt.Errorf("wrap: %w", queryexec.ErrReadBudgetExceeded),
			checks: []error{ErrReadBudgetExceeded, ErrResourceExhausted},
		},
		{
			name:   "shard busy",
			input:  fmt.Errorf("wrap: %w", storageengine.ErrShardBusy),
			checks: []error{ErrEngineBusy, ErrResourceExhausted},
		},
		{
			name:   "empty measurement",
			input:  catalog.ErrEmptyMeasurement,
			checks: []error{ErrInvalidOptions},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := publicError(tc.input)
			for _, want := range tc.checks {
				if !errors.Is(got, want) {
					t.Fatalf("publicError() = %v, want errors.Is(%v)", got, want)
				}
			}
		})
	}
}
