package user

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/openmts/mts/internal/storagefs"
)

const userMetadataFile = "users.bin"

// Manager 是默认本地用户和 DB 级权限管理器。
type Manager struct {
	mu    sync.RWMutex
	store *localStateStore
	opts  Options
}

// Open 打开或创建默认本地用户管理器。
func Open(dir string) (*Manager, error) {
	return OpenWithOptions(dir, DefaultOptions())
}

func OpenWithOptions(dir string, opts Options) (*Manager, error) {
	opts = normalizeOptions(opts)
	if opts.Endpoint != EndpointLocal {
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedEndpoint, opts.Endpoint)
	}
	if err := storagefs.MkdirAll(dir); err != nil {
		return nil, err
	}
	manager := &Manager{
		store: newLocalStateStore(dir),
		opts:  opts,
	}
	if err := manager.load(); err != nil {
		return nil, err
	}
	return manager, nil
}

// Close 关闭本地用户管理器。当前实现无常驻文件句柄。
func (m *Manager) Close() error {
	return nil
}

func (m *Manager) Options() Options {
	return m.opts
}

func (m *Manager) CreateUser(ctx context.Context, user User) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	user, err := normalizeUser(user)
	if err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.store.users[user.Name]; ok {
		return fmt.Errorf("%w: %s", ErrUserAlreadyExists, user.Name)
	}
	users, grants, passwords, tokens := m.clonedStateLocked()
	users[user.Name] = cloneUser(user)
	return m.replaceStateLocked(users, grants, passwords, tokens)
}

func (m *Manager) UpdateUser(ctx context.Context, user User) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	user, err := normalizeUser(user)
	if err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.store.users[user.Name]; !ok {
		return fmt.Errorf("%w: %s", ErrUserNotFound, user.Name)
	}
	users, grants, passwords, tokens := m.clonedStateLocked()
	users[user.Name] = cloneUser(user)
	return m.replaceStateLocked(users, grants, passwords, tokens)
}

func (m *Manager) GetUser(ctx context.Context, name string) (User, bool, error) {
	if err := ctx.Err(); err != nil {
		return User{}, false, err
	}
	name = strings.TrimSpace(name)
	m.mu.RLock()
	defer m.mu.RUnlock()
	user, ok := m.store.users[name]
	if !ok {
		return User{}, false, nil
	}
	return cloneUser(user), true, nil
}

func (m *Manager) ListUsers(ctx context.Context) ([]User, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	names := sortedUserNames(m.store.users)
	users := make([]User, len(names))
	for index, name := range names {
		users[index] = cloneUser(m.store.users[name])
	}
	return users, nil
}

func (m *Manager) DeleteUser(ctx context.Context, name string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	name = strings.TrimSpace(name)
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.store.users[name]; !ok {
		return fmt.Errorf("%w: %s", ErrUserNotFound, name)
	}
	users, grants, passwords, tokens := m.clonedStateLocked()
	delete(users, name)
	delete(grants, name)
	delete(passwords, name)
	tokens = removeUserTokens(tokens, name)
	return m.replaceStateLocked(users, grants, passwords, tokens)
}

func (m *Manager) load() error {
	return m.store.loadIntoMemory()
}

func (m *Manager) replaceStateLocked(
	users map[string]User,
	grants map[string]map[string]map[Permission]struct{},
	passwords map[string]passwordRecord,
	tokens map[string]tokenRecord,
) error {
	if err := m.store.replace(users, grants, passwords, tokens); err != nil {
		return err
	}
	return nil
}

func (m *Manager) clonedStateLocked() (
	map[string]User,
	map[string]map[string]map[Permission]struct{},
	map[string]passwordRecord,
	map[string]tokenRecord,
) {
	return m.store.cloned()
}

func normalizeUser(user User) (User, error) {
	user.Name = strings.TrimSpace(user.Name)
	if user.Name == "" {
		return User{}, ErrInvalidUser
	}
	user.DisplayName = strings.TrimSpace(user.DisplayName)
	user.Role = normalizeRole(user.Role)
	if !validRole(user.Role) {
		return User{}, ErrInvalidUser
	}
	user.Metadata = cloneStringMap(user.Metadata)
	return user, nil
}

func normalizeRole(role Role) Role {
	role = Role(strings.TrimSpace(string(role)))
	if role == "" {
		return RoleUser
	}
	return role
}

func validRole(role Role) bool {
	switch role {
	case RoleUser, RoleAdmin:
		return true
	default:
		return false
	}
}

func normalizeOptions(opts Options) Options {
	if strings.TrimSpace(opts.Endpoint) == "" {
		opts.Endpoint = EndpointLocal
	} else {
		opts.Endpoint = strings.TrimSpace(opts.Endpoint)
	}
	return opts
}
