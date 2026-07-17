package queryservice

import (
	"context"
	"time"

	"github.com/openmts/mts/internal/model"
	"github.com/openmts/mts/internal/queryanalyzer"
	"github.com/openmts/mts/internal/queryexec"
	"github.com/openmts/mts/internal/querylang"
	"github.com/openmts/mts/internal/queryoptimizer"
	"github.com/openmts/mts/internal/queryphysical"
	"github.com/openmts/mts/internal/queryplanner"
)

type LayeredReader interface {
	queryanalyzer.SchemaProvider
	QuerySpecRows(context.Context, querylang.QuerySpec) ([]model.Row, error)
	QuerySpecWithExplain(
		context.Context,
		querylang.QuerySpec,
	) ([]model.ColumnSeries, model.QueryExplain, model.QueryStats, error)
}

type LayeredStreamReader interface {
	QuerySpecRowStream(context.Context, querylang.QuerySpec) (queryexec.RowStream, error)
	QuerySpecColumnStream(context.Context, querylang.QuerySpec) (queryexec.ColumnStream, error)
}

// LayeredExecutor 是可选的分层查询包装（analyze/plan/optimize）。
// 权威查询语义与结果以 internal/engine + queryexec 为准；本执行器仅做兼容/实验路径，
// 生产路径（public API / mts-server）应直接调用 Engine 查询方法。
type LayeredExecutor struct {
	reader LayeredReader
}

func NewLayeredExecutor(reader LayeredReader) LayeredExecutor {
	return LayeredExecutor{reader: reader}
}

func (e LayeredExecutor) Query(ctx context.Context, query model.Query) (Result, error) {
	if e.reader == nil {
		return Result{}, nil
	}
	var recorder profileRecorder
	spec, err := querylang.FromModelQuery(query, querylang.Defaults{})
	if err != nil {
		return Result{}, err
	}
	layered, err := e.buildLayers(ctx, spec, query.Budget, &recorder)
	if err != nil {
		return Result{Profile: recorder.profile}, err
	}
	if len(spec.Aggregates) > 0 {
		return e.queryColumns(ctx, spec, layered, &recorder)
	}
	return e.queryRows(ctx, spec, layered, &recorder)
}

func (e LayeredExecutor) QueryStream(ctx context.Context, query model.Query) (StreamResult, error) {
	if e.reader == nil {
		return StreamResult{}, nil
	}
	streamReader, ok := e.reader.(LayeredStreamReader)
	if !ok {
		return StreamResult{}, ErrStreamingUnsupported
	}
	var recorder profileRecorder
	spec, err := querylang.FromModelQuery(query, querylang.Defaults{})
	if err != nil {
		return StreamResult{}, err
	}
	layered, err := e.buildLayers(ctx, spec, query.Budget, &recorder)
	if err != nil {
		return StreamResult{Profile: recorder.profile}, err
	}
	if len(spec.Aggregates) > 0 {
		return e.queryColumnStream(ctx, spec, layered, streamReader, &recorder)
	}
	return e.queryRowStream(ctx, spec, layered, streamReader, &recorder)
}

type layeredPlan struct {
	logical   queryplanner.LogicalPlan
	physical  queryphysical.PhysicalPlan
	optimized queryoptimizer.OptimizedPlan
}

func (e LayeredExecutor) buildLayers(
	ctx context.Context,
	spec querylang.QuerySpec,
	budget model.QueryBudget,
	recorder *profileRecorder,
) (layeredPlan, error) {
	start := time.Now()
	analysis, err := queryanalyzer.New(e.reader).Analyze(ctx, spec)
	recorder.record("analyze", start, err, nil)
	if err != nil {
		return layeredPlan{}, err
	}
	start = time.Now()
	logical, err := queryplanner.Build(analysis)
	recorder.record("logical_plan", start, err, nil)
	if err != nil {
		return layeredPlan{}, err
	}
	start = time.Now()
	optimized, err := queryoptimizer.Optimize(logical, queryoptimizer.Context{Budget: budget})
	recorder.record("optimize", start, err, nil)
	if err != nil {
		return layeredPlan{}, err
	}
	start = time.Now()
	physical, err := queryphysical.Build(optimized)
	recorder.record("physical_plan", start, err, func(entry *queryexec.OperatorProfile) {
		entry.ColumnsOut = len(physical.Operators)
	})
	if err != nil {
		return layeredPlan{}, err
	}
	return layeredPlan{logical: logical, physical: physical, optimized: optimized}, nil
}

