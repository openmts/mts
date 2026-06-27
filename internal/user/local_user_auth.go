package user

import (
	"context"
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

const (
	passwordSaltBytes  = 16
	passwordHashBytes  = 32
	passwordIterations = 210000
	tokenBytes         = 32
)

type passwordRecord struct {
	Salt       []byte
	Hash       []byte
	Iterations int
}

type tokenRecord struct {
	UserName          string
	ExpiresAtUnixNano int64
}

func (m *Manager) SetPassword(ctx context.Context, userName string, password string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if m.opts.PasswordAuthDisabled {
		return ErrAuthenticationDisabled
	}
	userName = strings.TrimSpace(userName)
	if userName == "" || strings.TrimSpace(password) == "" {
		return ErrInvalidCredentials
	}
	record, err := newPasswordRecord(password)
	if err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.store.users[userName]; !ok {
		return fmt.Errorf("%w: %s", ErrUserNotFound, userName)
	}
	users, grants, passwords, tokens := m.clonedStateLocked()
	passwords[userName] = record
	tokens = removeUserTokens(tokens, userName)
	return m.replaceStateLocked(users, grants, passwords, tokens)
}

func (m *Manager) ChangePassword(
	ctx context.Context,
	userName string,
	oldPassword string,
	newPassword string,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if m.opts.PasswordAuthDisabled {
		return ErrAuthenticationDisabled
	}
	userName = strings.TrimSpace(userName)
	if strings.TrimSpace(newPassword) == "" {
		return ErrInvalidCredentials
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	user, ok := m.store.users[userName]
	if !ok || user.Disabled {
		return ErrInvalidCredentials
	}
	if !verifyPasswordRecord(m.store.passwords[userName], oldPassword) {
		return ErrInvalidCredentials
	}
	record, err := newPasswordRecord(newPassword)
	if err != nil {
		return err
	}
	users, grants, passwords, tokens := m.clonedStateLocked()
	passwords[userName] = record
	tokens = removeUserTokens(tokens, userName)
	return m.replaceStateLocked(users, grants, passwords, tokens)
}

func (m *Manager) VerifyPassword(ctx context.Context, userName string, password string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if m.opts.PasswordAuthDisabled {
		return ErrAuthenticationDisabled
	}
	userName = strings.TrimSpace(userName)
	m.mu.RLock()
	defer m.mu.RUnlock()
	user, ok := m.store.users[userName]
	if !ok || user.Disabled {
		return ErrInvalidCredentials
	}
	if !verifyPasswordRecord(m.store.passwords[userName], password) {
		return ErrInvalidCredentials
	}
	return nil
}

func (m *Manager) Authenticate(ctx context.Context, credentials Credentials, ttl time.Duration) (AuthToken, error) {
	if err := ctx.Err(); err != nil {
		return AuthToken{}, err
	}
	if m.opts.PasswordAuthDisabled {
		return AuthToken{}, ErrAuthenticationDisabled
	}
	if ttl <= 0 {
		return AuthToken{}, ErrInvalidCredentials
	}
	if err := m.VerifyPassword(ctx, credentials.UserName, credentials.Password); err != nil {
		return AuthToken{}, err
	}
	raw, err := randomBytes(tokenBytes)
	if err != nil {
		return AuthToken{}, err
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	expiresAt := time.Now().UTC().Add(ttl)
	userName := strings.TrimSpace(credentials.UserName)
	m.mu.Lock()
	defer m.mu.Unlock()
	user, ok := m.store.users[userName]
	if !ok || user.Disabled {
		return AuthToken{}, ErrInvalidCredentials
	}
	users, grants, passwords, tokens := m.clonedStateLocked()
	tokens[tokenDigest(token)] = tokenRecord{UserName: userName, ExpiresAtUnixNano: expiresAt.UnixNano()}
	if err := m.replaceStateLocked(users, grants, passwords, tokens); err != nil {
		return AuthToken{}, err
	}
	return AuthToken{Token: token, UserName: userName, ExpiresAt: expiresAt}, nil
}

func (m *Manager) VerifyToken(ctx context.Context, token string) (Principal, error) {
	if err := ctx.Err(); err != nil {
		return Principal{}, err
	}
	if m.opts.PasswordAuthDisabled {
		return Principal{}, ErrAuthenticationDisabled
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return Principal{}, ErrInvalidCredentials
	}
	now := time.Now().UTC().UnixNano()
	m.mu.RLock()
	record, ok := m.store.tokens[tokenDigest(token)]
	user := m.store.users[record.UserName]
	m.mu.RUnlock()
	if !ok || record.ExpiresAtUnixNano <= now || user.Name == "" || user.Disabled {
		if ok && record.ExpiresAtUnixNano <= now {
			_ = m.RevokeToken(ctx, token)
		}
		return Principal{}, ErrInvalidCredentials
	}
	return Principal{UserName: record.UserName}, nil
}

func (m *Manager) RevokeToken(ctx context.Context, token string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if m.opts.PasswordAuthDisabled {
		return ErrAuthenticationDisabled
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return ErrInvalidCredentials
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	users, grants, passwords, tokens := m.clonedStateLocked()
	delete(tokens, tokenDigest(token))
	return m.replaceStateLocked(users, grants, passwords, tokens)
}

func newPasswordRecord(password string) (passwordRecord, error) {
	salt, err := randomBytes(passwordSaltBytes)
	if err != nil {
		return passwordRecord{}, err
	}
	hash, err := pbkdf2.Key(sha256.New, password, salt, passwordIterations, passwordHashBytes)
	if err != nil {
		return passwordRecord{}, err
	}
	return passwordRecord{Salt: salt, Hash: hash, Iterations: passwordIterations}, nil
}

func verifyPasswordRecord(record passwordRecord, password string) bool {
	if len(record.Salt) == 0 || len(record.Hash) == 0 || record.Iterations <= 0 {
		return false
	}
	hash, err := pbkdf2.Key(sha256.New, password, record.Salt, record.Iterations, len(record.Hash))
	if err != nil {
		return false
	}
	return subtle.ConstantTimeCompare(hash, record.Hash) == 1
}

func tokenDigest(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func randomBytes(size int) ([]byte, error) {
	value := make([]byte, size)
	if _, err := rand.Read(value); err != nil {
		return nil, err
	}
	return value, nil
}
