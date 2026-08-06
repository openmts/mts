package mts

import (
	"errors"

	"github.com/openmts/mts/internal/catalog"
	storageengine "github.com/openmts/mts/internal/engine"
	"github.com/openmts/mts/internal/queryanalyzer"
	"github.com/openmts/mts/internal/queryexec"
	"github.com/openmts/mts/internal/runtime"
)

// ErrInvalidOptions 表示 Engine 配置非法。
var ErrInvalidOptions = errors.New("invalid options")

// ErrNotFound 表示请求的 measurement、field 或资源不存在。
var ErrNotFound = errors.New("not found")

// ErrUnsupported 表示当前单机公开 API 不支持该能力或语义。
var ErrUnsupported = errors.New("unsupported")

// ErrInvalidPrecision 表示时间精度声明非法或转换后会溢出。
var ErrInvalidPrecision = errors.New("invalid precision")

// ErrReadBudgetExceeded 表示查询读取预算已耗尽。
var ErrReadBudgetExceeded = queryexec.ErrReadBudgetExceeded

// ErrCardinalityLimit 表示 series/field/tag 基数超过配置上限。
var ErrCardinalityLimit = catalog.ErrCardinalityLimit

// ErrStorageMemoryLimitExceeded 表示存储内存预算耗尽。
var ErrStorageMemoryLimitExceeded = storageengine.ErrStorageMemoryLimitExceeded

// ErrResourceExhausted 表示资源预算/配额耗尽（内存、读预算、基数、引擎繁忙等）。
var ErrResourceExhausted = errors.New("resource exhausted")

// ErrEngineBusy 表示引擎或 shard 正忙（例如活跃查询占用读引用）。
var ErrEngineBusy = storageengine.ErrShardBusy

// 用户管理相关错误。
var (
	ErrInvalidUser             = runtime.ErrInvalidUser
	ErrInvalidPageLimit        = runtime.ErrInvalidPageLimit
	ErrUserNotFound            = runtime.ErrUserNotFound
	ErrUserAlreadyExists       = runtime.ErrUserAlreadyExists
	ErrInvalidPermission       = runtime.ErrInvalidPermission
	ErrPermissionDenied        = runtime.ErrPermissionDenied
	ErrInvalidCredentials      = runtime.ErrInvalidCredentials
	ErrAuthenticationDisabled  = runtime.ErrAuthenticationDisabled
	ErrUnsupportedUserEndpoint = runtime.ErrUnsupportedUserEndpoint
)

func publicError(err error) error {
	if err == nil {
		return nil
	}
	switch {
	case queryanalyzer.IsCode(err, queryanalyzer.ErrMeasurementNotFound),
		queryanalyzer.IsCode(err, queryanalyzer.ErrFieldNotFound),
		errors.Is(err, storageengine.ErrDownsamplePolicyNotFound),
		errors.Is(err, ErrNotFound),
		errors.Is(err, ErrUserNotFound):
		return errors.Join(ErrNotFound, err)
	case queryanalyzer.IsCode(err, queryanalyzer.ErrUnsupportedFunction),
		errors.Is(err, queryexec.ErrUnsupportedAggregate),
		errors.Is(err, ErrUnsupported):
		return errors.Join(ErrUnsupported, err)
	case errors.Is(err, catalog.ErrCardinalityLimit):
		return errors.Join(ErrCardinalityLimit, ErrResourceExhausted, err)
	case errors.Is(err, storageengine.ErrStorageMemoryLimitExceeded):
		return errors.Join(ErrStorageMemoryLimitExceeded, ErrResourceExhausted, err)
	case errors.Is(err, queryexec.ErrReadBudgetExceeded):
		return errors.Join(ErrReadBudgetExceeded, ErrResourceExhausted, err)
	case errors.Is(err, storageengine.ErrShardBusy):
		return errors.Join(ErrEngineBusy, ErrResourceExhausted, err)
	case errors.Is(err, catalog.ErrEmptyMeasurement),
		errors.Is(err, catalog.ErrEmptyFields):
		return errors.Join(ErrInvalidOptions, err)
	default:
		return err
	}
}
