package mts

import (
	"context"
	"fmt"

	"github.com/openmts/mts/internal/model"
)

// DeleteRequest 描述按时间范围删除本地时序数据。
//
// Measurement 必填。Tags 为空时删除 measurement 在时间范围内的全部 series；
// Tags 非空时仅删除精确匹配的 series。删除通过 tombstone 生效，查询立即不可见，
// 磁盘回收依赖后续 compaction。
type DeleteRequest struct {
	Database        string            `json:"database,omitempty"`
	RetentionPolicy string            `json:"retention_policy,omitempty"`
	Measurement     string            `json:"measurement"`
	Tags            map[string]string `json:"tags,omitempty"`
	StartTime       int64             `json:"start_time,omitempty"`
	EndTime         int64             `json:"end_time,omitempty"`
	Precision       TimePrecision     `json:"precision,omitempty"`
}

// Delete 删除匹配的本地时序数据。
func (e *Engine) Delete(ctx context.Context, req DeleteRequest) error {
	converted, err := toModelDeleteRequest(req)
	if err != nil {
		return err
	}
	return publicError(e.runtime.Storage().Delete(ctx, converted))
}

func toModelDeleteRequest(req DeleteRequest) (model.DeleteRequest, error) {
	if req.Measurement == "" {
		return model.DeleteRequest{}, fmt.Errorf("%w: measurement is empty", ErrInvalidOptions)
	}
	factor, err := timePrecisionFactor(req.Precision)
	if err != nil {
		return model.DeleteRequest{}, err
	}
	start, err := timestampToNanoseconds(req.StartTime, factor)
	if err != nil {
		return model.DeleteRequest{}, err
	}
	end, err := timestampToNanoseconds(req.EndTime, factor)
	if err != nil {
		return model.DeleteRequest{}, err
	}
	if end < start {
		return model.DeleteRequest{}, fmt.Errorf("%w: delete end time before start time", ErrInvalidOptions)
	}
	return model.DeleteRequest{
		Database:        req.Database,
		RetentionPolicy: req.RetentionPolicy,
		Measurement:     req.Measurement,
		Tags:            cloneStringMap(req.Tags),
		StartTime:       start,
		EndTime:         end,
	}, nil
}
