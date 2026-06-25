package mts

import (
	"context"
	"errors"

	storageengine "github.com/openmts/mts/internal/engine"
)

// Engine 是单机 MTS 存储引擎实例。
//
// Engine 持有本地数据目录、WAL、MemTable、SSTable、元数据和后台维护任务。
// 使用完成后必须调用 Close 释放文件句柄和后台任务。
type Engine struct {
	inner            *storageengine.Engine
	userManager      UserManager
	closeUserManager func() error
}

// Open 打开或创建一个本地 Engine。
//
// 推荐使用 DefaultOptions(path) 构造 opts。Open 会创建本地数据目录并加载
// 已有 shard、manifest 和 metadata。ctx 当前仅保留给 API 一致性；打开期间
// 的文件系统错误会直接返回。
func Open(ctx context.Context, opts Options) (*Engine, error) {
	inner, err := storageengine.Open(ctx, toModelOptions(opts))
	if err != nil {
		return nil, err
	}
	userManager, closeUserManager, err := openEngineUserManager(opts)
	if err != nil {
		closeErr := inner.Close(ctx)
		return nil, errors.Join(err, closeErr)
	}
	return &Engine{
		inner:            inner,
		userManager:      userManager,
		closeUserManager: closeUserManager,
	}, nil
}

func openEngineUserManager(opts Options) (UserManager, func() error, error) {
	if opts.UserManager != nil {
		return opts.UserManager, nil, nil
	}
	manager, err := openLocalUserManager(opts.Path)
	if err != nil {
		return nil, nil, err
	}
	return manager, manager.Close, nil
}
