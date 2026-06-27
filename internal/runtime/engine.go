package runtime

import (
	"context"
	"errors"

	storageengine "github.com/openmts/mts/internal/engine"
	"github.com/openmts/mts/internal/model"
)

type Options struct {
	Storage     model.Options
	User        UserOptions
	UserManager UserManager
}

type Engine struct {
	storage          *storageengine.Engine
	users            UserManager
	closeUserManager func() error
}

func OpenEngine(ctx context.Context, opts Options) (*Engine, error) {
	storage, err := storageengine.Open(ctx, opts.Storage)
	if err != nil {
		return nil, err
	}
	users, closeUsers, err := openRuntimeEngineUserManager(opts)
	if err != nil {
		closeErr := storage.Close(ctx)
		return nil, errors.Join(err, closeErr)
	}
	return &Engine{
		storage:          storage,
		users:            users,
		closeUserManager: closeUsers,
	}, nil
}

func openRuntimeEngineUserManager(opts Options) (UserManager, func() error, error) {
	if opts.UserManager != nil {
		return opts.UserManager, nil, nil
	}
	manager, err := openRuntimeUserManager(opts.Storage.Path, opts.User)
	if err != nil {
		return nil, nil, err
	}
	return manager, manager.Close, nil
}

func (e *Engine) Close(ctx context.Context) error {
	err := e.storage.Close(ctx)
	if e.closeUserManager != nil {
		err = errors.Join(err, e.closeUserManager())
	}
	return err
}

func (e *Engine) Storage() *storageengine.Engine {
	return e.storage
}

func (e *Engine) Users() UserManager {
	return e.users
}
