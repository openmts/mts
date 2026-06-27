package user

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/openmts/mts/internal/storagefs"
)

type localStateStore struct {
	path      string
	users     map[string]User
	grants    map[string]map[string]map[Permission]struct{}
	passwords map[string]passwordRecord
	tokens    map[string]tokenRecord
}

func newLocalStateStore(dir string) *localStateStore {
	return &localStateStore{
		path:      filepath.Join(dir, userMetadataFile),
		users:     make(map[string]User),
		grants:    make(map[string]map[string]map[Permission]struct{}),
		passwords: make(map[string]passwordRecord),
		tokens:    make(map[string]tokenRecord),
	}
}

func (s *localStateStore) load() (
	map[string]User,
	map[string]map[string]map[Permission]struct{},
	map[string]passwordRecord,
	map[string]tokenRecord,
	error,
) {
	data, err := storagefs.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return emptyLocalState()
		}
		return nil, nil, nil, nil, err
	}
	users, grants, passwords, tokens, err := decodeUserMetadata(data)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("decode user metadata: %w", err)
	}
	return users, grants, passwords, tokens, nil
}

func (s *localStateStore) replace(
	users map[string]User,
	grants map[string]map[string]map[Permission]struct{},
	passwords map[string]passwordRecord,
	tokens map[string]tokenRecord,
) error {
	if err := storagefs.WriteFileAtomic(s.path, encodeUserMetadata(users, grants, passwords, tokens)); err != nil {
		return err
	}
	s.users = users
	s.grants = grants
	s.passwords = passwords
	s.tokens = tokens
	return nil
}

func (s *localStateStore) loadIntoMemory() error {
	users, grants, passwords, tokens, err := s.load()
	if err != nil {
		return err
	}
	s.users = users
	s.grants = grants
	s.passwords = passwords
	s.tokens = tokens
	return nil
}

func (s *localStateStore) cloned() (
	map[string]User,
	map[string]map[string]map[Permission]struct{},
	map[string]passwordRecord,
	map[string]tokenRecord,
) {
	return cloneUsers(s.users),
		cloneGrants(s.grants),
		clonePasswordRecords(s.passwords),
		cloneTokenRecords(s.tokens)
}

func emptyLocalState() (
	map[string]User,
	map[string]map[string]map[Permission]struct{},
	map[string]passwordRecord,
	map[string]tokenRecord,
	error,
) {
	return make(map[string]User),
		make(map[string]map[string]map[Permission]struct{}),
		make(map[string]passwordRecord),
		make(map[string]tokenRecord),
		nil
}