func (e LayeredExecutor) queryColumns(
	ctx context.Context,
	spec querylang.QuerySpec,
	layered layeredPlan,
	recorder *profileRecorder,
) (Result, error) {
	start := time.Now()
	columns, explain, stats, err := e.reader.QuerySpecWithExplain(ctx, spec)
	recorder.record("execute", start, err, func(entry *queryexec.OperatorProfile) {
		entry.ColumnsOut = len(columns)
		entry.SamplesOut = countColumnSamples(columns)
	})
	if err != nil {
		result := layeredResult(layered)
		result.Profile = recorder.profile
		return result, err
	}
	result := layeredResult(layered)
	result.Columns = columns
	result.Explain = explain
	result.Stats = stats
	result.Profile = recorder.profile
	return result, nil
}

func (e LayeredExecutor) queryRows(
	ctx context.Context,
	spec querylang.QuerySpec,
	layered layeredPlan,
	recorder *profileRecorder,
) (Result, error) {
	start := time.Now()
	rows, err := e.reader.QuerySpecRows(ctx, spec)
	recorder.record("execute", start, err, func(entry *queryexec.OperatorProfile) {
		entry.RowsOut = len(rows)
	})
	if err != nil {
		result := layeredResult(layered)
		result.Profile = recorder.profile
		return result, err
	}
	result := layeredResult(layered)
	result.Rows = rows
	result.Profile = recorder.profile
	return result, nil
}

func (e LayeredExecutor) queryColumnStream(
	ctx context.Context,
	spec querylang.QuerySpec,
	layered layeredPlan,
	reader LayeredStreamReader,
	recorder *profileRecorder,
) (StreamResult, error) {
	start := time.Now()
	stream, err := reader.QuerySpecColumnStream(ctx, spec)
	profile := recorder.record("execute", start, err, nil)
	result := streamResult(layered)
	result.Profile = recorder.profile
	if err != nil {
		return result, err
	}
	result.Columns = queryexec.NewProfiledColumnStream(stream, profile)
	return result, nil
}

func (e LayeredExecutor) queryRowStream(
	ctx context.Context,
	spec querylang.QuerySpec,
	layered layeredPlan,
	reader LayeredStreamReader,
	recorder *profileRecorder,
) (StreamResult, error) {
	start := time.Now()
	stream, err := reader.QuerySpecRowStream(ctx, spec)
	profile := recorder.record("execute", start, err, nil)
	result := streamResult(layered)
	result.Profile = recorder.profile
	if err != nil {
		return result, err
	}
	result.Rows = queryexec.NewProfiledRowStream(stream, profile)
	return result, nil
}

func layeredResult(layered layeredPlan) Result {
	return Result{
		LogicalPlanRoot:   layered.logical.Explain().Root,
		PhysicalOperators: physicalOperatorNames(layered.physical.Operators),
		Pushdowns:         append([]string(nil), layered.optimized.Pushdowns...),
	}
}

func physicalOperatorNames(operators []queryphysical.Operator) []string {
	out := make([]string, 0, len(operators))
	for _, operator := range operators {
		out = append(out, string(operator.Kind))
	}
	return out
}

func streamResult(layered layeredPlan) StreamResult {
	return StreamResult{
		LogicalPlanRoot:   layered.logical.Explain().Root,
		PhysicalOperators: physicalOperatorNames(layered.physical.Operators),
		Pushdowns:         append([]string(nil), layered.optimized.Pushdowns...),
	}
}
