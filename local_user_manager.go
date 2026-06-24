package mts

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/openmts/mts/internal/storagefs"
)

const userMetadataFile = "users.bin"

// LocalUserManager 是默认本地用户和 DB 级权限管理器。
type LocalUserManager struct {
	mu     sync.RWMutex
	path   string
	users  map[string]User
	grants map[string]map[string]map[DatabasePermission]struct{}
}

var _ UserManager = (*LocalUserManager)(nil)

// OpenLocalUserManager 打开或创建默认本地用户管理器。
func OpenLocalUserManager(dir string) (*LocalUserManager, error) {
	if err := storagefs.MkdirAll(dir); err != nil {
		return nil, err
	}
	manager := &LocalUserManager{
		path:   filepath.Join(dir, userMetadataFile),
		users:  make(map[string]User),
		grants: make(map[string]map[string]map[DatabasePermission]struct{}),
	}
	if err := manager.load(); err != nil {
		return nil, err
	}
	return manager, nil
}

// Close 关闭本地用户管理器。当前实现无常驻文件句柄。
func (m *LocalUserManager) Close() error {
	return nil
}

func (m *LocalUserManager) CreateUser(ctx context.Context, user User) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	user, err := normalizeUser(user)
	if err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.users[user.Name]; ok {
		return fmt.Errorf("%w: %s", ErrUserAlreadyExists, user.Name)
	}
	users, grants := m.clonedStateLocked()
	users[user.Name] = cloneUser(user)
	return m.replaceStateLocked(users, grants)
}

func (m *LocalUserManager) UpdateUser(ctx context.Context, user User) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	user, err := normalizeUser(user)
	if err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.users[user.Name]; !ok {
		return fmt.Errorf("%w: %s", ErrUserNotFound, user.Name)
	}
	users, grants := m.clonedStateLocked()
	users[user.Name] = cloneUser(user)
	return m.replaceStateLocked(users, grants)
}

func (m *LocalUserManager) GetUser(ctx context.Context, name string) (User, bool, error) {
	if err := ctx.Err(); err != nil {
		return User{}, false, err
	}
	name = strings.TrimSpace(name)
	m.mu.RLock()
	defer m.mu.RUnlock()
	user, ok := m.users[name]
	if !ok {
		return User{}, false, nil
	}
	return cloneUser(user), true, nil
}

func (m *LocalUserManager) ListUsers(ctx context.Context) ([]User, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	names := sortedUserNames(m.users)
	users := make([]User, len(names))
	for index, name := range names {
		users[index] = cloneUser(m.users[name])
	}
	return users, nil
}

func (m *LocalUserManager) DeleteUser(ctx context.Context, name string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	name = strings.TrimSpace(name)
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.users[name]; !ok {
		return fmt.Errorf("%w: %s", ErrUserNotFound, name)
	}
	users, grants := m.clonedStateLocked()
	delete(users, name)
	delete(grants, name)
	return m.replaceStateLocked(users, grants)
}

func (m *LocalUserManager) load() error {
	data, err := storagefs.ReadFile(m.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	users, grants, err := decodeUserMetadata(data)
	if err != nil {
		return fmt.Errorf("decode user metadata: %w", err)
	}
	m.users = users
	m.grants = grants
	return nil
}

func (m *LocalUserManager) replaceStateLocked(
	users map[string]User,
	grants map[string]map[string]map[DatabasePermission]struct{},
) error {
	if err := storagefs.WriteFileAtomic(m.path, encodeUserMetadata(users, grants)); err != nil {
		return err
	}
	m.users = users
	m.grants = grants
	return nil
}

func (m *LocalUserManager) clonedStateLocked() (
	map[string]User,
	map[string]map[string]map[DatabasePermission]struct{},
) {
	return cloneUsers(m.users), cloneGrants(m.grants)
}

func normalizeUser(user User) (User, error) {
	user.Name = strings.TrimSpace(user.Name)
	if user.Name == "" {
		return User{}, ErrInvalidUser
	}
	user.DisplayName = strings.TrimSpace(user.DisplayName)
	user.Metadata = cloneStringMap(user.Metadata)
	return user, nil
}
