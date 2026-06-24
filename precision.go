package mts

import "fmt"

// TimePrecision 表示 public API 中整数时间戳的单位。
type TimePrecision string

const (
	// PrecisionNanosecond 表示 Unix nanosecond。
	PrecisionNanosecond TimePrecision = "ns"
	// PrecisionMicrosecond 表示 Unix microsecond。
	PrecisionMicrosecond TimePrecision = "us"
	// PrecisionMillisecond 表示 Unix millisecond。
	PrecisionMillisecond TimePrecision = "ms"
	// PrecisionSecond 表示 Unix second。
	PrecisionSecond TimePrecision = "s"
)

const (
	maxInt64 = int64(^uint64(0) >> 1)
	minInt64 = -maxInt64 - 1
)

func timePrecisionFactor(precision TimePrecision) (int64, error) {
	switch precision {
	case "", PrecisionNanosecond:
		return 1, nil
	case PrecisionMicrosecond:
		return 1_000, nil
	case PrecisionMillisecond:
		return 1_000_000, nil
	case PrecisionSecond:
		return 1_000_000_000, nil
	default:
		return 0, fmt.Errorf("%w: %q", ErrInvalidPrecision, precision)
	}
}

func timestampToNanoseconds(value int64, factor int64) (int64, error) {
	if factor == 1 {
		return value, nil
	}
	if value > maxInt64/factor || value < minInt64/factor {
		return 0, fmt.Errorf("%w: timestamp %d overflows nanoseconds", ErrInvalidPrecision, value)
	}
	return value * factor, nil
}

func timestampFromNanoseconds(value int64, factor int64) int64 {
	if factor == 1 {
		return value
	}
	return value / factor
}
