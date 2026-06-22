# MTS

MTS 是一个单机嵌入式时序存储库，面向 Go 应用内的本地时序数据写入、查询、元数据管理、compaction、retention 和降采样场景。

当前项目只承诺单机本地目录能力：不做分布式查询、分布式存储、外部元数据系统，也不提供 SQL、InfluxQL、PromQL 或 MetricsQL parser。查询入口以 Go Builder/API 为主。

当前稳定对外接口只有 Go library 根包 `github.com/openmts/mts` 和本地维护 CLI `cmd/mts-storage`。`internal/queryservice` 和 `internal/service` 中的 HTTP 查询、metrics、health、admin 实现不作为当前外部稳定 API。

## 安装

```bash
go get github.com/openmts/mts
```

## 快速开始

```go
ctx := context.Background()
engine, err := mts.Open(ctx, mts.DefaultOptions("/var/lib/mts"))
if err != nil {
	return err
}
defer func() {
	_ = engine.Close(ctx)
}()

err = engine.Write(ctx, []mts.Point{{
	Measurement: "cpu",
	Tags:        map[string]string{"host": "a"},
	Timestamp:   time.Now().UnixNano(),
	Fields: map[string]mts.FieldValue{
		"usage": mts.Float64Value(0.42),
	},
}}, mts.WriteOptions{Sync: true})
if err != nil {
	return err
}

query, err := mts.NewQuery().
	From("", "", "cpu").
	Where(mts.TagEq("host", "a")).
	TimeRangeTime(time.Unix(0, 0), time.Now()).
	Limit(100).
	Build()
if err != nil {
	return err
}

rows, err := engine.QueryRows(ctx, query)
if err != nil {
	return err
}
_ = rows
```

`QueryRows` 和 `QueryColumns` 适合小结果集。生产路径建议优先使用 `QueryRowIterator` 或 `QueryColumnIterator`，并配置 `Limit` 或 `QueryBudget`。

## Builder 查询

```go
query, err := mts.NewQuery().
	Select("usage").
	From("metrics", "autogen", "cpu").
	Where(
		mts.TagEq("host", "a"),
		mts.FieldGT("usage", mts.Float64Value(0.8)),
	).
	Aggregate(mts.AggregateMean, "usage").
	GroupByTime(time.Minute).
	OrderByTimeDesc().
	Limit(100).
	Build()
```

聚合函数支持矩阵见 [docs/query/builder-aggregate-functions.md](docs/query/builder-aggregate-functions.md)。

## 批量写入

高吞吐写入优先使用 `WriteTypedBatch`，避免每个点都构造字段 map：

```go
batch := mts.TypedBatch{
	Measurement: "cpu",
	Tags: []mts.TagColumn{
		{Name: "host", Values: []string{"a", "a", "b"}},
	},
	Timestamps: []int64{10, 20, 30},
	Fields: []mts.TypedFieldColumn{
		{Name: "usage", Type: mts.FieldFloat64, Float64Values: []float64{0.1, 0.2, 0.3}},
	},
}

err := engine.WriteTypedBatch(ctx, batch, mts.WriteOptions{Sync: true})
```

## 降采样

MTS 支持本地降采样策略，结果写入显式目标 retention policy。查询降采样结果时使用 `FromDownsamplePolicy`：

```go
query, err := mts.NewQuery().
	FromDownsamplePolicy(policy).
	Select("avg_usage").
	TimeRange(start, end).
	Build()
```

详细运维说明见 [docs/storage/downsample-runbook.md](docs/storage/downsample-runbook.md)。

## 本地维护工具

`cmd/mts-storage` 提供本地目录检查、修复、迁移、快照和恢复：

```bash
go install github.com/openmts/mts/cmd/mts-storage@latest

go run ./cmd/mts-storage check /var/lib/mts
go run ./cmd/mts-storage repair --dry-run /var/lib/mts
go run ./cmd/mts-storage snapshot /var/lib/mts /backup/mts-snapshot
go run ./cmd/mts-storage restore /backup/mts-snapshot /var/lib/mts-restored
```

## 质量门禁

常规改动完成后执行：

```bash
go test ./... -count=1 -timeout 10m
golangci-lint run ./...
go test $(go list ./... | grep -v '/tests/') -cover -count=1 -timeout 10m
go test ./tests/... -count=1 -timeout 10m
```

商用交付门禁要求生产包代码行覆盖率达到 90% 以上；`tests/**` 下的 e2e、fault、scale 和 pprof harness 作为行为验证单独执行。
