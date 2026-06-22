package mts

import (
	"errors"

	storageengine "github.com/openmts/mts/internal/engine"
	"github.com/openmts/mts/internal/queryanalyzer"
	"github.com/openmts/mts/internal/queryexec"
)

// ErrInvalidOptions 表示 Engine 配置非法。
var ErrInvalidOptions = errors.New("invalid options")

// ErrNotFound 表示请求的 measurement、field 或资源不存在。
var ErrNotFound = errors.New("not found")

// ErrUnsupported 表示当前单机公开 API 不支持该能力或语义。
var ErrUnsupported = errors.New("unsupported")

// ErrReadBudgetExceeded 表示查询读取预算已耗尽。
var ErrReadBudgetExceeded = queryexec.ErrReadBudgetExceeded

func publicError(err error) error {
	if err == nil {
		return nil
	}
	if queryanalyzer.IsCode(err, queryanalyzer.ErrMeasurementNotFound) ||
		queryanalyzer.IsCode(err, queryanalyzer.ErrFieldNotFound) ||
		errors.Is(err, storageengine.ErrDownsamplePolicyNotFound) {
		return errors.Join(ErrNotFound, err)
	}
	if queryanalyzer.IsCode(err, queryanalyzer.ErrUnsupportedFunction) {
		return errors.Join(ErrUnsupported, err)
	}
	if errors.Is(err, queryexec.ErrUnsupportedAggregate) {
		return errors.Join(ErrUnsupported, err)
	}
	return err
}
