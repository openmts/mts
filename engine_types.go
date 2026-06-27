package mts

import (
	"context"

	"github.com/openmts/mts/internal/runtime"
)

// Engine 是单机 MTS 存储引擎实例。
//
// Engine 持有本地数据目录、WAL、MemTable、SSTable、元数据和后台维护任务。
// 使用完成后必须调用 Close 释放文件句柄和后台任务。
type Engine struct {
	runtime *runtime.Engine
}

// Open 打开或创建一个本地 Engine。
//
// 推荐使用 DefaultOptions(path) 构造 opts。Open 会创建本地数据目录并加载
// 已有 shard、manifest 和 metadata。ctx 当前仅保留给 API 一致性；打开期间
// 的文件系统错误会直接返回。
func Open(ctx context.Context, opts Options) (*Engine, error) {
	engine, err := runtime.OpenEngine(ctx, runtime.Options{
		Storage: toModelOptions(opts),
		User:    runtime.UserOptions(opts.User),
	})
	if err != nil {
		return nil, err
	}
	return &Engine{runtime: engine}, nil
}
